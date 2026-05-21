package server

import "sync"

// Event is the payload sent to each subscriber on broadcast. It is
// intentionally empty: the SSE contract (ADR 0002 §D9) emits a single
// event type `board-changed` with no data — clients refetch
// `/api/board` on receipt.
type Event struct{}

// Broker fans a single source event (the watcher firing) out to
// every subscribed SSE client. Subscribe returns a per-client channel
// plus an unsubscribe closure; Broadcast performs a non-blocking send
// to every channel so a slow client cannot stall the fan-out
// (ADR 0002 §D9 — server is best-effort, browser auto-retries).
type Broker struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
}

// NewBroker constructs an empty broker. The zero value is unsafe
// because the clients map is nil — always go through NewBroker.
func NewBroker() *Broker {
	return &Broker{clients: map[chan Event]struct{}{}}
}

// Subscribe registers a new client and returns the channel it should
// read from plus an unsubscribe closure. The channel is buffered (1)
// so the broker's non-blocking send succeeds even when the client is
// momentarily inside its write loop.
//
// The unsubscribe closure removes the channel from the broker, closes
// it, and is safe to call multiple times — the second call is a
// no-op (delete on a missing key, double-close protected via the
// presence check).
func (b *Broker) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 1)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			if _, ok := b.clients[ch]; ok {
				delete(b.clients, ch)
				close(ch)
			}
			b.mu.Unlock()
		})
	}
	return ch, unsubscribe
}

// Broadcast delivers one Event to every subscribed channel using a
// non-blocking send. A client whose buffer is already full silently
// drops the event — the next broadcast (or the heartbeat) will catch
// it up; the SSE semantics already cover redelivery via refetch.
func (b *Broker) Broadcast() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- Event{}:
		default:
		}
	}
}

// Len reports the number of currently-subscribed clients. Exposed
// for tests; production code never reads it.
func (b *Broker) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clients)
}

// Close evicts every currently-subscribed client by closing their
// channels. SSE handlers detect the close and return, allowing
// http.Server.Shutdown to drain in-flight connections within the 5 s
// timeout instead of timing out against long-lived event streams.
//
// Subscribe calls after Close are still permitted (the clients map
// stays valid) but production callers should treat Close as a
// terminal signal.
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		close(ch)
		delete(b.clients, ch)
	}
}
