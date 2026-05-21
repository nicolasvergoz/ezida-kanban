package server

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Open launches the user's default browser at url using a
// platform-appropriate command:
//
//   - darwin: `open <url>`
//   - linux: `xdg-open <url>`
//
// Any other GOOS returns an error; v1 explicitly excludes Windows
// support per ADR 0001 §D2. The launched command runs detached;
// Open returns as soon as the helper process has started (or errored
// out at exec time). It does not wait for the browser to load.
func Open(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// execCommandRunner is the production implementation of commandRunner
// (declared in server.go). It simply delegates to Open; tests
// substitute a stub that records calls and skips exec.
type execCommandRunner struct{}

// Open implements commandRunner via the package-level Open helper.
func (execCommandRunner) Open(url string) error { return Open(url) }
