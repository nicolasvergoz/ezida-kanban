package board

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

var idPattern = regexp.MustCompile(`^[0-9a-z]{6}$`)

// --- ID tests --------------------------------------------------------------

func TestNewIDFormat(t *testing.T) {
	for i := 0; i < 1000; i++ {
		id := NewID()
		if !idPattern.MatchString(id) {
			t.Fatalf("NewID() returned %q which does not match ^[0-9a-z]{6}$", id)
		}
	}
}

func TestNewUniqueIDCollisions(t *testing.T) {
	// Happy path: empty existing set returns a valid ID.
	id, err := NewUniqueID(nil)
	if err != nil {
		t.Fatalf("NewUniqueID(nil) returned err = %v, want nil", err)
	}
	if !idPattern.MatchString(id) {
		t.Fatalf("NewUniqueID(nil) returned %q which does not match the id pattern", id)
	}

	// Happy path: existing set that does not contain the generated id.
	id2, err := NewUniqueID([]string{"aaaaaa", "bbbbbb"})
	if err != nil {
		t.Fatalf("NewUniqueID(non-colliding) returned err = %v, want nil", err)
	}
	if id2 == "aaaaaa" || id2 == "bbbbbb" {
		t.Fatalf("NewUniqueID returned %q which collides with existing set", id2)
	}

	// Exhausted path: inject a generator that always returns a value present
	// in the existing set, forcing all 10 attempts to collide.
	const sticky = "zzzzzz"
	_, err = newUniqueIDWith(func() string { return sticky }, []string{sticky})
	if !errors.Is(err, ErrIDExhausted) {
		t.Fatalf("newUniqueIDWith exhausted path: got err = %v, want ErrIDExhausted", err)
	}
}

// --- Validation tests ------------------------------------------------------

// unmarshalFixture parses a testdata TOML file straight into a *Board without
// running Validate, so the test can inspect validation results in isolation.
func unmarshalFixture(t *testing.T, name string) *Board {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var b Board
	if err := toml.Unmarshal(data, &b); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
	return &b
}

