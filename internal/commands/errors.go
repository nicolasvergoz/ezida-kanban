package commands

import "fmt"

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
func (e *InvalidFilterError) ExitCode() int    { return 1 }
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
