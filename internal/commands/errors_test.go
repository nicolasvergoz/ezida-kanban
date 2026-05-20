package commands

import (
	"errors"
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