func hasViolationForRule(verr *ValidationError, rule int) bool {
	if verr == nil {
		return false
	}
	for _, v := range verr.Violations {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

func TestValidationErrorImplementsError(t *testing.T) {
	var verr error = &ValidationError{Violations: []Violation{{Rule: 1, Message: "x"}}}
	var target *ValidationError
	if !errors.As(verr, &target) {
		t.Fatalf("errors.As did not find *ValidationError")
	}
	if target.Error() == "" {
		t.Fatalf("ValidationError.Error() returned empty string")
	}

	var sverr error = &SchemaVersionError{FileVersion: 2, SupportedVersion: 1}
	var sverrTarget *SchemaVersionError
	if !errors.As(sverr, &sverrTarget) {
		t.Fatalf("errors.As did not find *SchemaVersionError")
	}
	if sverrTarget.FileVersion != 2 || sverrTarget.SupportedVersion != 1 {
		t.Fatalf("SchemaVersionError did not preserve fields: %+v", sverrTarget)
	}
}

func TestValidate_Valid(t *testing.T) {
	b := unmarshalFixture(t, "valid.toml")
	if err := Validate(b); err != nil {
		t.Fatalf("Validate(valid.toml) returned %v, want nil", err)
	}

	bm := unmarshalFixture(t, "valid_minimal.toml")
	if err := Validate(bm); err != nil {
		t.Fatalf("Validate(valid_minimal.toml) returned %v, want nil", err)
	}
}

func TestValidate_Rule1_SchemaVersion(t *testing.T) {
	b := unmarshalFixture(t, "invalid_rule1_schema_version.toml")
	err := Validate(b)
	if err == nil || !hasViolationForRule(err, 1) {
		t.Fatalf("expected rule 1 violation, got %v", err)
	}
}

func TestValidate_Rule2_Columns(t *testing.T) {
	b := unmarshalFixture(t, "invalid_rule2_columns.toml")
	err := Validate(b)
	if err == nil || !hasViolationForRule(err, 2) {
		t.Fatalf("expected rule 2 violation, got %v", err)
	}
}

func TestValidate_Rule3_Priorities(t *testing.T) {
	b := unmarshalFixture(t, "invalid_rule3_priorities.toml")
	err := Validate(b)
	if err == nil || !hasViolationForRule(err, 3) {
		t.Fatalf("expected rule 3 violation, got %v", err)
	}
}

func TestValidate_Rule4_IDFormat(t *testing.T) {
	b := unmarshalFixture(t, "invalid_rule4_id_format.toml")
	err := Validate(b)
	if err == nil || !hasViolationForRule(err, 4) {
		t.Fatalf("expected rule 4 violation, got %v", err)
	}
}

func TestValidate_Rule5_DuplicateIDs(t *testing.T) {
	b := unmarshalFixture(t, "invalid_rule5_duplicate_ids.toml")
	err := Validate(b)
	if err == nil || !hasViolationForRule(err, 5) {
		t.Fatalf("expected rule 5 violation, got %v", err)
	}
}

func TestValidate_Rule6_EmptyTitle(t *testing.T) {
	b := unmarshalFixture(t, "invalid_rule6_empty_title.toml")
	err := Validate(b)
	if err == nil || !hasViolationForRule(err, 6) {
		t.Fatalf("expected rule 6 violation, got %v", err)
	}
}

func TestValidate_Rule7_UnknownColumn(t *testing.T) {
	b := unmarshalFixture(t, "invalid_rule7_unknown_column.toml")
	err := Validate(b)
	if err == nil || !hasViolationForRule(err, 7) {
		t.Fatalf("expected rule 7 violation, got %v", err)
	}
}

func TestValidate_Rule8_UnknownPriority(t *testing.T) {
	b := unmarshalFixture(t, "invalid_rule8_unknown_priority.toml")
	err := Validate(b)
	if err == nil || !hasViolationForRule(err, 8) {
		t.Fatalf("expected rule 8 violation, got %v", err)
	}
}

func TestValidate_Rule9_Timestamps(t *testing.T) {
	b := unmarshalFixture(t, "invalid_rule9_timestamps.toml")
	err := Validate(b)
	if err == nil || !hasViolationForRule(err, 9) {
		t.Fatalf("expected rule 9 violation, got %v", err)
	}
}

// --- Load / Save tests -----------------------------------------------------

func TestLoadSave_RoundTrip(t *testing.T) {
	b, err := Load(filepath.Join("testdata", "valid.toml"))
	if err != nil {
		t.Fatalf("Load(valid.toml) returned err = %v", err)
	}
	if b.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", b.SchemaVersion)
	}
	if len(b.Cards) != 3 {
		t.Fatalf("got %d cards, want 3", len(b.Cards))
	}

	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "kanban.toml")
	if err := Save(out, b); err != nil {
		t.Fatalf("Save returned err = %v", err)
	}

	b2, err := Load(out)
	if err != nil {
		t.Fatalf("Load(saved file) returned err = %v", err)
	}

	if b2.SchemaVersion != b.SchemaVersion {
		t.Fatalf("schema_version round-trip: got %d, want %d", b2.SchemaVersion, b.SchemaVersion)
	}
	if !reflectStringSliceEqual(b2.Board.Columns, b.Board.Columns) {
		t.Fatalf("columns round-trip: got %v, want %v", b2.Board.Columns, b.Board.Columns)
	}
	if !reflectStringSliceEqual(b2.Board.Priorities, b.Board.Priorities) {
		t.Fatalf("priorities round-trip: got %v, want %v", b2.Board.Priorities, b.Board.Priorities)
	}
	if len(b2.Cards) != len(b.Cards) {
		t.Fatalf("card count round-trip: got %d, want %d", len(b2.Cards), len(b.Cards))
	}
	for i := range b.Cards {
		if b2.Cards[i].ID != b.Cards[i].ID {
			t.Fatalf("card[%d].ID: got %q, want %q", i, b2.Cards[i].ID, b.Cards[i].ID)
		}
		if b2.Cards[i].Title != b.Cards[i].Title {
			t.Fatalf("card[%d].Title: got %q, want %q", i, b2.Cards[i].Title, b.Cards[i].Title)
		}
		if b2.Cards[i].Column != b.Cards[i].Column {
			t.Fatalf("card[%d].Column: got %q, want %q", i, b2.Cards[i].Column, b.Cards[i].Column)
		}
		if !b2.Cards[i].CreatedAt.Equal(b.Cards[i].CreatedAt) {
			t.Fatalf("card[%d].CreatedAt: got %v, want %v", i, b2.Cards[i].CreatedAt, b.Cards[i].CreatedAt)
		}
		if !b2.Cards[i].UpdatedAt.Equal(b.Cards[i].UpdatedAt) {
			t.Fatalf("card[%d].UpdatedAt: got %v, want %v", i, b2.Cards[i].UpdatedAt, b.Cards[i].UpdatedAt)
		}
	}
}

func reflectStringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLoad_SchemaVersionMismatch(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "invalid_rule1_schema_version.toml"))
	if err == nil {
		t.Fatal("Load returned nil error, want SchemaVersionError")
	}
	var sverr *SchemaVersionError
	if !errors.As(err, &sverr) {
		t.Fatalf("Load returned %v, want *SchemaVersionError", err)
	}
	if sverr.FileVersion != 2 {
		t.Fatalf("FileVersion = %d, want 2", sverr.FileVersion)
	}
	if sverr.SupportedVersion != SupportedSchemaVersion {
		t.Fatalf("SupportedVersion = %d, want %d", sverr.SupportedVersion, SupportedSchemaVersion)
	}
}

func TestSave_AtomicTempFileCleanup(t *testing.T) {
	b, err := Load(filepath.Join("testdata", "valid.toml"))
	if err != nil {
		t.Fatalf("Load returned err = %v", err)
	}

	tmpDir := t.TempDir()
	out := filepath.Join(tmpDir, "kanban.toml")
	if err := Save(out, b); err != nil {
		t.Fatalf("Save returned err = %v", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if name == "kanban.toml" {
			continue
		}
		// Anything else in the dir is a leftover temp file.
		t.Fatalf("unexpected leftover file in tmp dir: %s", name)
	}
}

// --- Card-order spike ------------------------------------------------------

func TestRoundTrip_PreservesCardOrder(t *testing.T) {
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	ids := []string{"aaaaaa", "bbbbbb", "cccccc", "dddddd", "eeeeee"}
	b := &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board: BoardConfig{
			Columns:    []string{"todo"},
			Priorities: []string{"low"},
		},
	}
	for _, id := range ids {
		b.Cards = append(b.Cards, Card{
			ID:        id,
			Title:     "Card " + id,
			Column:    "todo",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "kanban.toml")
	if err := Save(path, b); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := make([]string, len(reloaded.Cards))
	for i, c := range reloaded.Cards {
		got[i] = c.ID
	}
	if !reflectStringSliceEqual(got, ids) {
		t.Fatalf("card order not preserved: got %v, want %v", got, ids)
	}
}

// --- AppendCardToColumn tests ---------------------------------------------

func TestAppendCardToColumn_AfterExistingSameColumn(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	b := &Board{
		Cards: []Card{
			{ID: "aaaaaa", Column: "todo", CreatedAt: now, UpdatedAt: now, Title: "A"},
			{ID: "bbbbbb", Column: "done", CreatedAt: now, UpdatedAt: now, Title: "B"},
			{ID: "cccccc", Column: "todo", CreatedAt: now, UpdatedAt: now, Title: "C"},
		},
	}
	d := Card{ID: "dddddd", Column: "todo", CreatedAt: now, UpdatedAt: now, Title: "D"}
	AppendCardToColumn(b, d)
	want := []string{"aaaaaa", "bbbbbb", "cccccc", "dddddd"}
	got := make([]string, len(b.Cards))
	for i, c := range b.Cards {
		got[i] = c.ID
	}
	if !reflectStringSliceEqual(got, want) {
		t.Fatalf("AppendCardToColumn order: got %v, want %v", got, want)
	}
}

func TestAppendCardToColumn_FirstInColumnAppendsToEnd(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	b := &Board{
		Cards: []Card{
			{ID: "aaaaaa", Column: "todo", CreatedAt: now, UpdatedAt: now, Title: "A"},
		},
	}
	bcard := Card{ID: "bbbbbb", Column: "done", CreatedAt: now, UpdatedAt: now, Title: "B"}
	AppendCardToColumn(b, bcard)
	want := []string{"aaaaaa", "bbbbbb"}
	got := make([]string, len(b.Cards))
	for i, c := range b.Cards {
		got[i] = c.ID
	}
	if !reflectStringSliceEqual(got, want) {
		t.Fatalf("AppendCardToColumn first-in-column: got %v, want %v", got, want)
	}
}

func TestValidate_MultipleViolations(t *testing.T) {
	// Construct a board in-memory that breaks rule 6 (empty title) and
	// rule 7 (unknown column) on the same card.
	b := &Board{
		SchemaVersion: 1,
		Board: BoardConfig{
			Columns:    []string{"todo", "done"},
			Priorities: []string{"low"},
		},
		Cards: []Card{
			{
				ID:        "aaaaaa",
				Title:     "",
				Column:    "ghost",
				CreatedAt: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC),
			},
		},
	}
	verr := Validate(b)
	if verr == nil {
		t.Fatalf("expected a ValidationError, got nil")
	}
	if !hasViolationForRule(verr, 6) || !hasViolationForRule(verr, 7) {
		t.Fatalf("expected rule 6 and 7 violations, got %+v", verr.Violations)
	}
	if len(verr.Violations) < 2 {
		t.Fatalf("expected at least 2 violations in one pass, got %d", len(verr.Violations))
	}
}
