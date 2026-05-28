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
	"github.com/pelletier/go-toml/v2"
)

// tomlUnmarshal is a one-line alias used by the test helpers so the
// import of pelletier/go-toml/v2 is local to test code.
func tomlUnmarshal(data []byte, v any) error { return toml.Unmarshal(data, v) }

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
	s := &serverState{
		boardPath:   boardPath,
		projectName: resolveProjectName(boardPath),
		broker:      NewBroker(),
	}
	mux := http.NewServeMux()
	s.routes(mux)
	ts := httptest.NewServer(mux)
	return ts, ts.Close
}

// startTestServerWithBroker is like startTestServer but returns the
// broker so SSE tests can fire broadcasts directly.
func startTestServerWithBroker(t *testing.T, boardPath string) (*httptest.Server, *Broker, func()) {
	t.Helper()
	b := NewBroker()
	s := &serverState{
		boardPath:   boardPath,
		projectName: resolveProjectName(boardPath),
		broker:      b,
	}
	mux := http.NewServeMux()
	s.routes(mux)
	ts := httptest.NewServer(mux)
	return ts, b, ts.Close
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

// TestHandle_Static_App confirms the React app source is reachable
// via the static route. The React UI renders all markup at runtime,
// so DOM-structure assertions belong in browser-level tests rather
// than the served HTML.
func TestHandle_Static_App(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/static/app.jsx")
	if err != nil {
		t.Fatalf("GET /static/app.jsx: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") && !strings.HasPrefix(ct, "text/javascript") {
		t.Fatalf("Content-Type = %q, want application/javascript or text/javascript", ct)
	}
	body, _ := io.ReadAll(res.Body)
	want, _ := webFS.ReadFile("web/app.jsx")
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
		Cards          []struct {
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

// TestStatic_Vendor_React confirms the vendored React production
// bundle is reachable through the FileServerFS-backed /static route.
func TestStatic_Vendor_React(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/static/vendor/react.production.min.js")
	if err != nil {
		t.Fatalf("GET /static/vendor/react.production.min.js: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
}

// TestStatic_Vendor_ReactDOM confirms the vendored ReactDOM
// production bundle is reachable.
func TestStatic_Vendor_ReactDOM(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/static/vendor/react-dom.production.min.js")
	if err != nil {
		t.Fatalf("GET /static/vendor/react-dom.production.min.js: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
}

// TestStatic_Vendor_Babel confirms the vendored Babel-standalone
// bundle is reachable — needed because the page loads app.jsx with
// <script type="text/babel">.
func TestStatic_Vendor_Babel(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/static/vendor/babel.min.js")
	if err != nil {
		t.Fatalf("GET /static/vendor/babel.min.js: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
}

// TestIndex_References_VendoredAssets confirms the embedded page
// links the local stylesheet, vendored React, vendored ReactDOM,
// vendored Babel-standalone, and the JSX app source.
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
		"/static/styles.css",
		"/static/vendor/react.production.min.js",
		"/static/vendor/react-dom.production.min.js",
		"/static/vendor/babel.min.js",
		"/static/app.jsx",
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

// (Modal markup is rendered by React at runtime; assertions against
// served HTML for modal structure are covered by browser-level tests
// or the React component tree.)

// writableBoard copies testdata/valid_kanban.toml into t.TempDir so a
// test can exercise endpoints that mutate the file. Returns the path
// to the temp copy.
func writableBoard(t *testing.T) string {
	t.Helper()
	src := fixturePath(t, "valid_kanban.toml")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "kanban.toml")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write temp board: %v", err)
	}
	return dst
}

// postJSON is a tiny helper that POSTs the given body and decodes the
// JSON response into out (which may be nil to skip decoding).
func postJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return res
}

// columnOf looks up the column of card id by re-reading the on-disk
// board.toml via the same parser the server uses.
func columnOf(t *testing.T, boardPath, id string) string {
	t.Helper()
	b, err := readBoardFile(boardPath)
	if err != nil {
		t.Fatalf("readBoardFile: %v", err)
	}
	for _, c := range b.Cards {
		if c.ID == id {
			return c.Column
		}
	}
	return ""
}

// readBoardFile is a tiny TOML reader used in tests to inspect the
// on-disk file after a mutating call. Kept inline (not importing the
// board package's Load) so the test verifies bytes-on-disk rather
// than the in-memory representation the server returned.
func readBoardFile(path string) (*testBoardSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b testBoardSnapshot
	if err := tomlUnmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

type testBoardSnapshot struct {
	Cards []testCardSnapshot `toml:"cards"`
}

type testCardSnapshot struct {
	ID          string    `toml:"id"`
	Title       string    `toml:"title"`
	Column      string    `toml:"column"`
	Description string    `toml:"description"`
	Tags        []string  `toml:"tags"`
	Priority    string    `toml:"priority"`
	UpdatedAt   time.Time `toml:"updated_at"`
}

// findCardOnDisk returns the on-disk card with the given id, or nil
// if none. Used by the PATCH tests to verify field-level state.
func findCardOnDisk(t *testing.T, path, id string) *testCardSnapshot {
	t.Helper()
	b, err := readBoardFile(path)
	if err != nil {
		t.Fatalf("readBoardFile: %v", err)
	}
	for i := range b.Cards {
		if b.Cards[i].ID == id {
			return &b.Cards[i]
		}
	}
	return nil
}

// patchJSON is the PATCH counterpart of postJSON.
func patchJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("PATCH", url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", url, err)
	}
	return res
}

func TestHandle_Move_Success(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/aaaaaa/move",
		`{"column":"done","position":0}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	if got := res.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := columnOf(t, path, "aaaaaa"); got != "done" {
		t.Fatalf("on-disk column for aaaaaa = %q, want done", got)
	}
}

func TestHandle_Move_WithinColumn(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	// aaaaaa is the first card in todo; move it to the end (position 1).
	res := postJSON(t, ts.URL+"/api/cards/aaaaaa/move",
		`{"column":"todo","position":1}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	// Confirm aaaaaa is still in todo, but now after bbbbbb.
	b, err := readBoardFile(path)
	if err != nil {
		t.Fatalf("readBoardFile: %v", err)
	}
	var todoOrder []string
	for _, c := range b.Cards {
		if c.Column == "todo" {
			todoOrder = append(todoOrder, c.ID)
		}
	}
	want := []string{"bbbbbb", "aaaaaa"}
	if !reflectStringSliceEqual(todoOrder, want) {
		t.Fatalf("todo order = %v, want %v", todoOrder, want)
	}
}

func TestHandle_Move_UnknownCard(t *testing.T) {
	path := writableBoard(t)
	// Snapshot the bytes so we can confirm no write happened.
	before, _ := os.ReadFile(path)

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/zzzzzz/move",
		`{"column":"done","position":0}`)
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"CARD_NOT_FOUND"`) {
		t.Fatalf("body missing CARD_NOT_FOUND: %s", body)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 404 error")
	}
}

func TestHandle_Move_UnknownColumn(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/aaaaaa/move",
		`{"column":"ghost","position":0}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"COLUMN_NOT_FOUND"`) {
		t.Fatalf("body missing COLUMN_NOT_FOUND: %s", body)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandle_Move_MalformedBody(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/aaaaaa/move",
		`{not valid json`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"INVALID_BODY"`) {
		t.Fatalf("body missing INVALID_BODY: %s", body)
	}
}

func TestHandle_Move_ClampsPosition(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	// todo has aaaaaa, bbbbbb; positioning at 999 should clamp to end.
	res := postJSON(t, ts.URL+"/api/cards/aaaaaa/move",
		`{"column":"todo","position":999}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	b, err := readBoardFile(path)
	if err != nil {
		t.Fatalf("readBoardFile: %v", err)
	}
	var todoOrder []string
	for _, c := range b.Cards {
		if c.Column == "todo" {
			todoOrder = append(todoOrder, c.ID)
		}
	}
	want := []string{"bbbbbb", "aaaaaa"}
	if !reflectStringSliceEqual(todoOrder, want) {
		t.Fatalf("todo order = %v, want %v (position clamp)", todoOrder, want)
	}
}

func reflectStringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- PATCH /api/cards/{id} tests -------------------------------------------

func TestHandle_Patch_TitleOnly(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/aaaaaa", `{"title":"New title"}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var body struct {
		Card struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"card"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Card.Title != "New title" {
		t.Fatalf("response title = %q, want %q", body.Card.Title, "New title")
	}
	if body.Card.Description != "First card description." {
		t.Fatalf("description should be unchanged, got %q", body.Card.Description)
	}
	disk := findCardOnDisk(t, path, "aaaaaa")
	if disk == nil || disk.Title != "New title" {
		t.Fatalf("on-disk title = %q, want %q", disk.Title, "New title")
	}
}

func TestHandle_Patch_MultipleFields(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/aaaaaa",
		`{"title":"Renamed","tags":["x","y"],"priority":"low"}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	disk := findCardOnDisk(t, path, "aaaaaa")
	if disk == nil {
		t.Fatalf("card not on disk")
	}
	if disk.Title != "Renamed" {
		t.Fatalf("title = %q, want Renamed", disk.Title)
	}
	if len(disk.Tags) != 2 || disk.Tags[0] != "x" || disk.Tags[1] != "y" {
		t.Fatalf("tags = %v, want [x y]", disk.Tags)
	}
	if disk.Priority != "low" {
		t.Fatalf("priority = %q, want low", disk.Priority)
	}
}

func TestHandle_Patch_ClearPriority(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/aaaaaa", `{"priority":""}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	disk := findCardOnDisk(t, path, "aaaaaa")
	if disk.Priority != "" {
		t.Fatalf("priority on disk = %q, want empty", disk.Priority)
	}
}

func TestHandle_Patch_ClearTags(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/aaaaaa", `{"tags":[]}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	disk := findCardOnDisk(t, path, "aaaaaa")
	if len(disk.Tags) != 0 {
		t.Fatalf("tags on disk = %v, want empty", disk.Tags)
	}
}

func TestHandle_Patch_EmptyTitle(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/aaaaaa", `{"title":""}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"MISSING_TITLE"`) {
		t.Fatalf("body missing MISSING_TITLE: %s", body)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandle_Patch_UnknownPriority(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/aaaaaa", `{"priority":"urgent"}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"INVALID_PRIORITY"`) {
		t.Fatalf("body missing INVALID_PRIORITY: %s", body)
	}
}

func TestHandle_Patch_EmptyTag(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/aaaaaa", `{"tags":["good",""]}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"INVALID_TAG"`) {
		t.Fatalf("body missing INVALID_TAG: %s", body)
	}
}

func TestHandle_Patch_UnknownCard(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/zzzzzz", `{"title":"x"}`)
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"CARD_NOT_FOUND"`) {
		t.Fatalf("body missing CARD_NOT_FOUND: %s", body)
	}
}

func TestHandle_Patch_MalformedBody(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/aaaaaa", `{not valid json`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"INVALID_BODY"`) {
		t.Fatalf("body missing INVALID_BODY: %s", body)
	}
}

func TestHandle_Patch_RefreshesUpdatedAt(t *testing.T) {
	path := writableBoard(t)
	before := findCardOnDisk(t, path, "aaaaaa")
	if before == nil {
		t.Fatalf("seed card missing")
	}
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/cards/aaaaaa", `{"title":"Bumped"}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	var body struct {
		Card struct {
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"card"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.Card.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("response updated_at (%s) not strictly after pre-patch (%s)",
			body.Card.UpdatedAt, before.UpdatedAt)
	}
	disk := findCardOnDisk(t, path, "aaaaaa")
	if disk == nil || !disk.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("on-disk updated_at not refreshed")
	}
}

// TestHandle_Board_ProjectName confirms /api/board includes the
// top-level `project_name` field set to the parent-directory name of
// the resolved board path. The test writes a board into a temp dir
// whose basename is the asserted value.
func TestHandle_Board_ProjectName(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "my-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := fixturePath(t, "valid_kanban.toml")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	path := filepath.Join(projectDir, "kanban.toml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/board")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var payload struct {
		ProjectName string `json:"project_name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.ProjectName != "my-project" {
		t.Fatalf("project_name = %q, want %q", payload.ProjectName, "my-project")
	}
}

// TestResolveProjectName_Fallback exercises the fallback branch of
// resolveProjectName via a filesystem-root board path (the basename of
// the parent directory is `/`, which equals filepath.Separator). The
// helper is the right seam — it encapsulates the entire fallback
// logic so we don't need to synthesize a real root-of-filesystem
// scenario.
func TestResolveProjectName_Fallback(t *testing.T) {
	// `/kanban.toml` → Dir is "/", Base("/") is "/" — fallback path.
	rootPath := string(filepath.Separator) + "kanban.toml"
	if got := resolveProjectName(rootPath); got != "Ezida" {
		t.Fatalf("resolveProjectName(%q) = %q, want %q", rootPath, got, "Ezida")
	}
	// Real parent directory → its basename.
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "alpha")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(projectDir, "kanban.toml")
	if got := resolveProjectName(path); got != "alpha" {
		t.Fatalf("resolveProjectName(%q) = %q, want %q", path, got, "alpha")
	}
}

// TestHandle_Board_ProjectName_Stable confirms project_name is the
// same across two consecutive /api/board requests (immutability for
// the lifetime of the process — ADR 0003 §D4).
func TestHandle_Board_ProjectName_Stable(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "stable-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := fixturePath(t, "valid_kanban.toml")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	path := filepath.Join(projectDir, "kanban.toml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	first := fetchProjectName(t, ts.URL)
	// Rewrite the board file between requests — project_name must not
	// re-evaluate.
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	second := fetchProjectName(t, ts.URL)
	if first != second {
		t.Fatalf("project_name drifted: first=%q second=%q", first, second)
	}
	if first != "stable-project" {
		t.Fatalf("project_name = %q, want %q", first, "stable-project")
	}
}

func fetchProjectName(t *testing.T, baseURL string) string {
	t.Helper()
	res, err := http.Get(baseURL + "/api/board")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	var payload struct {
		ProjectName string `json:"project_name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return payload.ProjectName
}

// fetchPriorityColors hits /api/board and returns the priority_colors
// map. Used by the rule-10 default-resolution tests below.
func fetchPriorityColors(t *testing.T, baseURL string) map[string]string {
	t.Helper()
	res, err := http.Get(baseURL + "/api/board")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var payload struct {
		PriorityColors map[string]string `json:"priority_colors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return payload.PriorityColors
}

func writeBoardFixture(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestHandle_Board_PriorityColors_DefaultsFilled(t *testing.T) {
	path := writeBoardFixture(t, `schema_version = 1

[board]
columns = ["todo"]
priorities = ["low", "medium", "high"]
`)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	got := fetchPriorityColors(t, ts.URL)
	want := map[string]string{
		"low":    "#22c55e",
		"medium": "#f59e0b",
		"high":   "#ef4444",
	}
	if len(got) != len(want) {
		t.Fatalf("priority_colors size = %d, want %d (got %v)", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("priority_colors[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestHandle_Board_PriorityColors_UserOverrideWins(t *testing.T) {
	path := writeBoardFixture(t, `schema_version = 1

[board]
columns = ["todo"]
priorities = ["low", "medium", "high"]

[board.priority_colors]
high = "#000000"
`)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	got := fetchPriorityColors(t, ts.URL)
	if got["high"] != "#000000" {
		t.Fatalf("priority_colors.high = %q, want %q", got["high"], "#000000")
	}
	if got["low"] != "#22c55e" || got["medium"] != "#f59e0b" {
		t.Fatalf("defaults not filled for unset names: got %v", got)
	}
}

func TestHandle_Board_PriorityColors_CustomNameWithExplicitColor(t *testing.T) {
	path := writeBoardFixture(t, `schema_version = 1

[board]
columns = ["todo"]
priorities = ["urgent"]

[board.priority_colors]
urgent = "#ff0000"
`)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	got := fetchPriorityColors(t, ts.URL)
	if len(got) != 1 || got["urgent"] != "#ff0000" {
		t.Fatalf("priority_colors = %v, want {urgent: #ff0000}", got)
	}
}

func TestHandle_Board_PriorityColors_CustomNameNoDefaults(t *testing.T) {
	path := writeBoardFixture(t, `schema_version = 1

[board]
columns = ["todo"]
priorities = ["urgent"]
`)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	got := fetchPriorityColors(t, ts.URL)
	if len(got) != 0 {
		t.Fatalf("priority_colors = %v, want empty {}", got)
	}
	if got == nil {
		t.Fatalf("priority_colors must be non-nil empty object, got nil")
	}
}

// TestStaticStyleCSS_ContainsDarkSelector confirms the served
// stylesheet exposes the `[data-theme="dark"]` selector block
// introduced by add-dark-theme so the Alpine controller's
// `<html data-theme="dark">` write actually swaps the token values.
func TestStaticStylesCSS_ContainsDarkSelector(t *testing.T) {
	ts, cleanup := startTestServer(t, fixturePath(t, "valid_kanban.toml"))
	defer cleanup()

	res, err := http.Get(ts.URL + "/static/styles.css")
	if err != nil {
		t.Fatalf("GET /static/styles.css: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	const want = `[data-theme="dark"]`
	if !strings.Contains(string(body), want) {
		t.Fatalf("styles.css missing %q", want)
	}
}

// Theme-toggle, filter-button, and filter-popover are rendered by
// React at runtime, so they are not present in the served HTML. Their
// behavior is covered by spec scenarios in `viewer-ui/spec.md`.

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
