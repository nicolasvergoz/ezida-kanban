package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// heartbeatInterval is how often the SSE handler emits a `: ping`
// comment line on an idle stream (ADR 0002 §D9). Exposed as a
// package variable so tests can shrink it.
var heartbeatInterval = 30 * time.Second

// InvalidBodyError is returned by handlers that decode a JSON
// request body when the body is missing, not valid JSON, or fails
// type-decoding into the expected struct. The HTTP layer surfaces it
// as 400 INVALID_BODY (ADR 0002 §D7 — error-envelope conventions).
type InvalidBodyError struct {
	Reason string
}

func (e *InvalidBodyError) Error() string {
	if e.Reason == "" {
		return "invalid request body"
	}
	return fmt.Sprintf("invalid request body: %s", e.Reason)
}

// routes registers every HTTP route the v1 viewer surface exposes:
//
//   - GET /              → embedded web/index.html
//   - GET /static/...    → embedded web subtree
//   - GET /api/board     → JSON snapshot of kanban.toml
//
// Any unrecognised path falls through to a JSON 404 envelope so the
// client side sees the same shape it gets from real API errors.
func (s *serverState) routes(mux *http.ServeMux) {
	staticFS, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
	mux.HandleFunc("GET /api/board", s.handleBoard)
	mux.HandleFunc("POST /api/cards", s.handleCreate)
	mux.HandleFunc("POST /api/cards/{id}/move", s.handleMove)
	mux.HandleFunc("POST /api/cards/{id}/archive", s.handleCardArchive)
	mux.HandleFunc("POST /api/cards/{id}/unarchive", s.handleCardUnarchive)
	mux.HandleFunc("PATCH /api/cards/{id}", s.handlePatch)
	mux.HandleFunc("DELETE /api/cards/{id}", s.handleDelete)
	mux.HandleFunc("POST /api/columns", s.handleColumnCreate)
	mux.HandleFunc("POST /api/columns/move", s.handleColumnMove)
	mux.HandleFunc("POST /api/columns/{name}/archive", s.handleColumnArchive)
	mux.HandleFunc("PATCH /api/columns/{name}", s.handleColumnPatch)
	mux.HandleFunc("DELETE /api/columns/{name}", s.handleColumnDelete)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("/", s.handleNotFound)
}

// movePayload is the JSON body shape accepted by POST
// /api/cards/{id}/move. Snake_case per ADR 0002 §D7; Column matches
// the board's [board].columns names verbatim.
type movePayload struct {
	Column   string `json:"column"`
	Position int    `json:"position"`
}

// handleMove implements POST /api/cards/{id}/move per the
// viewer-server delta spec: decode body, load board, call MoveCard,
// persist via Save, encode {card: ...}. Error mapping is handled by
// httpError (CARD_NOT_FOUND → 404; COLUMN_NOT_FOUND → 400;
// INVALID_BODY → 400; load/save failures stay 500).
func (s *serverState) handleMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var p movePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpError(w, &InvalidBodyError{Reason: err.Error()})
		return
	}

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}

	if err := board.MoveCard(b, id, p.Column, p.Position); err != nil {
		httpError(w, err)
		return
	}

	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}

	// Locate the moved card so we can return its post-move state.
	for _, c := range b.Cards {
		if c.ID == id {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"card": cardToResponse(c),
			})
			return
		}
	}
	// Defensive: if MoveCard succeeded but the card vanished from the
	// slice afterwards, something is very wrong. Fall through to a 500.
	httpError(w, fmt.Errorf("card %q missing after move", id))
}

