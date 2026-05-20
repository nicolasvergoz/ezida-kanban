package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// DetailedError is the richer P4 error contract. Typed errors that need
// to carry a structured details payload distinct from their text
// rendering (e.g. ColumnInUseError with its embedded card list) implement
// this interface. The output layer recognises it via errors.As so the
// commands package can keep its concrete types private.
//
//   - Code returns the stable UPPER_SNAKE error code.
//   - Details returns the JSON-mode `error.details` payload.
//   - ShortMessage returns the one-line JSON-mode message.
//   - Error() (inherited from error) returns the full text-mode rendering.
type DetailedError interface {
	error
	Code() string
	Details() any
	ShortMessage() string
}

// classify maps an error to the (code, exitCode, message, jsonMessage, details, textBody) tuple
// used by Fail.
//
// For DetailedError values the JSON message is ShortMessage() and the
// text body is Error(). For legacy CodedError values both JSON message
// and text body equal Error() and details come from the legacy map.
func classify(err error) (code string, exit int, textBody string, jsonMessage string, jsonDetails any) {
	if err == nil {
		return "", ExitOK, "", "", nil
	}

	// DetailedError takes priority over CodedError: the P4 errors
	// implement both interfaces (legacy CodedError methods are absent),
	// so this branch only catches the new typed errors.
	var de DetailedError
	if errors.As(err, &de) {
		return de.Code(), ExitUserError, de.Error(), de.ShortMessage(), de.Details()
	}

	// CodedError takes precedence so command-specific errors can carry
	// their own classification.
	var ce CodedError
	if errors.As(err, &ce) {
		return ce.ErrorCode(), ce.ExitCode(), ce.Error(), ce.Error(), mapToAny(ce.Details())
	}

	var sv *board.SchemaVersionError
	if errors.As(err, &sv) {
		return "SCHEMA_VERSION_MISMATCH", ExitUserError, err.Error(), err.Error(), map[string]any{
			"file_version":      sv.FileVersion,
			"supported_version": sv.SupportedVersion,
		}
	}

	var ve *board.ValidationError
	if errors.As(err, &ve) {
		return "VALIDATION_FAILED", ExitUserError, err.Error(), err.Error(), nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		// Heuristic: most fs.ErrNotExist cases the CLI surfaces are a
		// missing kanban.toml. The message hint nudges toward `ezida init`.
		msg := "kanban.toml not found in this directory; run `ezida init` to create one"
		return "BOARD_NOT_FOUND", ExitUserError, msg, msg, nil
	}

	if errors.Is(err, fs.ErrPermission) {
		return "IO_ERROR", ExitSystemError, err.Error(), err.Error(), nil
	}

	// UsageError-like errors raised by cobra (argument count, unknown
	// flag, unknown command) are user errors per ADR §D10. We detect
	// them by message prefix because cobra does not expose a sentinel.
	msg := err.Error()
	if isUsageError(msg) {
		return "USAGE_ERROR", ExitUserError, msg, msg, nil
	}

	return "IO_ERROR", ExitSystemError, err.Error(), err.Error(), nil
}

// mapToAny normalises a legacy details map (used by CodedError) into the
// any payload the JSON envelope accepts. nil/empty maps become nil so
// the "details" key is omitted.
func mapToAny(m map[string]any) any {
	if len(m) == 0 {
		return nil
	}
	return m
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
	c, e, _, _, _ := classify(err)
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
//
// In text mode it writes `Error: <Error()>\n`. In JSON mode it emits
// `{"error":{"code":...,"message":<ShortMessage>,"details":<Details>}}\n`.
func FailTo(stderr io.Writer, err error, asJSON bool) int {
	code, exit, textBody, jsonMessage, details := classify(err)
	if asJSON {
		inner := map[string]any{
			"code":    code,
			"message": jsonMessage,
		}
		if details != nil {
			inner["details"] = details
		}
		buf, mErr := json.Marshal(map[string]any{"error": inner})
		if mErr != nil {
			// Fallback: write the marshal error as plain text so we
			// never lose signal.
			fmt.Fprintf(stderr, "Error: %s\n", jsonMessage)
			return exit
		}
		fmt.Fprintln(stderr, string(buf))
		return exit
	}
	fmt.Fprintf(stderr, "Error: %s\n", textBody)
	return exit
}
