package server

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// hotReloadFixtureBody is the minimal valid kanban file used by the
// Run-level hot-reload tests. Cards are absent on purpose; the
// watcher only cares that the file exists at startup.
const hotReloadFixtureBody = `schema_version = 1

[board]
columns = ["todo", "done"]
priorities = ["low", "medium", "high"]
`

func writeHotReloadFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	if err := os.WriteFile(path, []byte(hotReloadFixtureBody), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return path
}

// freeLoopbackPort allocates and immediately releases a 127.0.0.1
// port so tests can hand it to runWithContext without colliding.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	defer probe.Close()
	return probe.Addr().(*net.TCPAddr).Port
}

func TestRun_HotReload_Smoke(t *testing.T) {
	silenceRunOutput(t)
	withStubRunner(t)

	path := writeHotReloadFixture(t)
	port := freeLoopbackPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWithContext(ctx, Options{Port: port, NoOpen: true, Board: path})
	}()
	defer func() {
		cancel()
		<-done
	}()
	waitForListen(t, port, 2*time.Second)

	// Open the SSE stream.
	req, _ := http.NewRequest("GET", "http://127.0.0.1:"+strconv.Itoa(port)+"/api/events", nil)
	rctx, rcancel := context.WithCancel(context.Background())
	defer rcancel()
	req = req.WithContext(rctx)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer res.Body.Close()
	br := bufio.NewReader(res.Body)
	_ = readSSEChunk(br, 1*time.Second) // drain retry directive

	// Give the handler a beat to call Subscribe(). Then atomically
	// rewrite the board file from outside and assert we see the
	// event within ~1 s (debounce 200 ms + scheduling slack).
	time.Sleep(100 * time.Millisecond)
	writeAtomic(t, path, []byte(hotReloadFixtureBody+"\n# touch\n"))

	chunk := readSSEChunk(br, 2*time.Second)
	if !strings.Contains(chunk, "event: board-changed") {
		t.Fatalf("expected board-changed event after external write, got: %q", chunk)
	}
}

func TestRun_FailsIfBoardMissing(t *testing.T) {
	silenceRunOutput(t)
	withStubRunner(t)

	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.toml")
	port := freeLoopbackPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := runWithContext(ctx, Options{Port: port, NoOpen: true, Board: missing})
	if err == nil {
		t.Fatal("expected error when board file is missing")
	}

	// The HTTP listener must NOT have been bound — confirm by
	// successfully binding the same port ourselves.
	ln, lerr := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if lerr != nil {
		t.Fatalf("port %d still in use: %v", port, lerr)
	}
	_ = ln.Close()
}

func TestRun_ShutdownReleasesClients(t *testing.T) {
	silenceRunOutput(t)
	withStubRunner(t)

	path := writeHotReloadFixture(t)
	port := freeLoopbackPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWithContext(ctx, Options{Port: port, NoOpen: true, Board: path})
	}()
	waitForListen(t, port, 2*time.Second)

	// Subscribe a client.
	req, _ := http.NewRequest("GET", "http://127.0.0.1:"+strconv.Itoa(port)+"/api/events", nil)
	rctx, rcancel := context.WithCancel(context.Background())
	defer rcancel()
	req = req.WithContext(rctx)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer res.Body.Close()
	br := bufio.NewReader(res.Body)
	_ = readSSEChunk(br, 1*time.Second)

	// Allow the handler a beat to register with the broker before
	// shutdown.
	time.Sleep(100 * time.Millisecond)

	clientLoopDone := make(chan struct{})
	go func() {
		// Read until EOF / connection close. This must return when
		// the server shuts down, freeing the goroutine.
		for {
			if _, err := br.ReadString('\n'); err != nil {
				close(clientLoopDone)
				return
			}
		}
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithContext returned error: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("runWithContext did not return within 6 s of cancel")
	}

	select {
	case <-clientLoopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("client read loop did not exit within 2 s of server shutdown")
	}
}