// handlePatch implements PATCH /api/cards/{id} per the viewer-server
// delta spec: decode the JSON body into a board.CardPatch (pointer
// fields distinguish absent vs. set-to-empty per ADR 0002 §D8), load
// the board, apply via board.UpdateCard, persist via board.Save, then
// return {card: ...} with the post-update card. Error mapping is
// handled by httpError (MISSING_TITLE / INVALID_PRIORITY /
// INVALID_TAG / INVALID_EPIC / INVALID_COLOR → 400; CARD_NOT_FOUND →
// 404; INVALID_BODY → 400; load/save failures stay 500).
//
// epic and color are accepted here as they are anywhere else: an epic
// naming an illegal target is INVALID_EPIC, a color that is not a hex
// value is INVALID_COLOR. Palette names are a CLI convenience and are
// refused on the wire, since only hex ever reaches the file.
func (s *serverState) handlePatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var patch board.CardPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		httpError(w, &InvalidBodyError{Reason: err.Error()})
		return
	}

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}

	archive, _, err := board.LoadArchive(board.ArchivePathFor(s.boardPath))
	if err != nil {
		httpError(w, err)
		return
	}

	if err := board.UpdateCard(b, archive, id, patch); err != nil {
		httpError(w, err)
		return
	}

	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}

	for _, c := range b.Cards {
		if c.ID == id {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"card": cardToResponse(c),
			})
			return
		}
	}
	// Defensive: UpdateCard succeeded but the card vanished afterwards.
	httpError(w, fmt.Errorf("card %q missing after patch", id))
}

