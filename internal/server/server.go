package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// Options carries the runtime configuration for the viewer HTTP
// server. Run constructs a server from these and blocks until a
// terminating signal arrives.
type Options struct {
	// Port is the starting port for the bind loop. Zero falls back
	// to the default 7777.
	Port int
	// NoOpen skips the cross-platform browser open helper.
	NoOpen bool
	// Board is the filesystem path to the kanban.toml file used by
	// the API handlers; typically "kanban.toml" in the current
	// working directory.
	Board string
}

// defaultStartPort is the conventional Ezida viewer port (ADR 0002
// §D6). Used when opts.Port is zero.
const defaultStartPort = 7777

// portFallbackWindow is the number of consecutive ports tried before
// giving up (ADR 0002 §D6).
const portFallbackWindow = 11

// shutdownTimeout is the drain budget for the graceful-shutdown path
// triggered by SIGINT/SIGTERM (ADR 0002 §D12).
const shutdownTimeout = 5 * time.Second

// commandRunner abstracts the browser-open side effect so tests can
// observe whether Run attempted to spawn a browser. Production code
// uses execCommandRunner which delegates to exec.Command (see
// browser.go).
type commandRunner interface {
	Open(url string) error
}

// runnerForOpen is the package-level seam for browser-launch
// behavior. Tests overwrite it to record calls; production code
// leaves it pointing at execCommandRunner{}.
var runnerForOpen commandRunner = execCommandRunner{}

// stdoutForRun and stderrForRun are package-level seams so tests can
// capture banner / warning output without manipulating os.Stdout /
// os.Stderr directly.
var (
	stdoutForRun = os.Stdout.WriteString
	stderrForRun = func(s string) (int, error) { return os.Stderr.WriteString(s) }
)

// Run boots the viewer HTTP server on 127.0.0.1, opens the user's
// browser unless NoOpen is set, then blocks until SIGINT/SIGTERM.
//
// Returns nil on clean shutdown, *PortUnavailableError if no port in
// the fallback window is free, and the underlying error for any
// other listener failure (e.g. permission denied).
func Run(opts Options) error {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return runWithContext(ctx, opts)
}

// runWithContext is the testable boot path. It blocks until ctx is
// done, then drains in-flight requests within shutdownTimeout and
// returns. Production callers use Run, which wires ctx to
// SIGINT/SIGTERM; tests pass a context they cancel directly.
func runWithContext(ctx context.Context, opts Options) error {
	start := opts.Port
	if start == 0 {
		start = defaultStartPort
	}

	boardPath := opts.Board
	if boardPath == "" {
		boardPath = "kanban.toml"
	}

	// Build the watcher BEFORE binding the listener so a missing
	// board file fails fast without occupying a port (ADR 0002 §D9 —
	// watcher startup is part of the bring-up contract).
	watcher, err := NewWatcher(boardPath)
	if err != nil {
		return err
	}

	ln, port, err := bindWithFallback(start, portFallbackWindow)
	if err != nil {
		// Release the watcher's fsnotify handle since we won't be
		// starting Run; Watcher.Run normally closes it on exit.
		watcher.fsw.Close()
		return err
	}

	broker := NewBroker()
	s := &serverState{boardPath: boardPath, broker: broker}

	mux := http.NewServeMux()
	s.routes(mux)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	url := "http://127.0.0.1:" + strconv.Itoa(port)
	_, _ = stdoutForRun("→ Ezida viewer running at " + url + "\n")

	if !opts.NoOpen {
		if oerr := runnerForOpen.Open(url); oerr != nil {
			_, _ = stderrForRun("warning: failed to open browser: " + oerr.Error() + "\n")
		}
	}

	// Watcher → broker pump. Watcher.Run closes its events channel
	// only indirectly (the channel itself is owned for the
	// lifetime); we stop the pump when ctx is cancelled by relying
	// on Run returning, after which no further events can fire.
	go watcher.Run(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-watcher.Events():
				broker.Broadcast()
			}
		}
	}()

	shutdownDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		// Evict every SSE client first so Shutdown's drain does not
		// block on long-lived event streams (each handler returns as
		// soon as its broker channel closes).
		broker.Close()
		sdCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(sdCtx)
		close(shutdownDone)
	}()

	serveErr := srv.Serve(ln)
	if errors.Is(serveErr, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return serveErr
}

// bindWithFallback tries net.Listen on 127.0.0.1:<p> for p in
// [start, start+window). Returns the first successful listener and
// chosen port. Returns *PortUnavailableError when every port in the
// window is busy with EADDRINUSE; surfaces any other listen error
// immediately.
func bindWithFallback(start, window int) (net.Listener, int, error) {
	for p := start; p < start+window; p++ {
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
		if err == nil {
			return ln, p, nil
		}
		if !isAddrInUse(err) {
			return nil, 0, err
		}
	}
	return nil, 0, &PortUnavailableError{Start: start, Window: window}
}

// isAddrInUse reports whether err is a "address already in use"
// error from net.Listen. Uses errors.As against *net.OpError ->
// *os.SyscallError so platform errno wrapping is handled correctly
// (avoids string matching).
func isAddrInUse(err error) bool {
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	var sysErr *os.SyscallError
	if !errors.As(opErr.Err, &sysErr) {
		return false
	}
	// syscall.EADDRINUSE on darwin/linux.
	return errors.Is(sysErr.Err, syscall.EADDRINUSE)
}

// serverState carries the per-server dependencies the HTTP handlers
// close over: the board file path, the SSE broker (V4), and any
// future seams (clocks, loaders, etc.). Kept private so tests share
// construction via the startTestServer helper.
type serverState struct {
	boardPath string
	broker    *Broker
}

// PortUnavailableError is returned by Run when every port in the
// fallback window starting at Start is already bound. The CLI's
// JSON-mode error envelope surfaces it as code "PORT_UNAVAILABLE"
// per ADR 0002 §D6.
type PortUnavailableError struct {
	Start  int
	Window int
}

// Error renders a human-readable description listing the range tried.
func (e *PortUnavailableError) Error() string {
	return fmt.Sprintf(
		"no free port in range %d..%d",
		e.Start, e.Start+e.Window-1,
	)
}

// Code returns the stable UPPER_SNAKE identifier surfaced by
// output.Fail when --json is set.
func (e *PortUnavailableError) Code() string { return "PORT_UNAVAILABLE" }

// ShortMessage is the one-line JSON-mode message.
func (e *PortUnavailableError) ShortMessage() string { return e.Error() }

// Details is the JSON-mode `error.details` payload.
func (e *PortUnavailableError) Details() any {
	return map[string]any{
		"start":  e.Start,
		"window": e.Window,
	}
}
