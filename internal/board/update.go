package board

import (
	"fmt"
	"strings"
	"time"
)

// CardPatch carries the optional fields the HTTP PATCH endpoint (and
// any other partial-update caller) may supply for UpdateCard. Pointer
// fields distinguish "absent from the patch" (nil) from "explicitly
// set, possibly to the zero value" (non-nil). See ADR 0002 §D8.
type CardPatch struct {
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	Priority    *string   `json:"priority,omitempty"`
}

// MissingTitleError is returned by UpdateCard when a patch attempts to
// set Title to an empty (whitespace-only) value. The board package
// owns its own copy of this typed error for the same reason as
// CardNotFoundError: internal/board cannot import internal/commands
// without inverting the dependency direction. The HTTP layer maps
// this to the wire code MISSING_TITLE (ADR 0001 §D8), the same code
// the CLI uses for `ezida add` / `ezida edit`.
type MissingTitleError struct{}

func (e *MissingTitleError) Error() string {
	return "board: title must be non-empty"
}

// InvalidPriorityError is returned by UpdateCard when a patch sets
// Priority to a non-empty value that is not declared in
// [board].priorities. See the rationale on MissingTitleError for the
// duplication versus internal/commands.
type InvalidPriorityError struct {
	Priority string
}

func (e *InvalidPriorityError) Error() string {
	return fmt.Sprintf("board: priority %q is not declared in [board].priorities", e.Priority)
}

// InvalidTagError is returned by UpdateCard when a patch's Tags slice
// contains an empty (whitespace-only) entry. See the rationale on
// MissingTitleError for the duplication versus internal/commands.
type InvalidTagError struct {
	Tag string
}

func (e *InvalidTagError) Error() string {
	return fmt.Sprintf("board: tag %q is empty or whitespace-only", e.Tag)
}

// UpdateCard applies p to the card identified by id. Each non-nil
// field in p replaces the corresponding card field; nil fields leave
// the card field unchanged (ADR 0002 §D8 — "present key replaces,
// absent key untouched"). On success, UpdateCard refreshes UpdatedAt
// to the current UTC time at second precision and runs Validate to
// catch any board-level invariant violations.
//
// Pre-mutation rule checks (in order):
//   - id must match an existing card → otherwise *CardNotFoundError
//   - if p.Title != nil and trimmed empty → *MissingTitleError
//   - if p.Tags != nil and any element is empty/whitespace → *InvalidTagError
//   - if p.Priority != nil and value is non-empty and not in
//     b.Board.Priorities → *InvalidPriorityError
//
// On any error, the board is left unmutated.
func UpdateCard(b *Board, id string, p CardPatch) error {
	idx := -1
	for i, c := range b.Cards {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return &CardNotFoundError{ID: id}
	}

	// Pre-validate every field before mutating so a partial failure
	// leaves the card untouched.
	if p.Title != nil {
		if strings.TrimSpace(*p.Title) == "" {
			return &MissingTitleError{}
		}
	}
	if p.Tags != nil {
		for _, t := range *p.Tags {
			if strings.TrimSpace(t) == "" {
				return &InvalidTagError{Tag: t}
			}
		}
	}
	if p.Priority != nil {
		if *p.Priority != "" {
			found := false
			for _, allowed := range b.Board.Priorities {
				if allowed == *p.Priority {
					found = true
					break
				}
			}
			if !found {
				return &InvalidPriorityError{Priority: *p.Priority}
			}
		}
	}

	c := b.Cards[idx]
	if p.Title != nil {
		c.Title = *p.Title
	}
	if p.Description != nil {
		c.Description = *p.Description
	}
	if p.Tags != nil {
		c.Tags = *p.Tags
	}
	if p.Priority != nil {
		c.Priority = *p.Priority
	}
	c.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	b.Cards[idx] = c

	if verr := Validate(b); verr != nil {
		return verr
	}
	return nil
}
