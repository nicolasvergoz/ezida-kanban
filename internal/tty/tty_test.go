package tty_test

import (
	"os"
	"testing"

	"github.com/nicolasvergoz/ezida-kanban/internal/tty"
)

func TestIsTTY_RegularFileReturnsFalse(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "tty-test-*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer f.Close()
	if tty.IsTTY(f) {
		t.Errorf("IsTTY(tempfile) = true, want false")
	}
}

func TestIsTTY_NilReturnsFalse(t *testing.T) {
	if tty.IsTTY(nil) {
		t.Errorf("IsTTY(nil) = true, want false")
	}
}
