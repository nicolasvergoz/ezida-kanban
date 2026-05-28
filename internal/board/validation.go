package board

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Violation describes a single broken validation rule.
type Violation struct {
	// Rule is the 1-based rule number from the spec (1..9).
	Rule int
	// Message is a human-readable description of the problem.
	Message string
	// CardID names the offending card; empty when the rule is board-level.
	CardID string
}

// ValidationError is returned by Validate when one or more rules are broken.
// It collects every violation found in a single pass.
type ValidationError struct {
	Violations []Violation
}

// Error renders one line per violation, in rule-then-card order.
func (e *ValidationError) Error() string {
	if e == nil || len(e.Violations) == 0 {
		return "board: validation error (no violations)"
	}
	var b strings.Builder
	b.WriteString("board: validation failed:")
	for _, v := range e.Violations {
		b.WriteByte('\n')
		if v.CardID != "" {
			fmt.Fprintf(&b, "  rule %d [card %s]: %s", v.Rule, v.CardID, v.Message)
		} else {
			fmt.Fprintf(&b, "  rule %d: %s", v.Rule, v.Message)
		}
	}
	return b.String()
}

// SchemaVersionError is returned by Load when the on-disk file's
// schema_version differs from SupportedSchemaVersion. It is not wrapped into
// ValidationError because no further validation can meaningfully run.
type SchemaVersionError struct {
	FileVersion      int
	SupportedVersion int
}

func (e *SchemaVersionError) Error() string {
	return fmt.Sprintf(
		"board: schema_version mismatch — file is v%d, binary supports v%d",
		e.FileVersion, e.SupportedVersion,
	)
}

var idValidationPattern = regexp.MustCompile(`^[0-9a-z]{6}$`)

var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// Validate runs the nine business rules in a single pass and returns a
// *ValidationError listing every violation, or nil if the board is valid.
func Validate(b *Board) *ValidationError {
	var vs []Violation

	// Rule 1: schema_version equals the supported version.
	if b.SchemaVersion != SupportedSchemaVersion {
		vs = append(vs, Violation{
			Rule: 1,
			Message: fmt.Sprintf(
				"schema_version is %d, expected %d",
				b.SchemaVersion, SupportedSchemaVersion,
			),
		})
	}

	// Rule 2: columns non-empty and contains no duplicates.
	if len(b.Board.Columns) == 0 {
		vs = append(vs, Violation{Rule: 2, Message: "[board].columns must be non-empty"})
	} else {
		seen := make(map[string]struct{}, len(b.Board.Columns))
		for _, col := range b.Board.Columns {
			if _, dup := seen[col]; dup {
				vs = append(vs, Violation{
					Rule:    2,
					Message: fmt.Sprintf("[board].columns contains duplicate %q", col),
				})
			}
			seen[col] = struct{}{}
		}
	}

	// Rule 3: priorities non-empty and contains no duplicates.
	if len(b.Board.Priorities) == 0 {
		vs = append(vs, Violation{Rule: 3, Message: "[board].priorities must be non-empty"})
	} else {
		seen := make(map[string]struct{}, len(b.Board.Priorities))
		for _, p := range b.Board.Priorities {
			if _, dup := seen[p]; dup {
				vs = append(vs, Violation{
					Rule:    3,
					Message: fmt.Sprintf("[board].priorities contains duplicate %q", p),
				})
			}
			seen[p] = struct{}{}
		}
	}

	// Build lookup sets for cross-field rules.
	colSet := make(map[string]struct{}, len(b.Board.Columns))
	for _, col := range b.Board.Columns {
		colSet[col] = struct{}{}
	}
	priSet := make(map[string]struct{}, len(b.Board.Priorities))
	for _, p := range b.Board.Priorities {
		priSet[p] = struct{}{}
	}

	// Rule 10: every priority_colors key exists in priorities; every value is a valid hex.
	for k, v := range b.Board.PriorityColors {
		if _, ok := priSet[k]; !ok {
			vs = append(vs, Violation{
				Rule:    10,
				Message: fmt.Sprintf("[board.priority_colors] key %q is not declared in [board].priorities", k),
			})
		}
		if !hexColorPattern.MatchString(v) {
			vs = append(vs, Violation{
				Rule:    10,
				Message: fmt.Sprintf("[board.priority_colors] value for %q is %q, expected hex color like #rgb or #rrggbb", k, v),
			})
		}
	}

	// Track first-seen ID for duplicate reporting.
	firstSeen := make(map[string]string, len(b.Cards))

	for _, c := range b.Cards {
		// Rule 4: id matches ^[0-9a-z]{6}$.
		if !idValidationPattern.MatchString(c.ID) {
			vs = append(vs, Violation{
				Rule:    4,
				CardID:  c.ID,
				Message: fmt.Sprintf("id %q does not match ^[0-9a-z]{6}$", c.ID),
			})
		}

		// Rule 5: card IDs are unique across the board.
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

		// Rule 7: column exists in [board].columns.
		if _, ok := colSet[c.Column]; !ok {
			vs = append(vs, Violation{
				Rule:    7,
				CardID:  c.ID,
				Message: fmt.Sprintf("column %q is not declared in [board].columns", c.Column),
			})
		}

		// Rule 8: priority, when present, exists in [board].priorities.
		if c.Priority != "" {
			if _, ok := priSet[c.Priority]; !ok {
				vs = append(vs, Violation{
					Rule:    8,
					CardID:  c.ID,
					Message: fmt.Sprintf("priority %q is not declared in [board].priorities", c.Priority),
				})
			}
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
				Rule:   9,
				CardID: c.ID,
				Message: fmt.Sprintf("updated_at (%s) must be >= created_at (%s)",
					c.UpdatedAt.Format(time.RFC3339), c.CreatedAt.Format(time.RFC3339)),
			})
		}
	}

	if len(vs) == 0 {
		return nil
	}
	return &ValidationError{Violations: vs}
}
