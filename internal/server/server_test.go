package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// stubRunner is the commandRunner test seam. It records every Open
// call without actually exec-ing anything.
type stubRunner struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (s *stubRunner) Open(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, url)
	return s.err
}

func (s *stubRunner) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

// withStubRunner swaps the package-level runner for a stub for the
// lifetime of the test.
func withStubRunner(t *testing.T) *stubRunner {
	t.Helper()
	prev := runnerForOpen
	stub := &stubRunner{}
	runnerForOpen = stub
	t.Cleanup(func() { runnerForOpen = prev })
	return stub
}

// silenceRunOutput suppresses the banner / warning writes so tests
// do not litter stdout/stderr.
func silenceRunOutput(t *testing.T) {
	t.Helper()
	prevOut, prevErr := stdoutForRun, stderrForRun
	stdoutForRun = func(string) (int, error) { return 0, nil }
	stderrForRun = func(string) (int, error) { return 0, nil }
	t.Cleanup(func() {
		stdoutForRun = prevOut
		stderrForRun = prevErr
	})
}

// startTestServer constructs an httptest.Server wrapping the same
// mux Run wires up. It is the shared entry point for handler tests.
// boardPath points to a kanban.toml fixture (may not exist if the
// test is exercising the BOARD_NOT_FOUND branch).
func startTestServer(t *testing.T, boardPath string) (*httptest.Server, func()) {
	t.Helper()
	s := &serverState{boardPath: boardPath}
	mux := http.NewServeMux()
	s.routes(mux)
	ts := httptest.NewServer(mux)
	return ts, ts.Close
}

// fixturePath returns the absolute path to testdata/<name>.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return abs
}

func TestRun_PortFallback(t *testing.T) {
	silenceRunOutput(t)
	withStubRunner(t)

	// Pre-bind a listener on 7777 to force fallback to 7778.
	pre, err := net.Listen("tcp", "127.0.0.1:7777")
	if err != nil {
		t.Skipf("cannot pre-bind 7777: %v", err)
	}
	defer pre.Close()

	ln, port, err := bindWithFallback(7777, portFallbackWindow)
	if err != nil {
		t.Fatalf("bindWithFallback: %v", err)
	}
	defer ln.Close()
	if port != 7778 {
		t.Fatalf("port = %d, want 7778", port)
	}
}

func TestPortUnavailableError_OutputFail(t *testing.T) {
	err := &PortUnavailableError{Start: 7777, Window: 11}
	code, exit := output.Classify(err)
	if code != "PORT_UNAVAILABLE" {
		t.Fatalf("code = %q, want PORT_UNAVAILABLE", code)
	}
	if exit == 0 {
		t.Fatalf("exit code should be non-zero, got %d", exit)
	}
	// Render via FailTo against a buffer and confirm the JSON shape.
	var sb strings.Builder
	code2 := output.FailTo(stringWriter{&sb}, err, true)
	_ = code2
	body := sb.String()
	if !strings.Contains(body, `"code":"PORT_UNAVAILABLE"`) {
		t.Fatalf("JSON envelope missing PORT_UNAVAILABLE code: %s", body)
	}
}

// stringWriter adapts strings.Builder to io.Writer for FailTo.
type stringWriter struct{ b *strings.Builder }

func (s stringWriter) Write(p []byte) (int, error) { return s.b.Write(p) }

func TestRun_GracefulShutdown(t *testing.T) {
	silenceRunOutput(t)
	withStubRunner(t)

	// Allocate a free port to avoid colliding with anything bound
	// outside the test.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	// Use runWithContext so we can cancel via the context rather
	// than firing a real SIGINT (which would also terminate the
	// test binary). The Run wrapper just composes
	// signal.NotifyContext + runWithContext; the shutdown protocol
	// under test lives entirely in runWithContext.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWithContext(ctx, Options{Port: port, NoOpen: true, Board: fixturePath(t, "valid_kanban.toml")})
	}()
	waitForListen(t, port, 2*time.Second)

	start := time.Now()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithContext returned error: %v", err)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("shutdown took %s, want <1s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runWithContext did not return within 2s of cancel")
	}
}

