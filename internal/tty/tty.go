// Package tty offers a stdlib-only check for whether a file is a
// character device (i.e. a terminal). Used by interactive command paths
// (`ezida rm`) to decide whether to prompt the user.
package tty

import "os"

// IsTTY reports whether f is a character device. Returns false if Stat
// fails or f is nil. Works on macOS and Linux (sufficient for v1
// targets per ADR §D2).
func IsTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
