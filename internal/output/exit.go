package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// Exit code convention (ADR §D10).
const (
	ExitOK          = 0
	ExitUserError   = 1
	ExitSystemError = 2
)

// CodedError is implemented by typed CLI errors that carry their own
// stable error code and exit code. The output layer recognises this
// interface so we can keep typed errors in any package without an
// import cycle.
type CodedError interface {
	error
	// ErrorCode returns the UPPER_SNAKE stable identifier.
	ErrorCode() string
	// ExitCode returns the process exit code to use.
	ExitCode() int
	// Details returns optional structured detail for JSON envelopes.
	// Returning nil omits the "details" key.
	Details() map[string]any
}

// classify maps an error to the (code, exitCode, details) triple used
// by Fail. The mapping rules match the design's Fail table:
//
//   - *board.SchemaVersionError → SCHEMA_VERSION_MISMATCH, exit 1
//   - *board.ValidationError    → VALIDATION_FAILED, exit 1
//   - fs.ErrNotExist            → BOARD_NOT_FOUND, exit 1
//   - fs.ErrPermission          → IO_ERROR, exit 2
//   - CodedError                → as carried by the error
//   - everything else           → IO_ERROR, exit 2
func classify(err error) (code string, exit int, message string, details map[string]any) {
	if err == nil {
		return "", ExitOK, "", nil
	}

	// CodedError takes precedence so command-specific errors can carry
	// their own classification.
	var ce CodedError
	if errors.As(err, &ce) {
		return ce.ErrorCode(), ce.ExitCode(), ce.Error(), ce.Details()
	}

	var sv *board.SchemaVersionError
	if errors.As(err, &sv) {
		return "SCHEMA_VERSION_MISMATCH", ExitUserError, err.Error(), map[string]any{
			"file_version":      sv.FileVersion,
			"supported_version": sv.SupportedVersion,
		}
	}

	var ve *board.ValidationError
	if errors.As(err, &ve) {
		return "VALIDATION_FAILED", ExitUserError, err.Error(), nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		// Heuristic: most fs.ErrNotExist cases the CLI surfaces are a
		// missing kanban.toml. The message hint nudges toward `ezida init`.
		return "BOARD_NOT_FOUND",
			ExitUserError,
			"kanban.toml not found in this directory; run `ezida init` to create one",
			nil
	}

	if errors.Is(err, fs.ErrPermission) {
		return "IO_ERROR", ExitSystemError, err.Error(), nil
	}

	// UsageError-like errors raised by cobra (argument count, unknown
	// flag, unknown command) are user errors per ADR §D10. We detect
	// them by message prefix because cobra does not expose a sentinel.
	msg := err.Error()
	if isUsageError(msg) {
		return "USAGE_ERROR", ExitUserError, msg, nil
	}

	return "IO_ERROR", ExitSystemError, err.Error(), nil
}

// isUsageError reports whether msg looks like one of cobra's argument-
// or command-validation errors. The strings are stable across cobra
// versions and there is no exported sentinel; matching by prefix is
// the documented escape hatch.
func isUsageError(msg string) bool {
	for _, prefix := range []string{
		"accepts ", "requires ", "unknown command", "unknown flag",
		"unknown shorthand flag",
	} {
		if hasPrefix(msg, prefix) {
			return true
		}
	}
	return false
}

func hasPrefix(s, p string) bool {
	if len(s) < len(p) {
		return false
	}
	return s[:len(p)] == p
}

// Classify is the exported test seam for the classify mapping. It
// returns the (code, exitCode) pair only; the message and details are
// kept internal to Fail.
func Classify(err error) (code string, exit int) {
	c, e, _, _ := classify(err)
	return c, e
}

// Fail writes the error envelope to stderr and exits the process with
// the appropriate code.
//
// Tests that need to inspect the output without terminating should use
// FailTo, which writes to an arbitrary io.Writer and returns the exit
// code instead of calling os.Exit.
func Fail(err error, asJSON bool) {
	exit := FailTo(os.Stderr, err, asJSON)
	os.Exit(exit)
}

// FailTo is the testable variant of Fail.
func FailTo(stderr *os.File, err error, asJSON bool) int {
	code, exit, message, details := classify(err)
	if asJSON {
		env := map[string]any{
			"error": map[string]any{
				"code":    code,
				"message": message,
			},
		}
		if details != nil {
			env["error"].(map[string]any)["details"] = details
		}
		buf, mErr := json.Marshal(env)
		if mErr != nil {
			// Fallback: write the marshal error as plain text so we
			// never lose signal.
			fmt.Fprintf(stderr, "Error: %s\n", message)
			return exit
		}
		fmt.Fprintln(stderr, string(buf))
		return exit
	}
	fmt.Fprintf(stderr, "Error: %s\n", message)
	return exit
}
