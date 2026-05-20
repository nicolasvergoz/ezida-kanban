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
func (e *AlreadyInitializedError) ExitCode() int    { return 1 }
func (e *AlreadyInitializedError) Details() map[string]any {
	return map[string]any{"path": e.Path}
}
