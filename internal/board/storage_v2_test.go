package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempBoard writes body to a fresh temp kanban.toml and returns
// its path.
func writeTempBoard(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kanban.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

const v2Fixture = `schema_version = 2

[board]
columns = ['backlog', 'todo', 'ongoing', 'done*']
priorities = ['low', 'high']

[[cards]]
id = 'rl4m9x'
title = 'Card relations'
column = 'todo'
description = ''
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
color = '#8b5cf6'

[[cards]]
id = 'f20wbo'
title = 'Card dependencies'
column = 'done'
description = ''
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
epic = 'rl4m9x'
`

func TestLoad_DecodesTerminalMarker(t *testing.T) {
	b, err := Load(writeTempBoard(t, v2Fixture))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := []string{"backlog", "todo", "ongoing", "done"}
	if !equalStringSlices(b.Board.Columns, want) {
		t.Fatalf("Columns = %v, want %v", b.Board.Columns, want)
	}
	if !b.Board.IsDoneColumn("done") {
		t.Error("done not reported as terminal")
	}
	if b.Board.IsDoneColumn("todo") {
		t.Error("todo reported as terminal")
	}
}

func TestSave_ReEncodesTerminalMarkerAndFields(t *testing.T) {
	path := writeTempBoard(t, v2Fixture)
	b, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := Save(path, b); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	body := string(out)

	for _, want := range []string{
		"'done*'",           // the marker survives the round-trip
		"epic = 'rl4m9x'",   // and so do the v2 card fields
		"color = '#8b5cf6'", //
		ColumnsComment,      // the file explains its own encoding
	} {
		if !strings.Contains(body, want) {
			t.Errorf("saved file is missing %q:\n%s", want, body)
		}
	}
	// A card in the terminal column stores the bare name.
	if strings.Contains(body, "column = 'done*'") {
		t.Errorf("a card stored the terminal marker:\n%s", body)
	}
	// Derived values are never persisted. (Matched as keys, since the
	// explanatory comment legitimately contains the word "progress".)
	for _, forbidden := range []string{"children =", "progress =", "done =", "total ="} {
		if strings.Contains(body, forbidden) {
			t.Errorf("saved file contains derived value %q:\n%s", forbidden, body)
		}
	}
	// Saving must not leave the in-memory board holding encoded names.
	if !equalStringSlices(b.Board.Columns, []string{"backlog", "todo", "ongoing", "done"}) {
		t.Errorf("Save mutated the caller's columns: %v", b.Board.Columns)
	}
}

// Two saves in a row must produce identical bytes — in particular, the
// explanatory comment must be written once, not accumulated.
func TestSave_IsIdempotent(t *testing.T) {
	path := writeTempBoard(t, v2Fixture)
	b, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := Save(path, b); err != nil {
		t.Fatalf("first save: %v", err)
	}
	first, _ := os.ReadFile(path)

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if err := Save(path, reloaded); err != nil {
		t.Fatalf("second save: %v", err)
	}
	second, _ := os.ReadFile(path)

	if string(first) != string(second) {
		t.Fatalf("save is not idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if n := strings.Count(string(second), ColumnsComment); n != 1 {
		t.Fatalf("comment appears %d times, want 1", n)
	}
}

func TestSave_AbsentEpicAndColorRoundTripAsAbsent(t *testing.T) {
	path := writeTempBoard(t, strings.Replace(
		strings.Replace(v2Fixture, "color = '#8b5cf6'\n", "", 1),
		"epic = 'rl4m9x'\n", "", 1))
	b, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := Save(path, b); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, _ := os.ReadFile(path)
	if strings.Contains(string(out), "epic =") || strings.Contains(string(out), "color =") {
		t.Fatalf("unset fields were written back:\n%s", out)
	}
}

func TestSave_MultipleTerminalColumns(t *testing.T) {
	path := writeTempBoard(t, `schema_version = 2

[board]
columns = ['todo', 'shipped*', 'wont-fix*']
priorities = ['low']
`)
	b, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !b.Board.IsDoneColumn("shipped") || !b.Board.IsDoneColumn("wont-fix") {
		t.Fatalf("terminal columns = %v, want both", b.Board.DoneColumns())
	}
	if err := Save(path, b); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "'shipped*'") || !strings.Contains(string(out), "'wont-fix*'") {
		t.Fatalf("markers lost:\n%s", out)
	}
}

func TestLoadUnchecked_ReadsAV1File(t *testing.T) {
	path := writeTempBoard(t, `schema_version = 1

[board]
columns = ['todo', 'done']
priorities = ['low']
`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted a v1 file")
	}
	b, err := LoadUnchecked(path)
	if err != nil {
		t.Fatalf("LoadUnchecked: %v", err)
	}
	if b.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", b.SchemaVersion)
	}
	if !equalStringSlices(b.Board.Columns, []string{"todo", "done"}) {
		t.Fatalf("Columns = %v", b.Board.Columns)
	}
}
