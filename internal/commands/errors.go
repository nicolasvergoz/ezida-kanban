package commands

import (
	"fmt"
	"strings"
)

// affectedCard is the per-card pair carried by refusal-with-detail
// errors (ColumnInUseError, PriorityInUseError). The JSON tags are the
// public contract used by `error.details.cards` in JSON-mode output.
type affectedCard struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// AffectedCard is the exported alias used by callers outside the
// commands package (e.g. output tests). The on-the-wire JSON shape is
// the unexported affectedCard's — they are the same type.
type AffectedCard = affectedCard

// NewColumnInUseError builds a *ColumnInUseError. Exported so callers
// outside the commands package can construct one without touching the
// unexported affectedCard slice element type.
func NewColumnInUseError(name string, cards []AffectedCard) *ColumnInUseError {
	return &ColumnInUseError{Name: name, Cards: cards}
}

// NewPriorityInUseError builds a *PriorityInUseError. See NewColumnInUseError.
func NewPriorityInUseError(name string, cards []AffectedCard) *PriorityInUseError {
	return &PriorityInUseError{Name: name, Cards: cards}
}

// NewDuplicateError builds a *DuplicateError.
func NewDuplicateError(kind, name string) *DuplicateError {
	return &DuplicateError{Kind: kind, Name: name}
}

// NewPositionOutOfRangeError builds a *PositionOutOfRangeError.
func NewPositionOutOfRangeError(position, max int) *PositionOutOfRangeError {
	return &PositionOutOfRangeError{Position: position, Max: max}
}

// NewLastColumnError builds a *LastColumnError.
func NewLastColumnError(name string) *LastColumnError {
	return &LastColumnError{Name: name}
}

// NewLastPriorityError builds a *LastPriorityError.
func NewLastPriorityError(name string) *LastPriorityError {
	return &LastPriorityError{Name: name}
}

// NewNothingToEditError builds a *NothingToEditError.
func NewNothingToEditError() *NothingToEditError {
	return &NothingToEditError{}
}

// detailedError is the unified contract every typed CLI error implements.
// output.Fail relies on this single interface so it does not need to
// branch per error type.
type detailedError interface {
	error
	Code() string
	Details() any
	ShortMessage() string
}

// defaultError adapts existing typed errors that only expose the legacy
// CodedError surface (ErrorCode / Details map / Error()) into the
// detailedError interface used by the P4 dispatcher.
//
// It is used by output.Fail via errors.As on detailedError; the seven
// new P4 errors below implement detailedError directly.
type defaultError struct {
	code    string
	message string
	details any
}

func (e *defaultError) Error() string        { return e.message }
func (e *defaultError) Code() string         { return e.code }
func (e *defaultError) ShortMessage() string { return e.message }
func (e *defaultError) Details() any         { return e.details }

// CardNotFoundError is returned by `get` when no card matches the requested ID.
type CardNotFoundError struct {
	ID string
}

func (e *CardNotFoundError) Error() string {
	return fmt.Sprintf("no card with id %q", e.ID)
}

// ErrorCode satisfies the output.CodedError contract.
func (e *CardNotFoundError) ErrorCode() string { return "CARD_NOT_FOUND" }

// ExitCode satisfies the output.CodedError contract.
func (e *CardNotFoundError) ExitCode() int { return 1 }

// Details satisfies the output.CodedError contract.
func (e *CardNotFoundError) Details() map[string]any {
	return map[string]any{"id": e.ID}
}

// InvalidFilterError is returned by `list` when a --column or --priority
// filter references a value not declared in the board.
type InvalidFilterError struct {
	Flag  string
	Value string
}

func (e *InvalidFilterError) Error() string {
	return fmt.Sprintf("--%s=%q is not declared in the board", e.Flag, e.Value)
}

func (e *InvalidFilterError) ErrorCode() string { return "INVALID_FILTER" }
func (e *InvalidFilterError) ExitCode() int     { return 1 }
func (e *InvalidFilterError) Details() map[string]any {
	return map[string]any{"flag": e.Flag, "value": e.Value}
}

// AlreadyInitializedError is returned by `init` when a kanban.toml
// already exists at the target path and --force was not passed.
type AlreadyInitializedError struct {
	Path string
}

func (e *AlreadyInitializedError) Error() string {
	return fmt.Sprintf("%s already exists; pass --force to overwrite", e.Path)
}

