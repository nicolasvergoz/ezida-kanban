package output

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// BoardEnvelope is the JSON shape for `ezida board --json` (ADR §D7).
// Columns and DoneColumns both carry bare names — the on-disk `*`
// suffix never reaches the wire.
type BoardEnvelope struct {
	SchemaVersion  int            `json:"schema_version"`
	Columns        []string       `json:"columns"`
	DoneColumns    []string       `json:"done_columns"`
	Priorities     []string       `json:"priorities"`
	CardsPerColumn map[string]int `json:"cards_per_column"`
}

// ColumnSummary is one entry of `ezida columns --json`.
type ColumnSummary struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Done  bool   `json:"done"`
}

// ColumnsEnvelope is the JSON shape for `ezida columns --json`.
type ColumnsEnvelope struct {
	Columns []ColumnSummary `json:"columns"`
}

// ColorHolder names the epic currently holding a color.
type ColorHolder struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ColorEntry is one entry of `ezida colors --json`. Name is null for a
// color in use that is not part of the palette.
type ColorEntry struct {
	Name   *string      `json:"name"`
	Hex    string       `json:"hex"`
	HeldBy *ColorHolder `json:"held_by"`
}

// ColorsEnvelope is the JSON shape for `ezida colors --json`.
type ColorsEnvelope struct {
	Colors []ColorEntry `json:"colors"`
}

// MigrateEnvelope is the JSON shape for `ezida migrate --json`.
type MigrateEnvelope struct {
	Migrated      bool   `json:"migrated"`
	FromVersion   int    `json:"from_version"`
	ToVersion     int    `json:"to_version"`
	DoneColumn    string `json:"done_column"`
	DoneReason    string `json:"done_column_reason"`
	BackupPath    string `json:"backup_path"`
	SkillReminder string `json:"skill_reminder"`
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
	Epic      string    `json:"epic,omitempty"`
	Color     string    `json:"color,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListEnvelope is the JSON shape for `ezida list --json`.
type ListEnvelope struct {
	Cards []ListCard `json:"cards"`
}

// EpicRef names the parent epic of a child card.
type EpicRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ChildRef is one child of an epic, in board file order.
type ChildRef struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Column string `json:"column"`
}

// Progress is an epic's derived done/total pair. It is computed at read
// time and never persisted (design D7).
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// GetCard is the per-card shape inside `ezida get --json`, including
// the full description.
//
// Epic and Children/Progress are mutually exclusive: one-level nesting
// makes a card that is both a child and a parent unrepresentable, so a
// card carries at most one of the two.
type GetCard struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Column      string     `json:"column"`
	Priority    string     `json:"priority,omitempty"`
	Tags        []string   `json:"tags"`
	Description string     `json:"description"`
	Color       string     `json:"color,omitempty"`
	Epic        *EpicRef   `json:"epic,omitempty"`
	Children    []ChildRef `json:"children,omitempty"`
	Progress    *Progress  `json:"progress,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// GetEnvelope is the JSON shape for `ezida get --json`.
type GetEnvelope struct {
	Card GetCard `json:"card"`
}

// MutateCard is the per-card shape echoed by the mutating commands
// (`add`, `edit`, `move`). It reports `epic` as the raw parent id
// rather than the resolved {id, title} object `get` uses: the echo
// answers "what did you just write", not "what does this card relate
// to".
type MutateCard struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Column      string    `json:"column"`
	Priority    string    `json:"priority,omitempty"`
	Tags        []string  `json:"tags"`
	Description string    `json:"description"`
	Epic        string    `json:"epic,omitempty"`
	Color       string    `json:"color,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MutateEnvelope is the JSON shape echoed by the mutating commands.
type MutateEnvelope struct {
	Card MutateCard `json:"card"`
}

// ExportCard mirrors the viewer's per-card response shape from
// internal/server/handlers.go (cardResponse). Includes description so
// the demo viewer can render the edit modal without a second fetch.
type ExportCard struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Column      string    `json:"column"`
	Priority    string    `json:"priority,omitempty"`
	Tags        []string  `json:"tags"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ExportEnvelope is the JSON shape for `ezida export --json`. Mirrors
// the viewer's boardResponse (GET /api/board) so the demo viewer can
// consume the snapshot through the same code path as the live server.
type ExportEnvelope struct {
	SchemaVersion  int               `json:"schema_version"`
	Columns        []string          `json:"columns"`
	Priorities     []string          `json:"priorities"`
	PriorityColors map[string]string `json:"priority_colors"`
	CardsPerColumn map[string]int    `json:"cards_per_column"`
	Cards          []ExportCard      `json:"cards"`
	ProjectName    string            `json:"project_name"`
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

// Columns marshals a ColumnsEnvelope and appends a newline.
func Columns(env ColumnsEnvelope) ([]byte, error) { return marshalLine(env) }

// Colors marshals a ColorsEnvelope and appends a newline.
func Colors(env ColorsEnvelope) ([]byte, error) { return marshalLine(env) }

// Migrate marshals a MigrateEnvelope and appends a newline.
func Migrate(env MigrateEnvelope) ([]byte, error) { return marshalLine(env) }

// Export marshals an ExportEnvelope and appends a newline.
func Export(env ExportEnvelope) ([]byte, error) { return marshalLine(env) }

// Error marshals an ErrorEnvelope and appends a newline.
func Error(env ErrorEnvelope) ([]byte, error) { return marshalLine(env) }

// JSONCard returns `{"card":{...}}\n` for the given card. Includes
// the `description` field (mutating commands echo the full card, unlike
// `list` which omits it). Returns the marshaled bytes; the trailing
// newline is always present. On marshal failure (which should not
// happen in practice with well-typed inputs) returns nil.
func JSONCard(c board.Card) []byte {
	tags := c.Tags
	if tags == nil {
		tags = []string{}
	}
	env := MutateEnvelope{Card: MutateCard{
		ID:          c.ID,
		Title:       c.Title,
		Column:      c.Column,
		Priority:    c.Priority,
		Tags:        tags,
		Description: c.Description,
		Epic:        c.Epic,
		Color:       c.Color,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}}
	buf, err := marshalLine(env)
	if err != nil {
		return nil
	}
	return buf
}

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
