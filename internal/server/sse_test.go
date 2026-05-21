package server

import (
	"testing"
	"time"
)

func TestBroker_UnsubscribeClosesChannel(t *testing.T) {
	b := NewBroker()
	ch, unsub := b.Subscribe()
	if got := b.Len(); got != 1 {
		t.Fatalf("len after subscribe = %d, want 1", got)
	}
	unsub()
	if got := b.Len(); got != 0 {
		t.Fatalf("len after unsubscribe = %d, want 0", got)
	}
	// Reading from a closed channel returns the zero Event and
	// ok=false immediately. We assert non-blocking close.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected channel to be closed (ok=false), got open")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel did not close within 100 ms")
	}
	// Calling unsubscribe again must be a no-op.
	unsub()
}

func TestBroker_SlowClientDoesNotBlockOthers(t *testing.T) {
	b := NewBroker()
	_, unsubSlow := b.Subscribe() // slow: never drained; buffer fills on first broadcast.
	fast, unsubFast := b.Subscribe()
	defer unsubSlow()
	defer unsubFast()

	// First broadcast: slow's buffer fills, fast receives 1 event.
	b.Broadcast()
	select {
	case <-fast:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("fast client did not receive first broadcast")
	}

	// Second broadcast: slow's buffer is still full (never read).
	// Non-blocking send must drop on slow and succeed on fast.
	b.Broadcast()
	select {
	case <-fast:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("fast client did not receive second broadcast — slow client blocked")
	}
}
