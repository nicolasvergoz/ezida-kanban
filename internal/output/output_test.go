package output_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/commands"
	. "github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// --- exit.go classification (task 2.1 + task 2.4 done condition) ---

func TestClassify_TypedBoardErrors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantCode  string
		wantExit  int
	}{
		{
			name:     "schema version mismatch",
			err:      &board.SchemaVersionError{FileVersion: 2, SupportedVersion: 1},
			wantCode: "SCHEMA_VERSION_MISMATCH",
			wantExit: 1,
		},
		{
			name:     "validation error",
			err:      &board.ValidationError{Violations: []board.Violation{{Rule: 1, Message: "x"}}},
			wantCode: "VALIDATION_FAILED",
			wantExit: 1,
		},
		{
			name:     "fs not exist",
			err:      fs.ErrNotExist,
			wantCode: "BOARD_NOT_FOUND",
			wantExit: 1,
		},
		{
			name:     "fs permission",
			err:      fs.ErrPermission,
			wantCode: "IO_ERROR",
			wantExit: 2,
		},
		{
			name:     "default unknown",
			err:      errors.New("boom"),
			wantCode: "IO_ERROR",
			wantExit: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotCode, gotExit := Classify(tc.err)
			if gotCode != tc.wantCode {
				t.Errorf("code: got %q, want %q", gotCode, tc.wantCode)
			}
			if gotExit != tc.wantExit {
				t.Errorf("exit: got %d, want %d", gotExit, tc.wantExit)
			}
		})
	}
}

func TestClassify_TypedCommandErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantExit int
	}{
		{
			name:     "card not found",
			err:      &commands.CardNotFoundError{ID: "zzzzzz"},
			wantCode: "CARD_NOT_FOUND",
			wantExit: 1,
		},
		{
			name:     "invalid filter",
			err:      &commands.InvalidFilterError{Flag: "column", Value: "ghost"},
			wantCode: "INVALID_FILTER",
			wantExit: 1,
		},
		{
			name:     "already initialized",
			err:      &commands.AlreadyInitializedError{Path: "kanban.toml"},
			wantCode: "ALREADY_INITIALIZED",
			wantExit: 1,
		},
		{
			name:     "column not found",
			err:      &commands.ColumnNotFoundError{Name: "ghost"},
			wantCode: "COLUMN_NOT_FOUND",
			wantExit: 1,
		},
		{
			name:     "invalid priority",
			err:      &commands.InvalidPriorityError{Name: "urgent"},
			wantCode: "INVALID_PRIORITY",
			wantExit: 1,
		},
		{
			name:     "missing title",
			err:      &commands.MissingTitleError{},
			wantCode: "MISSING_TITLE",
			wantExit: 1,
		},
		{
			name:     "invalid tag",
			err:      &commands.InvalidTagError{Raw: ",foo,"},
			wantCode: "INVALID_TAG",
			wantExit: 1,
		},
		{
			name:     "interactive required",
			err:      &commands.InteractiveRequiredError{Hint: "use --yes"},
			wantCode: "INTERACTIVE_REQUIRED",
			wantExit: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotCode, gotExit := Classify(tc.err)
			if gotCode != tc.wantCode {
				t.Errorf("code: got %q, want %q", gotCode, tc.wantCode)
			}
			if gotExit != tc.wantExit {
				t.Errorf("exit: got %d, want %d", gotExit, tc.wantExit)
			}
			// errors.As must round-trip the concrete type.
			var ce CodedError
			if !errors.As(tc.err, &ce) {
				t.Errorf("errors.As(CodedError) returned false")
			}
		})
	}
}

// --- text.go (task 2.2) ---

func TestResolveColor(t *testing.T) {
	tests := []struct {
		name        string
		force       bool
		noColorEnv  string
		isTTY       bool
		wantEnabled bool
	}{
		{"no-color flag wins", true, "", true, false},
		{"NO_COLOR env disables in TTY", false, "1", true, false},
		{"non-TTY disables", false, "", false, false},
		{"TTY with no overrides enables", false, "", true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveColor(tc.force, tc.noColorEnv, tc.isTTY)
			if got != tc.wantEnabled {
				t.Errorf("got %v, want %v", got, tc.wantEnabled)
			}
		})
	}
}

func TestTable_AlignsColumns_WithEmptyCells(t *testing.T) {
	rows := [][]string{
		{"a3f2k9", "todo", "high", "Refactor auth", "security"},
		{"b7m1p4", "todo", "", "Update README", ""},
	}
	headers := []string{"ID", "COLUMN", "PRI", "TITLE", "TAGS"}
	got := Table(rows, headers)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d:\n%s", len(lines), got)
	}
	// The column boundary positions are determined by the widest cell in
	// each column. For these rows the widths are:
	//   ID:6  COLUMN:6  PRI:4  TITLE:13  TAGS:8
	// With two-space separators, the start offset of each column is:
	//   col 0: 0
	//   col 1: 6 + 2 = 8
	//   col 2: 8 + 6 + 2 = 16
	//   col 3: 16 + 4 + 2 = 22
	//   col 4: 22 + 13 + 2 = 37
	wantTITLE := 22
	wantTAGS := 37
	// Header line.
	if got := strings.Index(lines[0], "TITLE"); got != wantTITLE {
		t.Errorf("header TITLE: got %d, want %d (line: %q)", got, wantTITLE, lines[0])
	}
	if got := strings.Index(lines[0], "TAGS"); got != wantTAGS {
		t.Errorf("header TAGS: got %d, want %d (line: %q)", got, wantTAGS, lines[0])
	}
	// Populated row.
	if got := strings.Index(lines[1], "Refactor"); got != wantTITLE {
		t.Errorf("row1 TITLE: got %d, want %d (line: %q)", got, wantTITLE, lines[1])
	}
	if got := strings.Index(lines[1], "security"); got != wantTAGS {
		t.Errorf("row1 TAGS: got %d, want %d (line: %q)", got, wantTAGS, lines[1])
	}
	// Row with empty cells must still place "Update README" at column 22.
	if got := strings.Index(lines[2], "Update"); got != wantTITLE {
		t.Errorf("row2 TITLE: got %d, want %d (line: %q)", got, wantTITLE, lines[2])
	}
}

