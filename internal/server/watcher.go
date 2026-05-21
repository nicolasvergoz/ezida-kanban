package server

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watcherDebounce is the burst-coalescing window applied to fsnotify
// events before a downstream event is fired (ADR 0002 §D10). Exposed
// as a package variable so tests can shrink it.
var watcherDebounce = 200 * time.Millisecond

// Watcher observes the board file for external changes and emits a
// single coalesced event on its Events() channel after a 200 ms
// debounce window per ADR 0002 §D10.
//
// The watcher re-arms the underlying fsnotify watch after Rename and
// Create events so atomic temp+rename rewrites (board.Save) continue
// to fire. Errors from the underlying fsnotify.Watcher are dropped on
// the floor in v1 (best-effort observability is acceptable for a
// localhost developer tool).
type Watcher struct {
	path   string
	events chan struct{}
	fsw    *fsnotify.Watcher

	// errMu guards lastErr so tests can observe non-fatal errors
	// without racing.
	errMu   sync.Mutex
	lastErr error
}

// NewWatcher constructs a Watcher armed on the given board file path.
// Returns an error when fsnotify cannot allocate a watcher or when
// the path cannot be added (e.g. the file does not exist).
func NewWatcher(path string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fsw.Add(path); err != nil {
		_ = fsw.Close()
		return nil, err
	}
	return &Watcher{
		path:   path,
		events: make(chan struct{}, 1),
		fsw:    fsw,
	}, nil
}

// Events returns the receive-only channel that fires once per
// debounced burst of filesystem events. The buffer is 1 so a slow
// consumer that misses one event does not back up the watcher: the
// next event after the burst is dropped if the buffer is still full.
func (w *Watcher) Events() <-chan struct{} { return w.events }

// Run blocks until ctx is cancelled. On exit it closes the underlying
// fsnotify watcher. Run owns the goroutine that drains fsnotify
// events, applies the debounce timer, and re-arms the watch on
// Rename/Create.
func (w *Watcher) Run(ctx context.Context) {
	defer func() { _ = w.fsw.Close() }()

	var debounceTimer *time.Timer
	stopTimer := func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
			debounceTimer = nil
		}
	}
	defer stopTimer()

	fire := func() {
		select {
		case w.events <- struct{}{}:
		default:
			// Drop if a previous event has not been consumed yet.
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			// On Rename/Create/Remove the watched inode may have
			// been replaced (atomic temp+rename — on Linux this
			// surfaces as Remove of the old inode, on macOS as
			// Rename). Re-arm so subsequent rewrites still fire.
			if ev.Op&(fsnotify.Rename|fsnotify.Create|fsnotify.Remove) != 0 {
				if err := w.fsw.Add(w.path); err != nil {
					if !errors.Is(err, fsnotify.ErrEventOverflow) {
						w.errMu.Lock()
						w.lastErr = err
						w.errMu.Unlock()
					}
				}
			}
			stopTimer()
			debounceTimer = time.AfterFunc(watcherDebounce, fire)
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.errMu.Lock()
			w.lastErr = err
			w.errMu.Unlock()
		}
	}
}
