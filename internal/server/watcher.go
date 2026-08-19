package server

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watcherDebounce is the burst-coalescing window applied to fsnotify
// events before a downstream event is fired (ADR 0002 §D10). Exposed
// as a package variable so tests can shrink it.
var watcherDebounce = 200 * time.Millisecond

// Watcher observes one or more files for external changes and emits a
// single coalesced event on its Events() channel after a 200 ms
// debounce window per ADR 0002 §D10, regardless of which watched file
// (or how many at once) changed.
//
// It arms one fsnotify watch per distinct parent directory of the
// given paths, rather than one per file, and filters delivered events
// by basename against the set of watched files. A directory watch
// survives a file within it not existing yet — this is what lets the
// archive sibling be watched from server boot even though it usually
// does not exist until the first archive operation creates it — and is
// not disturbed by an atomic temp+rename underneath it, so unlike the
// single-file design this watcher never needs to re-arm.
type Watcher struct {
	names  map[string]struct{} // watched basenames
	events chan struct{}
	fsw    *fsnotify.Watcher

	// errMu guards lastErr so tests can observe non-fatal errors
	// without racing.
	errMu   sync.Mutex
	lastErr error
}

// NewWatcher constructs a Watcher armed on the parent directory of
// each given path. Returns an error when fsnotify cannot allocate a
// watcher or when a parent directory cannot be added (e.g. it does not
// exist) — but NOT when an individual file does not yet exist, since
// that is the expected state for a not-yet-created archive file.
func NewWatcher(paths ...string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	dirs := make(map[string]struct{}, len(paths))
	names := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		dirs[filepath.Dir(p)] = struct{}{}
		names[filepath.Base(p)] = struct{}{}
	}
	for dir := range dirs {
		if err := fsw.Add(dir); err != nil {
			_ = fsw.Close()
			return nil, err
		}
	}
	return &Watcher{
		names:  names,
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
// events, filters them to the watched basenames, and applies the
// debounce timer.
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
			if _, watched := w.names[filepath.Base(ev.Name)]; !watched {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
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