func (e *AlreadyInitializedError) ErrorCode() string { return "ALREADY_INITIALIZED" }
func (e *AlreadyInitializedError) ExitCode() int     { return 1 }
func (e *AlreadyInitializedError) Details() map[string]any {
	return map[string]any{"path": e.Path}
}

// ColumnNotFoundError is returned by mutating commands when a flag or
// argument references a column name that is not declared in the board.
type ColumnNotFoundError struct {
	Name string
}

func (e *ColumnNotFoundError) Error() string {
	return fmt.Sprintf("column %q is not declared in the board", e.Name)
}

func (e *ColumnNotFoundError) ErrorCode() string { return "COLUMN_NOT_FOUND" }
func (e *ColumnNotFoundError) ExitCode() int     { return 1 }
func (e *ColumnNotFoundError) Details() map[string]any {
	return map[string]any{"column": e.Name}
}

// InvalidPriorityError is returned by `add` (and any future mutating
// command that sets a priority) when the supplied value is not declared
// in the board.
type InvalidPriorityError struct {
	Name string
}

func (e *InvalidPriorityError) Error() string {
	return fmt.Sprintf("priority %q is not declared in the board", e.Name)
}

func (e *InvalidPriorityError) ErrorCode() string { return "INVALID_PRIORITY" }
func (e *InvalidPriorityError) ExitCode() int     { return 1 }
func (e *InvalidPriorityError) Details() map[string]any {
	return map[string]any{"priority": e.Name}
}

// MissingTitleError is returned by `add` when the positional title is
// empty (or whitespace-only).
type MissingTitleError struct{}

func (e *MissingTitleError) Error() string {
	return "title must be non-empty"
}

func (e *MissingTitleError) ErrorCode() string       { return "MISSING_TITLE" }
func (e *MissingTitleError) ExitCode() int           { return 1 }
func (e *MissingTitleError) Details() map[string]any { return nil }

// InvalidTagError is returned by `add` when the --tags CSV contains an
// empty entry (e.g. a leading or trailing comma).
type InvalidTagError struct {
	Raw string
}

func (e *InvalidTagError) Error() string {
	return fmt.Sprintf("invalid tag list %q: empty entries are not allowed", e.Raw)
}

func (e *InvalidTagError) ErrorCode() string { return "INVALID_TAG" }
func (e *InvalidTagError) ExitCode() int     { return 1 }
func (e *InvalidTagError) Details() map[string]any {
	return map[string]any{"raw": e.Raw}
}

// InteractiveRequiredError is returned by `rm` when invoked without
// --yes in a context where prompting is not possible (JSON mode or
// non-TTY stdin/stdout).
type InteractiveRequiredError struct {
	Hint string
}

func (e *InteractiveRequiredError) Error() string {
	if e.Hint == "" {
		return "interactive confirmation required; pass --yes"
	}
	return "interactive confirmation required; " + e.Hint
}

func (e *InteractiveRequiredError) ErrorCode() string { return "INTERACTIVE_REQUIRED" }
func (e *InteractiveRequiredError) ExitCode() int     { return 1 }
func (e *InteractiveRequiredError) Details() map[string]any {
	if e.Hint == "" {
		return nil
	}
	return map[string]any{"hint": e.Hint}
}

// ColumnInUseError is returned by `columns rm` when one or more cards
// still reference the target column. The Error() rendering matches the
// text-mode refusal payload (D14 / spec); the Cards slice is the JSON
// payload.
type ColumnInUseError struct {
	Name  string
	Cards []affectedCard
}

func (e *ColumnInUseError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "column %q still referenced by %d cards:\n", e.Name, len(e.Cards))
	for _, c := range e.Cards {
		fmt.Fprintf(&sb, "  %s  %s\n", c.ID, c.Title)
	}
	sb.WriteString("Move or remove these cards first.")
	return sb.String()
}

func (e *ColumnInUseError) Code() string { return "COLUMN_IN_USE" }
func (e *ColumnInUseError) ShortMessage() string {
	return fmt.Sprintf("column %q still referenced by %d cards", e.Name, len(e.Cards))
}
func (e *ColumnInUseError) Details() any {
	return map[string]any{"column": e.Name, "cards": e.Cards}
}

// PriorityInUseError is the priorities counterpart of ColumnInUseError.
type PriorityInUseError struct {
	Name  string
	Cards []affectedCard
}