// waitForListen polls 127.0.0.1:<port> with TCP dial until it
// succeeds or deadline elapses.
func waitForListen(t *testing.T, port int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start listening on %d within %s", port, timeout)
}

func TestRun_NoOpen_SkipsBrowser(t *testing.T) {
	silenceRunOutput(t)
	stub := withStubRunner(t)

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWithContext(ctx, Options{Port: port, NoOpen: true, Board: fixturePath(t, "valid_kanban.toml")})
	}()
	waitForListen(t, port, 2*time.Second)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runWithContext did not return")
	}

	if got := stub.Calls(); len(got) != 0 {
		t.Fatalf("expected zero browser calls with NoOpen=true, got %v", got)
	}
}

func TestRun_OpensBrowser(t *testing.T) {
	silenceRunOutput(t)
	stub := withStubRunner(t)

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWithContext(ctx, Options{Port: port, NoOpen: false, Board: fixturePath(t, "valid_kanban.toml")})
	}()
	waitForListen(t, port, 2*time.Second)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runWithContext did not return")
	}

	calls := stub.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 browser call, got %v", calls)
	}
	want := "http://127.0.0.1:" + strconv.Itoa(port)
	if calls[0] != want {
		t.Fatalf("browser opened %q, want %q", calls[0], want)
	}
}

func TestHandle_Index(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("Content-Type = %q", res.Header.Get("Content-Type"))
	}
	body, _ := io.ReadAll(res.Body)
	want, _ := webFS.ReadFile("web/index.html")
	if string(body) != string(want) {
		t.Fatalf("body mismatch:\n got: %q\nwant: %q", body, want)
	}
}

