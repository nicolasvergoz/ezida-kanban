package board

import (
	"fmt"
	"strings"
)

// ValidateArchive runs the archive's own rule set and returns a
// *ValidationError listing every violation, or nil if the archive is
// valid. Rule numbers continue the board's numbering and keep their
// board meanings.
//
// Kept from board validation: 1 (schema version), 4 (id format), 5
// (ids unique within the archive), 6 (title non-empty), 9
// (timestamps), 12 (epic != self), 14 (color hex).
//
// Deliberately NOT applied: 7 (column membership), 8 (priority
// membership), 11 (epic exists), 13 (nesting), 15 (schema gate on
// epic/color), and every [board]-table rule (2, 3, 10, 16, 17) — the
// archive has no [board] table, and an archived card's column/epic may
// reference something the live board no longer declares.
//
// Added: 18 (archived_at non-zero, not before created_at), 19 (column
// non-empty).
func ValidateArchive(a *Archive) *ValidationError {
	var vs []Violation

	// Rule 1: schema_version equals the supported version.
	if a.SchemaVersion != SupportedSchemaVersion {
		vs = append(vs, Violation{
			Rule: 1,
			Message: fmt.Sprintf(
				"schema_version is %d, expected %d",
				a.SchemaVersion, SupportedSchemaVersion,
			),
		})
	}

	firstSeen := make(map[string]string, len(a.Cards))

	for _, c := range a.Cards {
		// Rule 4: id matches ^[0-9a-z]{6}$.
		if !idValidationPattern.MatchString(c.ID) {
			vs = append(vs, Violation{
				Rule:    4,
				CardID:  c.ID,
				Message: fmt.Sprintf("id %q does not match ^[0-9a-z]{6}$", c.ID),
			})
		}

		// Rule 5: card ids are unique within the archive.
		if prev, dup := firstSeen[c.ID]; dup {
			vs = append(vs, Violation{
				Rule:    5,
				CardID:  c.ID,
				Message: fmt.Sprintf("duplicate card id %q (first seen on card %q)", c.ID, prev),
			})
		} else {
			firstSeen[c.ID] = c.ID
		}

		// Rule 6: title is non-empty.
		if strings.TrimSpace(c.Title) == "" {
			vs = append(vs, Violation{
				Rule:    6,
				CardID:  c.ID,
				Message: "title must be non-empty",
			})
		}

		// Rule 19: column is non-empty — unarchive always has somewhere
		// to aim at (falling back to the board's first column when the
		// stored one no longer exists).
		if strings.TrimSpace(c.Column) == "" {
			vs = append(vs, Violation{
				Rule:    19,
				CardID:  c.ID,
				Message: "column must be non-empty",
			})
		}

		// Rule 12: epic is not the card's own id. Rule 11 (epic exists)
		// and rule 13 (nesting) are deliberately not checked — an
		// archived child may outlive its parent.
		if c.Epic != "" && c.Epic == c.ID {
			vs = append(vs, Violation{
				Rule:    12,
				CardID:  c.ID,
				Message: "epic must not reference the card itself",
			})
		}

		// Rule 14: color, when present, is a hex value.
		if c.Color != "" && !hexColorPattern.MatchString(c.Color) {
			vs = append(vs, Violation{
				Rule:   14,
				CardID: c.ID,
				Message: fmt.Sprintf(
					"color %q is not a hex color like #rgb or #rrggbb", c.Color),
			})
		}

		// Rule 9: created_at and updated_at are non-zero and updated_at >= created_at.
		if c.CreatedAt.IsZero() {
			vs = append(vs, Violation{
				Rule:    9,
				CardID:  c.ID,
				Message: "created_at must be a non-zero timestamp",
			})
		}
		if c.UpdatedAt.IsZero() {
			vs = append(vs, Violation{
				Rule:    9,
				CardID:  c.ID,
				Message: "updated_at must be a non-zero timestamp",
			})
		}
		if !c.CreatedAt.IsZero() && !c.UpdatedAt.IsZero() && c.UpdatedAt.Before(c.CreatedAt) {
			vs = append(vs, Violation{
				Rule:    9,
				CardID:  c.ID,
				Message: fmt.Sprintf("updated_at (%s) must be >= created_at (%s)", c.UpdatedAt, c.CreatedAt),
			})
		}

		// Rule 18: archived_at is non-zero and not before created_at.
		if c.ArchivedAt.IsZero() {
			vs = append(vs, Violation{
				Rule:    18,
				CardID:  c.ID,
				Message: "archived_at must be a non-zero timestamp",
			})
		} else if !c.CreatedAt.IsZero() && c.ArchivedAt.Before(c.CreatedAt) {
			vs = append(vs, Violation{
				Rule:    18,
				CardID:  c.ID,
				Message: fmt.Sprintf("archived_at (%s) must not precede created_at (%s)", c.ArchivedAt, c.CreatedAt),
			})
		}
	}

	if len(vs) == 0 {
		return nil
	}
	return &ValidationError{Violations: vs}
}