// createCardPayload is the JSON body shape accepted by POST
// /api/cards. Snake_case per ADR 0002 §D7. Optional fields default
// to their zero values when absent from the JSON, which is exactly
// the behaviour the spec requires (UI-4 design.md §D6).
type createCardPayload struct {
	Column      string   `json:"column"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Tags        []string `json:"tags"`
}

// handleCreate implements POST /api/cards (UI-4 design.md §D7).
// Validation order (each step is its own block for readability):
//
//  1. Decode body → 400 INVALID_BODY.
//  2. Trim-empty title → 400 MISSING_TITLE.
//  3. Each tag must be non-empty after trim → 400 INVALID_TAG.
//  4. column must be declared in [board].columns → 404 COLUMN_NOT_FOUND
//     (deliberate departure from V2's 400 — design.md §D2).
//  5. priority, if non-empty, must be declared in [board].priorities
//     → 400 INVALID_PRIORITY.
//  6. board.NewUniqueID against existing IDs (ErrIDExhausted → 500
//     IO_ERROR via httpError catch-all).
//  7. Build the Card, PrependCardToColumn, Save.
//  8. 201 with {"card": cardToResponse(card)}.
func (s *serverState) handleCreate(w http.ResponseWriter, r *http.Request) {
	var p createCardPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErrorJSON(w, http.StatusBadRequest,
			"INVALID_BODY", err.Error(), nil)
		return
	}

	title := strings.TrimSpace(p.Title)
	if title == "" {
		writeErrorJSON(w, http.StatusBadRequest,
			"MISSING_TITLE", "title must be non-empty", nil)
		return
	}

	for _, tag := range p.Tags {
		if strings.TrimSpace(tag) == "" {
			writeErrorJSON(w, http.StatusBadRequest,
				"INVALID_TAG",
				fmt.Sprintf("tag %q is empty or whitespace-only", tag),
				map[string]any{"tag": tag})
			return
		}
	}

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}

	// Column must exist before any other check that depends on it.
	colKnown := false
	for _, c := range b.Board.Columns {
		if c == p.Column {
			colKnown = true
			break
		}
	}
	if !colKnown {
		// Deliberate 404 departure from V2's 400 (design.md §D2).
		writeErrorJSON(w, http.StatusNotFound,
			"COLUMN_NOT_FOUND",
			fmt.Sprintf("column %q is not declared in [board].columns", p.Column),
			map[string]any{"column": p.Column})
		return
	}

	if p.Priority != "" {
		priKnown := false
		for _, pr := range b.Board.Priorities {
			if pr == p.Priority {
				priKnown = true
				break
			}
		}
		if !priKnown {
			writeErrorJSON(w, http.StatusBadRequest,
				"INVALID_PRIORITY",
				fmt.Sprintf("priority %q is not declared in [board].priorities", p.Priority),
				map[string]any{"priority": p.Priority})
			return
		}
	}

	archive, _, err := board.LoadArchive(board.ArchivePathFor(s.boardPath))
	if err != nil {
		httpError(w, err)
		return
	}
	id, err := board.NewUniqueID(board.ExistingIDs(b, archive))
	if err != nil {
		httpError(w, err)
		return
	}

	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	now := time.Now().UTC().Truncate(time.Second)
	card := board.Card{
		ID:          id,
		Title:       title,
		Column:      p.Column,
		Description: p.Description,
		Priority:    p.Priority,
		Tags:        tags,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	board.PrependCardToColumn(b, card)
	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"card": cardToResponse(card),
	})
}

// handleDelete implements DELETE /api/cards/{id} (UI-4 design.md §D7).
// Loads the board, removes the card via board.DeleteCard (which
// returns *CardNotFoundError if no card matches), persists via
// board.Save, then responds with `{"deleted":"<id>"}`.
func (s *serverState) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}

	if err := board.DeleteCard(b, id); err != nil {
		httpError(w, err)
		return
	}

	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"deleted": id,
	})
}

// --- Archive endpoints -------------------------------------------------------

// handleCardArchive implements POST /api/cards/{id}/archive: archives
// the named card, cascading to its epic children exactly as
// board.ArchiveCard does. The archive file is saved before the board
// (destination gains the cards before the source loses them), the same
// order the CLI's mutateArchiveAndSave uses, so a crash between the
// two writes can only duplicate a card, never lose one.
func (s *serverState) handleCardArchive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}
	archivePath := board.ArchivePathFor(s.boardPath)
	archive, _, err := board.LoadArchive(archivePath)
	if err != nil {
		httpError(w, err)
		return
	}

	archived, err := board.ArchiveCard(b, archive, id, time.Now().UTC().Truncate(time.Second))
	if err != nil {
		httpError(w, err)
		return
	}

	if err := board.SaveArchive(archivePath, archive); err != nil {
		httpError(w, err)
		return
	}
	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}

	cascaded := make([]string, 0, len(archived))
	for _, c := range archived {
		if c.ID != id {
			cascaded = append(cascaded, c.ID)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"archived": id,
		"cascaded": cascaded,
	})
}

// unarchivePayload is the JSON body shape accepted by
// POST /api/cards/{id}/unarchive. Absent or empty means "use the
// stored column".
type unarchivePayload struct {
	Column string `json:"column"`
}

// handleCardUnarchive implements POST /api/cards/{id}/unarchive:
// restores the named archived card (and its archived children) exactly
// as board.UnarchiveCard does. The board is saved before the archive
// (destination gains before source loses), the inverse of the archive
// route's order.
func (s *serverState) handleCardUnarchive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var p unarchivePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		httpError(w, &InvalidBodyError{Reason: err.Error()})
		return
	}

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}
	archivePath := board.ArchivePathFor(s.boardPath)
	archive, _, err := board.LoadArchive(archivePath)
	if err != nil {
		httpError(w, err)
		return
	}

	restored, orphaned, relocated, err := board.UnarchiveCard(b, archive, id, p.Column)
	if err != nil {
		httpError(w, err)
		return
	}

	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}
	if err := board.SaveArchive(archivePath, archive); err != nil {
		httpError(w, err)
		return
	}

	var primary board.Card
	cascaded := make([]string, 0, len(restored))
	for _, c := range restored {
		if c.ID == id {
			primary = c
			continue
		}
		cascaded = append(cascaded, c.ID)
	}
	if orphaned == nil {
		orphaned = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"card":      cardToResponse(primary),
		"cascaded":  cascaded,
		"orphaned":  orphaned,
		"relocated": relocated,
	})
}

// --- Column endpoints (UI-6) ------------------------------------------------

// columnCreatePayload is the JSON body shape accepted by
// POST /api/columns. Snake_case per ADR 0002 §D7.
type columnCreatePayload struct {
	Name string `json:"name"`
}

// columnPatchPayload is the JSON body shape accepted by
// PATCH /api/columns/{name}. Both keys are optional; a pointer is what
// separates "absent, leave it alone" from "present and blank/false".
// `{"name":""}` therefore stays a rejected rename and `{"done":false}`
// still clears the terminal marker (add-terminal-column-ui design D1).
type columnPatchPayload struct {
	Name *string `json:"name"`
	Done *bool   `json:"done"`
}

// columnMovePayload is the JSON body shape accepted by
// POST /api/columns/move.
type columnMovePayload struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// handleColumnCreate implements POST /api/columns (UI-6 design.md).
// Decode → trim → load → AddColumn → save → 201 with the post-add
// column list.
func (s *serverState) handleColumnCreate(w http.ResponseWriter, r *http.Request) {
	var p columnCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpError(w, &InvalidBodyError{Reason: err.Error()})
		return
	}
	trimmed := strings.TrimSpace(p.Name)
	if trimmed == "" {
		httpError(w, &board.EmptyColumnNameError{})
		return
	}

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}
	if err := board.AddColumn(b, trimmed); err != nil {
		httpError(w, err)
		return
	}
	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"columns": b.Board.Columns,
	})
}

// handleColumnPatch implements PATCH /api/columns/{name} (UI-6,
// extended by add-terminal-column-ui). Decode → refuse an empty patch →
// load → verify the column exists → rename → apply the terminal marker
// → save → 200 with the post-patch column list, plus the
// `renamed: {from, to}` echo when a rename actually happened.
//
// The terminal marker is not echoed: the sibling column endpoints do
// not echo it either, and the page learns the new state from the
// board-changed refetch.
func (s *serverState) handleColumnPatch(w http.ResponseWriter, r *http.Request) {
	from := r.PathValue("name")

	var p columnPatchPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpError(w, &InvalidBodyError{Reason: err.Error()})
		return
	}
	if p.Name == nil && p.Done == nil {
		httpError(w, &InvalidBodyError{Reason: "body carries neither name nor done"})
		return
	}

	// Empty/whitespace-only post-trim is rejected as INVALID_BODY via
	// EmptyColumnNameError. We trim here rather than leaving it to
	// board.RenameColumn so a same-name no-op (without surrounding
	// whitespace) succeeds before the helper trims.
	trimmed := ""
	if p.Name != nil {
		trimmed = strings.TrimSpace(*p.Name)
		if trimmed == "" {
			httpError(w, &board.EmptyColumnNameError{})
			return
		}
	}

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}

	// Existence is checked here rather than left to RenameColumn.
	// SetDoneColumn does not verify membership, so a done-only patch on
	// an unknown column would write a flag that EncodeColumns then
	// drops on save — a 200 that did nothing (design D3). Checking up
	// front also covers the rename path with the same error.
	if !slices.Contains(b.Board.Columns, from) {
		httpError(w, &board.ColumnNotFoundError{Column: from})
		return
	}

	target := from
	renamed := false
	if p.Name != nil {
		if err := board.RenameColumn(b, from, trimmed); err != nil {
			httpError(w, err)
			return
		}
		renamed = trimmed != from
		target = trimmed
	}
	// After the rename, never before, and against the post-rename name:
	// RenameColumn carries the terminal flag across to the new name, so
	// setting it first would let that carry-across overwrite the value
	// the client just asked for (design D2).
	if p.Done != nil {
		b.Board.SetDoneColumn(target, *p.Done)
	}

	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}

	out := map[string]any{"columns": b.Board.Columns}
	if renamed {
		out["renamed"] = map[string]any{"from": from, "to": trimmed}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// handleColumnDelete implements DELETE /api/columns/{name} (UI-6).
// Short-circuits the COLUMN_NOT_FOUND case before calling
// board.DeleteColumn so the 404 status code is emitted inline (design
// TD4); other refusals (CANNOT_DELETE_LAST_COLUMN, COLUMN_HAS_CARDS)
// flow through httpError as 400.
func (s *serverState) handleColumnDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}

	// 404 short-circuit per design TD4: handleColumnDelete owns the
	// "not found" status code for this one route.
	found := false
	for _, col := range b.Board.Columns {
		if col == name {
			found = true
			break
		}
	}
	if !found {
		writeErrorJSON(w, http.StatusNotFound,
			"COLUMN_NOT_FOUND",
			fmt.Sprintf("board: column %q is not declared in [board].columns", name),
			map[string]any{"column": name})
		return
	}

	if err := board.DeleteColumn(b, name); err != nil {
		httpError(w, err)
		return
	}
	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"columns": b.Board.Columns,
	})
}

// handleColumnMove implements POST /api/columns/move (UI-6). Decode →
// load → MoveColumn → save → 200 with the post-move column list.
// Position out-of-range is silently clamped by the board helper.
func (s *serverState) handleColumnMove(w http.ResponseWriter, r *http.Request) {
	var p columnMovePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpError(w, &InvalidBodyError{Reason: err.Error()})
		return
	}

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}
	if err := board.MoveColumn(b, p.Name, p.Position); err != nil {
		httpError(w, err)
		return
	}
	if err := board.Save(s.boardPath, b); err != nil {
		httpError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"columns": b.Board.Columns,
	})
}

// handleColumnArchive implements POST /api/columns/{name}/archive:
// archives every card in the named column, cascading to epic children
// living in other columns, exactly as board.ArchiveColumn does. There
// is no interactive prompt on this route — HTTP has no TTY to prompt
// against — so it always behaves as the CLI's --yes path; the viewer
// asks for confirmation client-side, before the request is even sent.
// The column itself is never removed. Both saves are skipped when
// nothing was archived, matching the CLI's empty-column no-op.
func (s *serverState) handleColumnArchive(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}
	archivePath := board.ArchivePathFor(s.boardPath)
	archive, _, err := board.LoadArchive(archivePath)
	if err != nil {
		httpError(w, err)
		return
	}

	direct, cascaded, err := board.ArchiveColumn(b, archive, name, time.Now().UTC().Truncate(time.Second))
	if err != nil {
		httpError(w, err)
		return
	}

	directIDs := make([]string, 0, len(direct))
	for _, c := range direct {
		directIDs = append(directIDs, c.ID)
	}
	cascadedIDs := make([]string, 0, len(cascaded))
	for _, c := range cascaded {
		cascadedIDs = append(cascadedIDs, c.ID)
	}

	if len(directIDs) > 0 || len(cascadedIDs) > 0 {
		if err := board.SaveArchive(archivePath, archive); err != nil {
			httpError(w, err)
			return
		}
		if err := board.Save(s.boardPath, b); err != nil {
			httpError(w, err)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"archived": directIDs,
		"cascaded": cascadedIDs,
	})
}

// handleIndex serves the embedded index.html with an explicit
// Content-Type. Reading from webFS keeps the byte payload stable
// across runs; tests assert on the exact bytes.
func (s *serverState) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := webFS.ReadFile("web/index.html")
	if err != nil {
		httpError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// boardResponse is the JSON envelope returned by GET /api/board.
// Field order and snake_case names match ADR 0002 §D7 — the viewer
// UI consumes this shape directly.
type boardResponse struct {
	SchemaVersion int      `json:"schema_version"`
	Columns       []string `json:"columns"`
	// DoneColumns lists the terminal columns by bare name. Always
	// non-nil so the JSON renders `[]` rather than `null`; the on-disk
	// `*` marker never reaches the wire.
	DoneColumns    []string          `json:"done_columns"`
	Priorities     []string          `json:"priorities"`
	PriorityColors map[string]string `json:"priority_colors"`
	CardsPerColumn map[string]int    `json:"cards_per_column"`
	Cards          []cardResponse    `json:"cards"`
	// ProjectName is the server-resolved name of the project (parent-
	// directory of the board path, with "Ezida" fallback). Computed
	// once at server start; immutable for the process lifetime.
	ProjectName string `json:"project_name"`
	// Version is the build-time server.Version constant, captured once
	// at boot. Immutable for the process lifetime; it is a build
	// constant, not a board value (design D3).
	Version string `json:"version"`
	// ArchivedCards is deliberately left nil (never make(...)) when
	// the reconciled archive has no cards, so omitempty drops the key
	// entirely and a board that has never archived anything produces
	// a response byte-identical to one from before this field existed.
	ArchivedCards []archivedCardResponse `json:"archived_cards,omitempty"`
}

// archivedCardResponse is the per-card JSON shape inside
// boardResponse.ArchivedCards. cardResponse is embedded anonymously so
// encoding/json flattens it into the same field set cardResponse
// carries, plus archived_at — mirroring how board.ArchivedCard embeds
// board.Card, with no second hand-written field list to keep in sync.
type archivedCardResponse struct {
	cardResponse
	ArchivedAt time.Time `json:"archived_at"`
}

// cardResponse is the per-card JSON shape returned inside
// boardResponse. Snake_case keys match ADR 0002 §D7; the
// description field is always present (empty string when unset)
// because the UI's edit modal renders it without a second fetch.
//
// Epic and Color are the raw stored values, never resolved: the
// envelope already carries every card on the board, so the client
// looks a parent up by id and counts children itself. No epic_title,
// epic_color, children, or progress belongs here (design D1).
type cardResponse struct {
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

// cardToResponse converts a board.Card to its wire shape. nil tags
// become an empty slice so JSON renders `"tags":[]` rather than
// `"tags":null` (keeps client code simpler).
func cardToResponse(c board.Card) cardResponse {
	tags := c.Tags
	if tags == nil {
		tags = []string{}
	}
	return cardResponse{
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
	}
}

// handleBoard loads kanban.toml and returns the full board JSON. The
// cards array carries every field (including description) so the
// edit modal can render without a second fetch (ADR 0002 §D7).
func (s *serverState) handleBoard(w http.ResponseWriter, r *http.Request) {
	b, err := board.Load(s.boardPath)
	if err != nil {
		httpError(w, err)
		return
	}
	archive, _, err := board.LoadArchive(board.ArchivePathFor(s.boardPath))
	if err != nil {
		httpError(w, err)
		return
	}
	board.ReconcileArchive(b, archive)

	counts := make(map[string]int, len(b.Board.Columns))
	for _, col := range b.Board.Columns {
		counts[col] = 0
	}
	for _, c := range b.Cards {
		counts[c.Column]++
	}

	cards := make([]cardResponse, 0, len(b.Cards))
	for _, c := range b.Cards {
		cards = append(cards, cardToResponse(c))
	}
	var archivedCards []archivedCardResponse
	for _, c := range archive.Cards {
		archivedCards = append(archivedCards, archivedCardResponse{
			cardResponse: cardToResponse(c.Card),
			ArchivedAt:   c.ArchivedAt,
		})
	}
	resp := boardResponse{
		SchemaVersion:  b.SchemaVersion,
		Columns:        b.Board.Columns,
		DoneColumns:    b.Board.DoneColumns(),
		Priorities:     b.Board.Priorities,
		PriorityColors: board.ResolvePriorityColors(b.Board.Priorities, b.Board.PriorityColors),
		CardsPerColumn: counts,
		Cards:          cards,
		ProjectName:    s.projectName,
		Version:        s.version,
		ArchivedCards:  archivedCards,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleNotFound returns the JSON 404 envelope used for any path
// that does not match a registered route. The wire shape matches
// ADR 0001 §D8 so CLI and HTTP consumers see the same error shape.
func (s *serverState) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeErrorJSON(w, http.StatusNotFound, "NOT_FOUND",
		"no route matches "+r.Method+" "+r.URL.Path, nil)
}

// httpError maps a Go error returned by board.Load (or by the
// embed FS) onto the matching HTTP error envelope. The status code
// is always 500 for board-related failures — these are server-side
// problems even when the underlying cause is a missing or invalid
// file (the user did not "request" a 4xx via the URL).
func httpError(w http.ResponseWriter, err error) {
	var ibe *InvalidBodyError
	if errors.As(err, &ibe) {
		writeErrorJSON(w, http.StatusBadRequest,
			"INVALID_BODY", err.Error(), nil)
		return
	}
	var cnf *board.CardNotFoundError
	if errors.As(err, &cnf) {
		writeErrorJSON(w, http.StatusNotFound,
			"CARD_NOT_FOUND", err.Error(),
			map[string]any{"id": cnf.ID})
		return
	}
	var colnf *board.ColumnNotFoundError
	if errors.As(err, &colnf) {
		writeErrorJSON(w, http.StatusBadRequest,
			"COLUMN_NOT_FOUND", err.Error(),
			map[string]any{"column": colnf.Column})
		return
	}
	var cna *board.CardNotArchivedError
	if errors.As(err, &cna) {
		writeErrorJSON(w, http.StatusNotFound,
			"CARD_NOT_ARCHIVED", err.Error(),
			map[string]any{"id": cna.ID})
		return
	}
	var idc *board.IDCollisionError
	if errors.As(err, &idc) {
		writeErrorJSON(w, http.StatusConflict,
			"ID_COLLISION", err.Error(),
			map[string]any{"id": idc.ID})
		return
	}
	var mte *board.MissingTitleError
	if errors.As(err, &mte) {
		writeErrorJSON(w, http.StatusBadRequest,
			"MISSING_TITLE", err.Error(), nil)
		return
	}
	var ipe *board.InvalidPriorityError
	if errors.As(err, &ipe) {
		writeErrorJSON(w, http.StatusBadRequest,
			"INVALID_PRIORITY", err.Error(),
			map[string]any{"priority": ipe.Priority})
		return
	}
	var ite *board.InvalidTagError
	if errors.As(err, &ite) {
		writeErrorJSON(w, http.StatusBadRequest,
			"INVALID_TAG", err.Error(),
			map[string]any{"tag": ite.Tag})
		return
	}
	// An epic target the board refuses and a malformed color are
	// invalid arguments, not I/O failures: without these two arms
	// they leave through the catch-all as 500 IO_ERROR and no client
	// has a message worth showing.
	var iee *board.InvalidEpicError
	if errors.As(err, &iee) {
		writeErrorJSON(w, http.StatusBadRequest,
			"INVALID_EPIC", err.Error(),
			map[string]any{"epic": iee.ID})
		return
	}
	var ice *board.InvalidColorError
	if errors.As(err, &ice) {
		writeErrorJSON(w, http.StatusBadRequest,
			"INVALID_COLOR", err.Error(),
			map[string]any{"color": ice.Value})
		return
	}
	var ene *board.EmptyColumnNameError
	if errors.As(err, &ene) {
		writeErrorJSON(w, http.StatusBadRequest,
			"INVALID_BODY", "column name must be non-empty", nil)
		return
	}
	var caee *board.ColumnAlreadyExistsError
	if errors.As(err, &caee) {
		writeErrorJSON(w, http.StatusBadRequest,
			"COLUMN_ALREADY_EXISTS", err.Error(),
			map[string]any{"name": caee.Name})
		return
	}
	var cdle *board.CannotDeleteLastColumnError
	if errors.As(err, &cdle) {
		writeErrorJSON(w, http.StatusBadRequest,
			"CANNOT_DELETE_LAST_COLUMN", err.Error(),
			map[string]any{"name": cdle.Name})
		return
	}
	var che *board.ColumnHasCardsError
	if errors.As(err, &che) {
		writeErrorJSON(w, http.StatusBadRequest,
			"COLUMN_HAS_CARDS", err.Error(),
			map[string]any{"column": che.Name, "cards": che.Cards})
		return
	}
	var sv *board.SchemaVersionError
	if errors.As(err, &sv) {
		writeErrorJSON(w, http.StatusInternalServerError,
			"SCHEMA_VERSION_MISMATCH", err.Error(),
			map[string]any{
				"file_version":      sv.FileVersion,
				"supported_version": sv.SupportedVersion,
			})
		return
	}
	var ve *board.ValidationError
	if errors.As(err, &ve) {
		writeErrorJSON(w, http.StatusInternalServerError,
			"VALIDATION_FAILED", err.Error(), nil)
		return
	}
	if errors.Is(err, fs.ErrNotExist) {
		writeErrorJSON(w, http.StatusInternalServerError,
			"BOARD_NOT_FOUND",
			"kanban.toml not found in this directory; run `ezida init` to create one",
			nil)
		return
	}
	writeErrorJSON(w, http.StatusInternalServerError,
		"IO_ERROR", err.Error(), nil)
}

// handleEvents implements `GET /api/events`, the Server-Sent Events
// stream the UI subscribes to for live updates (ADR 0002 §D9). On
// connect the handler writes the `retry: 2000` directive (so browsers
// reconnect with a 2 s delay), then loops on the broker channel and a
// 30 s heartbeat ticker. The loop exits when the client closes the
// connection (r.Context().Done()) or when the broker channel is
// closed during shutdown.
//
// The handler does NOT emit `data:` payloads — the spec emits a
// single event type `board-changed` with an empty data line. Clients
// refetch `/api/board` on receipt.
func (s *serverState) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	// X-Accel-Buffering disables proxy buffering (nginx, etc.).
	// Safari quirk: even on localhost, Safari sometimes waits for
	// ~1 KB or a flush before dispatching the first EventSource
	// event. The initial comment line below + this header keeps the
	// stream interactive.
	w.Header().Set("X-Accel-Buffering", "no")

	// Initial directive: 2 s reconnect delay (ADR 0002 §D9).
	// Padding comment + dummy `hello` event force Safari to dispatch
	// the open event immediately AND warm the EventSource parser so
	// the first real `board-changed` event isn't dropped. Safari
	// requires named events to carry a non-empty `data:` line and
	// often drops the very first named event before any other event
	// has been delivered.
	_, _ = fmt.Fprintf(w, "retry: 2000\n: padding %s\nevent: hello\ndata: 1\n\n", strings.Repeat(" ", 2048))
	flusher.Flush()

	if s.broker == nil {
		// Defensive: a serverState constructed without a broker (e.g.
		// in older tests) should still close cleanly. Block on the
		// request context so the connection stays open until the
		// client disconnects.
		<-r.Context().Done()
		return
	}

	ch, unsubscribe := s.broker.Subscribe()
	defer unsubscribe()

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: board-changed\ndata: 1\n\n")
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// writeErrorJSON renders the canonical error envelope at the given
// status code. The body shape is
//
//	{"error":{"code":"<CODE>","message":"<msg>","details":{...}}}
//
// matching the CLI's JSON-mode error contract (ADR 0001 §D8).
func writeErrorJSON(w http.ResponseWriter, status int, code, message string, details any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if details != nil {
		body["error"].(map[string]any)["details"] = details
	}
	_ = json.NewEncoder(w).Encode(body)
}
