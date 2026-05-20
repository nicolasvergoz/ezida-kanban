package output

import (
	"os"
	"strings"
)

// colorEnabled is the resolved decision for whether text-mode output
// may include ANSI escape sequences. v1 does not actually emit color
// (per task 8.3) but the gate is wired so future phases can opt in
// without touching every command.
var colorEnabled bool

// ConfigureColor resolves whether text-mode output may colorize, based
// on the --no-color flag, the NO_COLOR env var, and stdout's TTY
// status. JSON output ignores this entirely.
//
//   - forceNoColor: the --no-color flag's value. true disables.
//   - NO_COLOR (any non-empty value): disables.
//   - stdout not a character device (piped/redirected): disables.
//   - otherwise: enabled.
func ConfigureColor(forceNoColor bool) {
	colorEnabled = ResolveColor(forceNoColor, os.Getenv("NO_COLOR"), stdoutIsTTY())
}

// ColorEnabled is the resolved value, exposed for tests and for future
// color-aware renderers.
func ColorEnabled() bool { return colorEnabled }

// ResolveColor is the pure decision function, exported for tests
// (and for callers that want to compute the resolved value without
// touching the package-level state).
func ResolveColor(forceNoColor bool, noColorEnv string, isTTY bool) bool {
	if forceNoColor {
		return false
	}
	if noColorEnv != "" {
		return false
	}
	if !isTTY {
		return false
	}
	return true
}

// stdoutIsTTY reports whether os.Stdout is a character device.
func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// KV is one key/value pair for the KeyValue renderer.
type KV struct {
	Key   string
	Value string
}

// Table renders a rows-of-strings table with optional headers,
// aligning every column to the width of its widest cell (including
// the header). Cells are separated by two spaces. Empty cells are
// rendered as the empty string (callers should pre-substitute "-"
// for missing values where the spec requires it).
//
// The returned string ends with a newline.
func Table(rows [][]string, headers []string) string {
	// Compute per-column widths.
	width := func(s string) int { return len(s) }
	ncols := 0
	if len(headers) > ncols {
		ncols = len(headers)
	}
	for _, r := range rows {
		if len(r) > ncols {
			ncols = len(r)
		}
	}
	widths := make([]int, ncols)
	for i, h := range headers {
		if width(h) > widths[i] {
			widths[i] = width(h)
		}
	}
	for _, r := range rows {
		for i, cell := range r {
			if width(cell) > widths[i] {
				widths[i] = width(cell)
			}
		}
	}

	var b strings.Builder
	writeRow := func(cells []string) {
		for i := 0; i < ncols; i++ {
			var cell string
			if i < len(cells) {
				cell = cells[i]
			}
			b.WriteString(cell)
			if i < ncols-1 {
				// Pad to column width, then 2 spaces.
				pad := widths[i] - width(cell)
				if pad < 0 {
					pad = 0
				}
				for j := 0; j < pad; j++ {
					b.WriteByte(' ')
				}
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}
	if len(headers) > 0 {
		writeRow(headers)
	}
	for _, r := range rows {
		writeRow(r)
	}
	return b.String()
}

// KeyValue renders aligned `Key:   Value` lines. The keys are
// right-padded so every colon column aligns to the widest key.
//
// The returned string ends with a newline.
func KeyValue(pairs []KV) string {
	maxKey := 0
	for _, kv := range pairs {
		if len(kv.Key) > maxKey {
			maxKey = len(kv.Key)
		}
	}
	var b strings.Builder
	for _, kv := range pairs {
		b.WriteString(kv.Key)
		b.WriteByte(':')
		// Pad so values align to (maxKey + 3) total column.
		pad := maxKey - len(kv.Key) + 3
		for j := 0; j < pad; j++ {
			b.WriteByte(' ')
		}
		b.WriteString(kv.Value)
		b.WriteByte('\n')
	}
	return b.String()
}