func (e *PriorityInUseError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "priority %q still referenced by %d cards:\n", e.Name, len(e.Cards))
	for _, c := range e.Cards {
		fmt.Fprintf(&sb, "  %s  %s\n", c.ID, c.Title)
	}
	sb.WriteString("Move or remove these cards first.")
	return sb.String()
}

func (e *PriorityInUseError) Code() string { return "PRIORITY_IN_USE" }
func (e *PriorityInUseError) ShortMessage() string {
	return fmt.Sprintf("priority %q still referenced by %d cards", e.Name, len(e.Cards))
}
func (e *PriorityInUseError) Details() any {
	return map[string]any{"priority": e.Name, "cards": e.Cards}
}

// DuplicateError is returned by `columns add` / `rename` and the
// priority equivalents when the target name already exists in the
// reference list.
type DuplicateError struct {
	Kind string // "column" or "priority"
	Name string
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("%s %q already exists", e.Kind, e.Name)
}

func (e *DuplicateError) Code() string         { return "DUPLICATE" }
func (e *DuplicateError) ShortMessage() string { return e.Error() }
func (e *DuplicateError) Details() any {
	return map[string]any{"kind": e.Kind, "name": e.Name}
}

// PositionOutOfRangeError is returned by `columns add --position=N` when
// N is below 1 or above len(columns)+1.
type PositionOutOfRangeError struct {
	Position int
	Max      int // inclusive upper bound (len(list)+1)
}

func (e *PositionOutOfRangeError) Error() string {
	return fmt.Sprintf("position %d is out of range [1, %d]", e.Position, e.Max)
}

func (e *PositionOutOfRangeError) Code() string         { return "POSITION_OUT_OF_RANGE" }
func (e *PositionOutOfRangeError) ShortMessage() string { return e.Error() }
func (e *PositionOutOfRangeError) Details() any {
	return map[string]any{"position": e.Position, "max": e.Max}
}

// LastColumnError is returned by `columns rm` when removing the target
// would leave [board].columns empty.
type LastColumnError struct {
	Name string
}

func (e *LastColumnError) Error() string {
	return fmt.Sprintf("cannot remove column %q: [board].columns would be empty", e.Name)
}

func (e *LastColumnError) Code() string         { return "LAST_COLUMN" }
func (e *LastColumnError) ShortMessage() string { return e.Error() }
func (e *LastColumnError) Details() any {
	return map[string]any{"column": e.Name}
}

// LastPriorityError is returned by `priorities rm` when removing the
// target would leave [board].priorities empty.
type LastPriorityError struct {
	Name string
}

func (e *LastPriorityError) Error() string {
	return fmt.Sprintf("cannot remove priority %q: [board].priorities would be empty", e.Name)
}

func (e *LastPriorityError) Code() string         { return "LAST_PRIORITY" }
func (e *LastPriorityError) ShortMessage() string { return e.Error() }
func (e *LastPriorityError) Details() any {
	return map[string]any{"priority": e.Name}
}

// NothingToEditError is returned by `edit` when no field flag was
// passed.
type NothingToEditError struct{}

func (e *NothingToEditError) Error() string {
	return "edit requires at least one of --title, --description, --priority, --tags, --column"
}

func (e *NothingToEditError) Code() string         { return "NOTHING_TO_EDIT" }
func (e *NothingToEditError) ShortMessage() string { return e.Error() }
func (e *NothingToEditError) Details() any         { return nil }

// AsDetailed wraps any error into a detailedError. For errors that
// already implement detailedError it returns them unchanged; for legacy
// CodedError errors it builds a defaultError adapter; for everything
// else it falls back to a generic IO_ERROR.
//
// This is the single seam used by output.Fail so the dispatcher only
// needs one branch path.
func AsDetailed(err error) detailedError {
	if err == nil {
		return nil
	}
	if d, ok := err.(detailedError); ok {
		return d
	}
	// CodedError-shaped legacy errors.
	type coded interface {
		Error() string
		ErrorCode() string
		Details() map[string]any
	}
	if c, ok := err.(coded); ok {
		return &defaultError{
			code:    c.ErrorCode(),
			message: c.Error(),
			details: legacyDetails(c.Details()),
		}
	}
	return &defaultError{code: "IO_ERROR", message: err.Error()}
}

// legacyDetails normalises a legacy map[string]any to the any payload
// the JSON envelope expects. nil maps become nil so omitempty applies.
func legacyDetails(m map[string]any) any {
	if len(m) == 0 {
		return nil
	}
	return m
}
