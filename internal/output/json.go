package output

import (
	"bytes"
	"encoding/json"
	"time"
)

// BoardEnvelope is the JSON shape for `ezida board --json` (ADR §D7).
type BoardEnvelope struct {
	SchemaVersion  int            `json:"schema_version"`
	Columns        []string       `json:"columns"`
	Priorities     []string       `json:"priorities"`
	CardsPerColumn map[string]int `json:"cards_per_column"`
}

// ListCard is the per-card shape inside `ezida list --json`. The
// `description` field is intentionally absent (ADR §D7, spec
// "Description omitted in list JSON").
type ListCard struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Column    string    `json:"column"`
	Priority  string    `json:"priority,omitempty"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListEnvelope is the JSON shape for `ezida list --json`.
type ListEnvelope struct {
	Cards []ListCard `json:"cards"`
}

// GetCard is the per-card shape inside `ezida get --json`, including
// the full description.
type GetCard struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Column      string    `json:"column"`
	Priority    string    `json:"priority,omitempty"`
	Tags        []string  `json:"tags"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetEnvelope is the JSON shape for `ezida get --json`.
type GetEnvelope struct {
	Card GetCard `json:"card"`
}

// ErrorEnvelope is the JSON shape for any command's error output
// (ADR §D8).
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the inner object of ErrorEnvelope.
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// marshalLine marshals v to compact JSON and appends a trailing
// newline. Returns the bytes ready to be written to stdout.
func marshalLine(v any) ([]byte, error) {
	buf, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(buf)+1)
	out = append(out, buf...)
	out = append(out, '\n')
	return out, nil
}

// Board marshals a BoardEnvelope and appends a newline.
func Board(env BoardEnvelope) ([]byte, error) { return marshalLine(env) }

// List marshals a ListEnvelope and appends a newline.
func List(env ListEnvelope) ([]byte, error) { return marshalLine(env) }

// Get marshals a GetEnvelope and appends a newline.
func Get(env GetEnvelope) ([]byte, error) { return marshalLine(env) }

// Error marshals an ErrorEnvelope and appends a newline.
func Error(env ErrorEnvelope) ([]byte, error) { return marshalLine(env) }

// Compact reports whether buf is a single JSON value followed by a
// single trailing newline. Used by tests to validate envelope shape
// without depending on encoder internals.
func Compact(buf []byte) bool {
	if len(buf) == 0 || buf[len(buf)-1] != '\n' {
		return false
	}
	trimmed := bytes.TrimRight(buf, "\n")
	return json.Valid(trimmed)
}
