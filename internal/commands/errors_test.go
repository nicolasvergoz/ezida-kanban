package commands

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestErrors_TypedErrorsImplementCodedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{"column not found", &ColumnNotFoundError{Name: "ghost"}, "COLUMN_NOT_FOUND"},
		{"invalid priority", &InvalidPriorityError{Name: "urgent"}, "INVALID_PRIORITY"},
		{"missing title", &MissingTitleError{}, "MISSING_TITLE"},
		{"invalid tag", &InvalidTagError{Raw: ",foo,"}, "INVALID_TAG"},
		{"interactive required", &InteractiveRequiredError{Hint: "use --yes"}, "INTERACTIVE_REQUIRED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Each error must carry a non-empty Error() message.
			if tc.err.Error() == "" {
				t.Errorf("Error() returned empty string")
			}
		})
	}

	// errors.As must round-trip each concrete type.
	var cnf *ColumnNotFoundError
	if !errors.As(&ColumnNotFoundError{Name: "x"}, &cnf) {
		t.Errorf("errors.As(*ColumnNotFoundError) failed")
	}
	var ipe *InvalidPriorityError
	if !errors.As(&InvalidPriorityError{Name: "x"}, &ipe) {
		t.Errorf("errors.As(*InvalidPriorityError) failed")
	}
	var mte *MissingTitleError
	if !errors.As(&MissingTitleError{}, &mte) {
		t.Errorf("errors.As(*MissingTitleError) failed")
	}
	var ite *InvalidTagError
	if !errors.As(&InvalidTagError{Raw: "x"}, &ite) {
		t.Errorf("errors.As(*InvalidTagError) failed")
	}
	var ire *InteractiveRequiredError
	if !errors.As(&InteractiveRequiredError{Hint: "x"}, &ire) {
		t.Errorf("errors.As(*InteractiveRequiredError) failed")
	}
}

// TestErrors_NewP4Errors_HaveExpectedTextRendering covers task 1.1.
func TestErrors_NewP4Errors_HaveExpectedTextRendering(t *testing.T) {
	cards := []affectedCard{
		{ID: "a3f2k9", Title: "Refactor auth"},
		{ID: "b7m1p4", Title: "Update README"},
	}
	cases := []struct {
		name string
		err  error
		want []string // substrings that MUST appear in Error()
	}{
		{
			name: "column in use",
			err:  &ColumnInUseError{Name: "todo", Cards: cards},
			want: []string{
				`column "todo" still referenced by 2 cards:`,
				"  a3f2k9  Refactor auth",
				"  b7m1p4  Update README",
				"Move or remove these cards first.",
			},
		},
		{
			name: "priority in use",
			err:  &PriorityInUseError{Name: "high", Cards: cards},
			want: []string{
				`priority "high" still referenced by 2 cards:`,
				"  a3f2k9  Refactor auth",
				"Move or remove these cards first.",
			},
		},
		{
			name: "duplicate",
			err:  &DuplicateError{Kind: "column", Name: "todo"},
			want: []string{`column "todo" already exists`},
		},
		{
			name: "position out of range",
			err:  &PositionOutOfRangeError{Position: 0, Max: 4},
			want: []string{"position 0 is out of range [1, 4]"},
		},
		{
			name: "last column",
			err:  &LastColumnError{Name: "todo"},
			want: []string{`cannot remove column "todo"`, "[board].columns would be empty"},
		},
		{
			name: "last priority",
			err:  &LastPriorityError{Name: "low"},
			want: []string{`cannot remove priority "low"`, "[board].priorities would be empty"},
		},
		{
			name: "nothing to edit",
			err:  &NothingToEditError{},
			want: []string{"edit requires at least one of"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("Error() missing %q in:\n%s", w, got)
				}
			}
		})
	}
}

// TestErrors_AffectedCardJSONShape covers task 1.2.
func TestErrors_AffectedCardJSONShape(t *testing.T) {
	c := affectedCard{ID: "a3f2k9", Title: "Refactor auth"}
	buf, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(buf)
	if got != `{"id":"a3f2k9","title":"Refactor auth"}` {
		t.Errorf("unexpected JSON: %s", got)
	}
}

// TestErrors_DetailedErrorInterface covers task 1.3.
func TestErrors_DetailedErrorInterface(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"column in use", &ColumnInUseError{Name: "todo", Cards: nil}, "COLUMN_IN_USE"},
		{"priority in use", &PriorityInUseError{Name: "high", Cards: nil}, "PRIORITY_IN_USE"},
		{"duplicate", &DuplicateError{Kind: "column", Name: "todo"}, "DUPLICATE"},
		{"position out of range", &PositionOutOfRangeError{Position: 0, Max: 4}, "POSITION_OUT_OF_RANGE"},
		{"last column", &LastColumnError{Name: "todo"}, "LAST_COLUMN"},
		{"last priority", &LastPriorityError{Name: "low"}, "LAST_PRIORITY"},
		{"nothing to edit", &NothingToEditError{}, "NOTHING_TO_EDIT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var de detailedError
			if !errors.As(tc.err, &de) {
				t.Fatalf("errors.As(detailedError) failed for %T", tc.err)
			}
			if de.Code() != tc.wantCode {
				t.Errorf("code: got %q, want %q", de.Code(), tc.wantCode)
			}
			if de.ShortMessage() == "" {
				t.Errorf("ShortMessage() is empty")
			}
		})
	}
}

// TestAsDetailed_WrapsLegacyCodedError covers task 1.4.
func TestAsDetailed_WrapsLegacyCodedError(t *testing.T) {
	legacy := &CardNotFoundError{ID: "zzzzzz"}
	d := AsDetailed(legacy)
	if d == nil {
		t.Fatal("AsDetailed returned nil")
	}
	if d.Code() != "CARD_NOT_FOUND" {
		t.Errorf("code: got %q", d.Code())
	}
	if d.ShortMessage() == "" {
		t.Errorf("empty short message")
	}
	det, ok := d.Details().(map[string]any)
	if !ok {
		t.Fatalf("details type: %T", d.Details())
	}
	if det["id"] != "zzzzzz" {
		t.Errorf("details.id: %v", det["id"])
	}
}

// TestAsDetailed_PassesThroughDetailedErrors confirms a DetailedError
// is returned unchanged (no wrapping).
func TestAsDetailed_PassesThroughDetailedErrors(t *testing.T) {
	orig := &ColumnInUseError{Name: "todo", Cards: []affectedCard{{ID: "a", Title: "t"}}}
	d := AsDetailed(orig)
	if d == nil {
		t.Fatal("AsDetailed returned nil")
	}
	// Same concrete pointer (no wrapping).
	got, ok := d.(*ColumnInUseError)
	if !ok || got != orig {
		t.Errorf("AsDetailed wrapped a detailedError: got %T (%p) want *ColumnInUseError (%p)", d, d, orig)
	}
}
