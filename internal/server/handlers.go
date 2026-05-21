package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
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
	mux.HandleFunc("POST /api/cards/{id}/move", s.handleMove)
	mux.HandleFunc("PATCH /api/cards/{id}", s.handlePatch)
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
// INVALID_TAG → 400; CARD_NOT_FOUND → 404; INVALID_BODY → 400;
// load/save failures stay 500).
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

	if err := board.UpdateCard(b, id, patch); err != nil {
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
	SchemaVersion  int            `json:"schema_version"`
	Columns        []string       `json:"columns"`
	Priorities     []string       `json:"priorities"`
	CardsPerColumn map[string]int `json:"cards_per_column"`
	Cards          []cardResponse `json:"cards"`
	// ProjectName is the server-resolved name of the project (parent-
	// directory of the board path, with "Ezida" fallback). Computed
	// once at server start; immutable for the process lifetime.
	ProjectName string `json:"project_name"`
}

// cardResponse is the per-card JSON shape returned inside
// boardResponse. Snake_case keys match ADR 0002 §D7; the
// description field is always present (empty string when unset)
// because the UI's edit modal renders it without a second fetch.
type cardResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Column      string    `json:"column"`
	Priority    string    `json:"priority,omitempty"`
	Tags        []string  `json:"tags"`
	Description string    `json:"description"`
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
	resp := boardResponse{
		SchemaVersion:  b.SchemaVersion,
		Columns:        b.Board.Columns,
		Priorities:     b.Board.Priorities,
		CardsPerColumn: counts,
		Cards:          cards,
		ProjectName:    s.projectName,
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
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Initial directive: 2 s reconnect delay (ADR 0002 §D9).
	_, _ = fmt.Fprintf(w, "retry: 2000\n\n")
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
			_, _ = fmt.Fprintf(w, "event: board-changed\ndata: \n\n")
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
