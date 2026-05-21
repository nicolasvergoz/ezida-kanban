package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeAtomic mirrors board.Save's temp+rename pattern so the
// watcher tests exercise the same Rename-driven path as production.
func writeAtomic(t *testing.T, path string, data []byte) {
	t.Helper()
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".watch-test-*")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	if _, err := tmp.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		t.Fatalf("rename: %v", err)
	}
}

// newWatcherFixture creates a fresh kanban-like file in a temp dir
// and returns the file path. The file is non-empty so fsnotify can
// reliably arm against it on all platforms.
func newWatcherFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	if err := os.WriteFile(path, []byte("schema_version = 1\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return path
}

func TestWatcher_DebouncesBurst(t *testing.T) {
	path := newWatcherFixture(t)
	w, err := NewWatcher(path)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	// Fire 3 writes within 100 ms.
	for i := 0; i < 3; i++ {
		writeAtomic(t, path, []byte("schema_version = 1\n# burst "+string(rune('a'+i))+"\n"))
		time.Sleep(30 * time.Millisecond)
	}

	// Collect events for 500 ms after the burst.
	deadline := time.After(500 * time.Millisecond)
	count := 0
collect:
	for {
		select {
		case <-w.Events():
			count++
		case <-deadline:
			break collect
		}
	}
	if count > 1 {
		t.Fatalf("debounce: got %d events, want <=1", count)
	}
	if count == 0 {
		t.Fatalf("debounce: got 0 events, want exactly 1 after a burst")
	}
}

func TestWatcher_SurvivesRename(t *testing.T) {
	path := newWatcherFixture(t)
	w, err := NewWatcher(path)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	// First atomic rewrite.
	writeAtomic(t, path, []byte("schema_version = 1\n# first\n"))
	if !waitEvent(w, 1*time.Second) {
		t.Fatalf("expected first event")
	}
	// Wait ~1 s and rewrite again — the re-arm logic must keep the
	// watch alive across the rename so this second event still fires.
	time.Sleep(1 * time.Second)
	writeAtomic(t, path, []byte("schema_version = 1\n# second\n"))
	if !waitEvent(w, 1*time.Second) {
		t.Fatalf("expected second event after rename re-arm")
	}
}

func TestWatcher_ShutsDownOnCancel(t *testing.T) {
	path := newWatcherFixture(t)
	w, err := NewWatcher(path)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("watcher did not return within 500 ms of context cancel")
	}
}

// waitEvent reads one event from the watcher, returning false on
// timeout.
func waitEvent(w *Watcher, timeout time.Duration) bool {
	select {
	case <-w.Events():
		return true
	case <-time.After(timeout):
		return false
	}
}
