package board

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// unmarshalArchiveFixture parses a testdata TOML file straight into an
// *Archive without running ValidateArchive, so the test can inspect
// validation results in isolation.
func unmarshalArchiveFixture(t *testing.T, name string) *Archive {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var a Archive
	if err := toml.Unmarshal(data, &a); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", name, err)
	}
	return &a
}

func TestValidateArchive_Valid(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_valid.toml")
	if verr := ValidateArchive(a); verr != nil {
		t.Fatalf("ValidateArchive(valid) = %v, want nil", verr)
	}
}

func TestValidateArchive_AllowsUnknownColumn(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_allows_unknown_column.toml")
	if verr := ValidateArchive(a); verr != nil {
		t.Fatalf("ValidateArchive(unknown column) = %v, want nil — the archive has no [board] table to check against", verr)
	}
}

func TestValidateArchive_AllowsDanglingEpic(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_allows_dangling_epic.toml")
	if verr := ValidateArchive(a); verr != nil {
		t.Fatalf("ValidateArchive(dangling epic) = %v, want nil — a lone archived child keeps an unresolvable epic", verr)
	}
}

func TestValidateArchive_Rule1_SchemaVersion(t *testing.T) {
	a := &Archive{SchemaVersion: 1}
	if !hasViolationForRule(ValidateArchive(a), 1) {
		t.Fatal("expected rule 1 violation for wrong schema_version")
	}
}

func TestValidateArchive_Rule4_IDFormat(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_invalid_rule4_id_format.toml")
	if !hasViolationForRule(ValidateArchive(a), 4) {
		t.Fatal("expected rule 4 violation for malformed id")
	}
}

func TestValidateArchive_Rule5_DuplicateIDs(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_invalid_rule5_duplicate_ids.toml")
	if !hasViolationForRule(ValidateArchive(a), 5) {
		t.Fatal("expected rule 5 violation for duplicate ids")
	}
}

func TestValidateArchive_Rule6_EmptyTitle(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_invalid_rule6_empty_title.toml")
	if !hasViolationForRule(ValidateArchive(a), 6) {
		t.Fatal("expected rule 6 violation for empty title")
	}
}

func TestValidateArchive_Rule9_Timestamps(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_invalid_rule9_timestamps.toml")
	if !hasViolationForRule(ValidateArchive(a), 9) {
		t.Fatal("expected rule 9 violation for updated_at before created_at")
	}
}

func TestValidateArchive_Rule12_SelfEpic(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_invalid_rule12_self_epic.toml")
	if !hasViolationForRule(ValidateArchive(a), 12) {
		t.Fatal("expected rule 12 violation for self-referential epic")
	}
}

func TestValidateArchive_Rule14_Color(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_invalid_rule14_color.toml")
	if !hasViolationForRule(ValidateArchive(a), 14) {
		t.Fatal("expected rule 14 violation for invalid color")
	}
}

func TestValidateArchive_Rule18_MissingArchivedAt(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_invalid_rule18_missing_archived_at.toml")
	if !hasViolationForRule(ValidateArchive(a), 18) {
		t.Fatal("expected rule 18 violation for missing archived_at")
	}
}

func TestValidateArchive_Rule18_ArchivedBeforeCreated(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_valid.toml")
	a.Cards[0].ArchivedAt = a.Cards[0].CreatedAt.AddDate(0, 0, -1)
	if !hasViolationForRule(ValidateArchive(a), 18) {
		t.Fatal("expected rule 18 violation for archived_at preceding created_at")
	}
}

func TestValidateArchive_Rule19_EmptyColumn(t *testing.T) {
	a := unmarshalArchiveFixture(t, "archive_invalid_rule19_empty_column.toml")
	if !hasViolationForRule(ValidateArchive(a), 19) {
		t.Fatal("expected rule 19 violation for empty column")
	}
}

func TestValidateArchive_DoesNotCheckRule7Or8Or11Or13(t *testing.T) {
	// A board-level rule number must never appear for an archive, even
	// on inputs that would trip it on the live board (no [board] table
	// to check membership against).
	a := unmarshalArchiveFixture(t, "archive_allows_unknown_column.toml")
	verr := ValidateArchive(a)
	for _, forbidden := range []int{2, 3, 7, 8, 10, 11, 13, 15, 16, 17} {
		if hasViolationForRule(verr, forbidden) {
			t.Fatalf("ValidateArchive must never report rule %d (board-only rule)", forbidden)
		}
	}
}
