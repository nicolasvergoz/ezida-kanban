package skill

import (
	"bytes"
	"os"
	"testing"
)

// TestBytes_MatchesFile confirms the embedded Bytes are byte-identical
// to the on-disk SKILL.md beside skill.go. If they drift, somebody
// edited the file without rebuilding, or two sources of truth slipped
// in — both bugs.
func TestBytes_MatchesFile(t *testing.T) {
	onDisk, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	if len(onDisk) != len(Bytes) {
		t.Fatalf("length mismatch: file=%d, embedded=%d", len(onDisk), len(Bytes))
	}
	if !bytes.Equal(onDisk, Bytes) {
		t.Fatal("embedded bytes differ from SKILL.md on disk")
	}
}

// TestBytes_NoPythonReferences guards against the Python fallback
// block sneaking back in via a hand-edit.
func TestBytes_NoPythonReferences(t *testing.T) {
	if bytes.Contains(Bytes, []byte("python <skill-directory>")) {
		t.Error("Bytes still contain the Python fallback reference")
	}
}

// TestBytes_NoPipReferences guards against the original "installed via
// pip" wording sneaking back in.
func TestBytes_NoPipReferences(t *testing.T) {
	if bytes.Contains(Bytes, []byte("installed via pip")) {
		t.Error("Bytes still mention pip installation")
	}
}

// TestBytes_MentionsInstallScript confirms the patched wording is
// present.
func TestBytes_MentionsInstallScript(t *testing.T) {
	if !bytes.Contains(Bytes, []byte("installed via the install script")) {
		t.Error("Bytes do not mention the install script")
	}
}
