package server

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// readSSEChunk reads until the next \n\n delimiter (one SSE message)
// and returns the chunk including the trailing blank line. Returns
// "" on EOF / timeout.
func readSSEChunk(r *bufio.Reader, timeout time.Duration) string {
	type result struct {
		s   string
		err error
	}
	done := make(chan result, 1)
	go func() {
		var sb strings.Builder
		for {
			line, err := r.ReadString('\n')
			sb.WriteString(line)
			if err != nil {
				done <- result{sb.String(), err}
				return
			}
			if line == "\n" {
				done <- result{sb.String(), nil}
				return
			}
		}
	}()
	select {
	case res := <-done:
		return res.s
	case <-time.After(timeout):
		return ""
	}
}

func TestHandle_Events_SendsRetryHeader(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	req, _ := http.NewRequest("GET", ts.URL+"/api/events", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	defer cancel()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}

	br := bufio.NewReader(res.Body)
	chunk := readSSEChunk(br, 1*time.Second)
	if !strings.Contains(chunk, "retry: 2000") {
		t.Fatalf("first chunk missing retry directive: %q", chunk)
	}
}

func TestHandle_Events_BroadcastsBoardChanged(t *testing.T) {
	ts, broker, cleanup := startTestServerWithBroker(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	req, _ := http.NewRequest("GET", ts.URL+"/api/events", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	defer cancel()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer res.Body.Close()

	br := bufio.NewReader(res.Body)
	// Drain the initial `retry: 2000` chunk.
	_ = readSSEChunk(br, 1*time.Second)

	// Wait briefly so Subscribe() has executed inside the handler.
	deadline := time.Now().Add(1 * time.Second)
	for broker.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if broker.Len() == 0 {
		t.Fatal("client never subscribed within 1 s")
	}

	broker.Broadcast()
	chunk := readSSEChunk(br, 1*time.Second)
	if !strings.Contains(chunk, "event: board-changed") {
		t.Fatalf("expected board-changed event, got: %q", chunk)
	}
}

func TestHandle_Events_HeartbeatTickerWorks(t *testing.T) {
	// Shrink the heartbeat for this test only.
	prev := heartbeatInterval
	heartbeatInterval = 100 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = prev })

	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	req, _ := http.NewRequest("GET", ts.URL+"/api/events", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	defer cancel()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer res.Body.Close()

	br := bufio.NewReader(res.Body)
	_ = readSSEChunk(br, 1*time.Second) // retry directive
	chunk := readSSEChunk(br, 500*time.Millisecond)
	if !strings.Contains(chunk, ": ping") {
		t.Fatalf("expected ping heartbeat, got: %q", chunk)
	}
}

func TestHandle_Events_ClientDisconnectFreesSubscription(t *testing.T) {
	ts, broker, cleanup := startTestServerWithBroker(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	req, _ := http.NewRequest("GET", ts.URL+"/api/events", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	br := bufio.NewReader(res.Body)
	_ = readSSEChunk(br, 1*time.Second)

	deadline := time.Now().Add(1 * time.Second)
	for broker.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if broker.Len() != 1 {
		t.Fatalf("len after subscribe = %d, want 1", broker.Len())
	}

	cancel()
	_ = res.Body.Close()

	deadline = time.Now().Add(2 * time.Second)
	for broker.Len() != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if broker.Len() != 0 {
		t.Fatalf("len after disconnect = %d, want 0", broker.Len())
	}
}