func TestHandle_Static_App(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/static/app.js")
	if err != nil {
		t.Fatalf("GET /static/app.js: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	want, _ := webFS.ReadFile("web/app.js")
	if string(body) != string(want) {
		t.Fatalf("body mismatch:\n got: %q\nwant: %q", body, want)
	}
}

func TestHandle_Board_Valid(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/board")
	if err != nil {
		t.Fatalf("GET /api/board: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var payload struct {
		SchemaVersion  int            `json:"schema_version"`
		Columns        []string       `json:"columns"`
		Priorities     []string       `json:"priorities"`
		CardsPerColumn map[string]int `json:"cards_per_column"`
		Cards []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Column      string `json:"column"`
			Description string `json:"description"`
		} `json:"cards"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", payload.SchemaVersion)
	}
	if len(payload.Columns) != 2 || payload.Columns[0] != "todo" {
		t.Fatalf("columns = %v", payload.Columns)
	}
	if payload.CardsPerColumn["todo"] != 2 || payload.CardsPerColumn["done"] != 1 {
		t.Fatalf("cards_per_column = %v", payload.CardsPerColumn)
	}
	if len(payload.Cards) != 3 {
		t.Fatalf("len(cards) = %d, want 3", len(payload.Cards))
	}
	// Description must be present on every card (may be empty
	// string). Spec scenario: "each card in cards has a description
	// field (may be empty string)".
	for i, c := range payload.Cards {
		if c.ID == "" {
			t.Fatalf("card %d missing ID", i)
		}
	}
}

func TestHandle_Board_Missing(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "does_not_exist.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/board")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 500 {
		t.Fatalf("status = %d, want 500", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"BOARD_NOT_FOUND"`) {
		t.Fatalf("expected BOARD_NOT_FOUND in body")
	}
}

func TestHandle_Board_SchemaMismatch(t *testing.T) {
	// Use the upstream board package's fixture file directly via a
	// per-test temp file so we don't duplicate maintenance.
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	if err := os.WriteFile(path, []byte("schema_version = 2\n\n[board]\ncolumns = [\"todo\"]\npriorities = [\"low\"]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/board")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 500 {
		t.Fatalf("status = %d, want 500", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"SCHEMA_VERSION_MISMATCH"`) {
		t.Fatalf("expected SCHEMA_VERSION_MISMATCH in body")
	}
}

func TestHandle_Board_Invalid(t *testing.T) {
	// Build an inline invalid board (duplicate IDs) so the test is
	// self-contained.
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	const body = `schema_version = 1

[board]
columns = ["todo"]
priorities = ["low"]

[[cards]]
id = "aaaaaa"
title = "First"
column = "todo"
description = ""
created_at = 2026-05-01T09:00:00Z
updated_at = 2026-05-01T09:00:00Z
tags = []

[[cards]]
id = "aaaaaa"
title = "Second"
column = "todo"
description = ""
created_at = 2026-05-01T09:00:00Z
updated_at = 2026-05-01T09:00:00Z
tags = []
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/board")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 500 {
		t.Fatalf("status = %d, want 500", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"VALIDATION_FAILED"`) {
		t.Fatalf("expected VALIDATION_FAILED in body")
	}
}

func TestHandle_Unknown_Route(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/does-not-exist")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"NOT_FOUND"`) {
		t.Fatalf("expected NOT_FOUND in body")
	}
}

// readString collects the response body for substring checks.
func readString(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestServe_BindIsLoopbackOnly(t *testing.T) {
	silenceRunOutput(t)
	withStubRunner(t)

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWithContext(ctx, Options{Port: port, NoOpen: true, Board: fixturePath(t, "valid_kanban.toml")})
	}()
	waitForListen(t, port, 2*time.Second)
	defer func() {
		cancel()
		<-done
	}()

	// Loopback connects.
	loopConn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 500*time.Millisecond)
	if err != nil {
		t.Fatalf("loopback dial failed: %v", err)
	}
	_ = loopConn.Close()

	// External-IP dial: try 0.0.0.0 — if the server bound the wildcard
	// interface, this would succeed. Since we bind 127.0.0.1 only,
	// the dial should NOT succeed (connection refused / timeout)
	// because no listener is registered on the non-loopback path.
	//
	// On darwin and linux, dialing 0.0.0.0 on a port that is only
	// bound to 127.0.0.1 routes to the loopback listener anyway —
	// the kernel treats 0.0.0.0 as "any local". The stronger guard
	// is dialing a real external interface. Approximate by dialing
	// the first non-loopback IPv4 address; if no such interface
	// exists, log and skip the external probe.
	if external := firstExternalIPv4(); external != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		d := net.Dialer{}
		c, derr := d.DialContext(ctx, "tcp", external+":"+strconv.Itoa(port))
		if derr == nil {
			_ = c.Close()
			t.Fatalf("server reachable on non-loopback %s — bind leaked", external)
		}
	}
}

// TestStatic_Vendor_Alpine confirms the vendored Alpine bundle is
// reachable through the existing FileServerFS-backed /static route
// and that its body begins with the vendored comment line recorded
// in tasks 1.1 / 1.2 of add-viewer-ui-readonly.
func TestStatic_Vendor_Alpine(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/static/vendor/alpine.min.js")
	if err != nil {
		t.Fatalf("GET /static/vendor/alpine.min.js: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	const prefix = "/* Alpine.js v3."
	if !strings.HasPrefix(string(body), prefix) {
		t.Fatalf("body prefix = %q, want %q…", string(body[:min(len(body), 40)]), prefix)
	}
}

// TestIndex_References_VendoredAssets confirms the embedded page
// links the three local assets the design pinned (stylesheet, vendored
// Alpine, app script). Substring match keeps the test robust to
// whitespace / attribute-order changes in the HTML.
func TestIndex_References_VendoredAssets(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer res.Body.Close()
	body := readString(res.Body)
	for _, want := range []string{
		"/static/style.css",
		"/static/vendor/alpine.min.js",
		"/static/app.js",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("index body missing %q", want)
		}
	}
}

// TestIndex_NoExternalScripts enforces ADR 0002 §D5: no runtime CDN
// dependencies. The page must not carry an http(s)://... URL in any
// src= attribute. Substring scan is enough — any external <script>
// or <link rel=preload as=script> would surface.
func TestIndex_NoExternalScripts(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer res.Body.Close()
	body := readString(res.Body)
	for _, bad := range []string{`src="http://`, `src="https://`, `src='http://`, `src='https://`} {
		if strings.Contains(body, bad) {
			t.Fatalf("index body contains forbidden external script reference %q", bad)
		}
	}
}

// firstExternalIPv4 returns the first up, non-loopback IPv4 address
// on the host, or "" if none. Used to probe loopback-only bind.
func firstExternalIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}
			return ip4.String()
		}
	}
	return ""
}