func TestKeyValue_Aligns(t *testing.T) {
	got := KeyValue([]KV{
		{"ID", "a3f2k9"},
		{"Title", "Refactor auth"},
		{"Description", "x"},
	})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d", len(lines))
	}
	// "Description:" is the widest key (11 chars). Every value should
	// start at column 14 (11 + 3 spaces). Check the value start
	// position via the location of "a3f2k9" / "Refactor" / "x".
	want := strings.Index(lines[2], "x")
	for i, ln := range lines {
		// Locate the value: it's the substring after ": ".
		idx := strings.Index(ln, ": ")
		if idx == -1 {
			t.Errorf("line %d missing colon: %q", i, ln)
			continue
		}
		valStart := idx + 2
		// All value-start columns should match.
		_ = valStart
	}
	// Stronger check: "Description:" is followed by 3 spaces of
	// padding. For the other keys their padding makes total prefix
	// width equal.
	if want != strings.Index(lines[2], "x") {
		t.Errorf("unexpected")
	}
	// Compute expected prefix width: longest key + ":" + 3 spaces = 11+1+3 = 15.
	const expectedPrefix = len("Description") + 1 + 3
	for i, ln := range lines {
		// First non-space char after the colon should be at expectedPrefix.
		colon := strings.IndexByte(ln, ':')
		// After colon, count spaces.
		j := colon + 1
		for j < len(ln) && ln[j] == ' ' {
			j++
		}
		if j != expectedPrefix {
			t.Errorf("line %d: value starts at %d, want %d (line: %q)", i, j, expectedPrefix, ln)
		}
	}
}

// --- json.go (task 2.3) ---

func TestBoard_RoundTrip(t *testing.T) {
	env := BoardEnvelope{
		SchemaVersion:  1,
		Columns:        []string{"todo", "done"},
		Priorities:     []string{"low", "high"},
		CardsPerColumn: map[string]int{"todo": 3, "done": 7},
	}
	buf, err := Board(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.HasSuffix(string(buf), "\n") {
		t.Errorf("missing trailing newline: %q", buf)
	}
	if !Compact(buf) {
		t.Errorf("not compact JSON: %q", buf)
	}
	var got map[string]any
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	for _, key := range []string{"schema_version", "columns", "priorities", "cards_per_column"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in %v", key, got)
		}
	}
}

func TestList_OmitsDescription(t *testing.T) {
	env := ListEnvelope{Cards: []ListCard{{ID: "a3f2k9", Title: "x", Column: "todo", Tags: []string{}}}}
	buf, err := List(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(buf, &raw); err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	cards := raw["cards"].([]any)
	for _, c := range cards {
		obj := c.(map[string]any)
		if _, has := obj["description"]; has {
			t.Errorf("list card unexpectedly carries description: %v", obj)
		}
	}
}

func TestJSONCard_IncludesAllExpectedKeys(t *testing.T) {
	now := mustTime("2026-05-20T14:30:00Z")
	c := board.Card{
		ID:          "a3f2k9",
		Title:       "Refactor auth",
		Column:      "todo",
		Description: "JWT migration",
		Tags:        []string{"security", "tech-debt"},
		Priority:    "high",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	buf := JSONCard(c)
	if !strings.HasSuffix(string(buf), "\n") {
		t.Errorf("missing trailing newline: %q", buf)
	}
	var raw struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"id", "title", "column", "description", "tags", "priority", "created_at", "updated_at"} {
		if _, has := raw.Card[k]; !has {
			t.Errorf("missing key %q in %v", k, raw.Card)
		}
	}
	if raw.Card["description"] != "JWT migration" {
		t.Errorf("description: %v", raw.Card["description"])
	}
}

func TestJSONCard_NilTagsBecomesEmptySlice(t *testing.T) {
	now := mustTime("2026-05-20T14:30:00Z")
	c := board.Card{
		ID:        "a3f2k9",
		Title:     "x",
		Column:    "todo",
		Tags:      nil,
		CreatedAt: now,
		UpdatedAt: now,
	}
	buf := JSONCard(c)
	var raw struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tags, ok := raw.Card["tags"].([]any)
	if !ok {
		t.Fatalf("tags not an array: %v", raw.Card["tags"])
	}
	if len(tags) != 0 {
		t.Errorf("want empty slice, got %v", tags)
	}
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestError_EnvelopeShape(t *testing.T) {
	env := ErrorEnvelope{Error: ErrorBody{
		Code:    "CARD_NOT_FOUND",
		Message: "no card with id \"zzzzzz\"",
		Details: map[string]any{"id": "zzzzzz"},
	}}
	buf, err := Error(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(buf, &raw); err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	errObj := raw["error"].(map[string]any)
	if errObj["code"] != "CARD_NOT_FOUND" {
		t.Errorf("code mismatch: %v", errObj["code"])
	}
	if _, ok := errObj["details"].(map[string]any); !ok {
		t.Errorf("missing details object")
	}
}
