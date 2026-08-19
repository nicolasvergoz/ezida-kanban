package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

func seedArchive(t *testing.T, boardPath string, cards ...board.ArchivedCard) {
	t.Helper()
	a := &board.Archive{SchemaVersion: board.SupportedSchemaVersion, Cards: cards}
	if err := board.SaveArchive(board.ArchivePathFor(boardPath), a); err != nil {
		t.Fatalf("seed archive: %v", err)
	}
}

// epicBoardBody is a small board with epic rl4m9x holding two children:
// f20wbo lives in the same column as the parent (todo), wrshlo lives in
// a different column (done) — exercising ArchiveColumn's cascade split
// between "direct" (same column) and "cascaded" (elsewhere).
const epicBoardBody = `schema_version = 2

[board]
columns = ["todo", "done"]
priorities = ["low", "medium", "high"]

[[cards]]
id = "rl4m9x"
title = "Epic parent"
column = "todo"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
color = "#8b5cf6"

[[cards]]
id = "f20wbo"
title = "Child in same column"
column = "todo"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
epic = "rl4m9x"

[[cards]]
id = "wrshlo"
title = "Child in another column"
column = "done"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
epic = "rl4m9x"
`

func archivedCardFixture(id, column string, at time.Time) board.ArchivedCard {
	return board.ArchivedCard{
		Card: board.Card{
			ID: id, Title: "Archived " + id, Column: column,
			CreatedAt: at, UpdatedAt: at,
		},
		ArchivedAt: at,
	}
}

// --- GET /api/board: archived_cards ----------------------------------------

func TestHandle_Board_OmitsArchivedCardsKeyWhenNoArchive(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/board")
	if err != nil {
		t.Fatalf("GET /api/board: %v", err)
	}
	defer res.Body.Close()
	var raw map[string]any
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, has := raw["archived_cards"]; has {
		t.Fatalf("response contains archived_cards key with no archive present: %v", raw["archived_cards"])
	}
}

func TestHandle_Board_IncludesArchivedCards(t *testing.T) {
	path := writableBoard(t)
	at := time.Now().UTC()
	seedArchive(t, path, archivedCardFixture("zzzzzz", "done", at))

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/board")
	if err != nil {
		t.Fatalf("GET /api/board: %v", err)
	}
	defer res.Body.Close()
	var payload struct {
		ArchivedCards []map[string]any `json:"archived_cards"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.ArchivedCards) != 1 {
		t.Fatalf("archived_cards = %v, want 1 entry", payload.ArchivedCards)
	}
	entry := payload.ArchivedCards[0]
	if entry["id"] != "zzzzzz" {
		t.Fatalf("archived card id = %v, want zzzzzz", entry["id"])
	}
	if _, has := entry["archived_at"]; !has {
		t.Fatalf("archived card missing archived_at: %v", entry)
	}
}

func TestHandle_Board_ReconcilesDuplicateAgainstLiveBoard(t *testing.T) {
	path := writableBoard(t)
	// writableBoard's fixture already has a live card "aaaaaa" (see
	// server_test.go's valid_kanban.toml). Seed an archive entry with
	// the SAME id to simulate a crash between the two writes of an
	// archive operation.
	at := time.Now().UTC()
	seedArchive(t, path, archivedCardFixture("aaaaaa", "done", at))

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/board")
	if err != nil {
		t.Fatalf("GET /api/board: %v", err)
	}
	defer res.Body.Close()
	var payload struct {
		Cards         []map[string]any `json:"cards"`
		ArchivedCards []map[string]any `json:"archived_cards"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	foundLive := false
	for _, c := range payload.Cards {
		if c["id"] == "aaaaaa" {
			foundLive = true
		}
	}
	if !foundLive {
		t.Fatal("live card aaaaaa missing from cards")
	}
	for _, c := range payload.ArchivedCards {
		if c["id"] == "aaaaaa" {
			t.Fatal("duplicate id aaaaaa must not appear in archived_cards — the live board wins")
		}
	}
}

// --- POST /api/cards/{id}/archive -------------------------------------------

func TestHandle_CardArchive_Success(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/aaaaaa/archive", "")
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	var body struct {
		Archived string   `json:"archived"`
		Cascaded []string `json:"cascaded"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Archived != "aaaaaa" {
		t.Fatalf("archived = %q, want aaaaaa", body.Archived)
	}
	if len(body.Cascaded) != 0 {
		t.Fatalf("cascaded = %v, want none", body.Cascaded)
	}

	b, err := board.Load(path)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	for _, c := range b.Cards {
		if c.ID == "aaaaaa" {
			t.Fatal("card still on the board after archive")
		}
	}
	archive, _, err := board.LoadArchive(board.ArchivePathFor(path))
	if err != nil {
		t.Fatalf("LoadArchive: %v", err)
	}
	found := false
	for _, c := range archive.Cards {
		if c.ID == "aaaaaa" {
			found = true
		}
	}
	if !found {
		t.Fatal("card not found in archive after archive")
	}
}

func TestHandle_CardArchive_CascadesEpicChildren(t *testing.T) {
	path := writableBoardWithBody(t, epicBoardBody)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/rl4m9x/archive", "")
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	var body struct {
		Cascaded []string `json:"cascaded"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Cascaded) != 2 {
		t.Fatalf("cascaded = %v, want 2 children", body.Cascaded)
	}
}

func TestHandle_CardArchive_404OnUnknownID(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/zzzzzz/archive", "")
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "CARD_NOT_FOUND" {
		t.Fatalf("code = %q, want CARD_NOT_FOUND", body.Error.Code)
	}
}

// --- POST /api/cards/{id}/unarchive -----------------------------------------

func TestHandle_CardUnarchive_RestoresIntoStoredColumn(t *testing.T) {
	path := writableBoard(t)
	at := time.Now().UTC()
	seedArchive(t, path, archivedCardFixture("dddddd", "done", at))

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/dddddd/unarchive", "")
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	var body struct {
		Card struct {
			Column string `json:"column"`
		} `json:"card"`
		Relocated bool `json:"relocated"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Card.Column != "done" {
		t.Fatalf("column = %q, want done", body.Card.Column)
	}
	if body.Relocated {
		t.Fatal("relocated = true, want false")
	}
	if _, err := os.Stat(board.ArchivePathFor(path)); !os.IsNotExist(err) {
		t.Fatal("archive file still present after restoring the only card")
	}
}

func TestHandle_CardUnarchive_400OnUnknownColumn(t *testing.T) {
	path := writableBoard(t)
	at := time.Now().UTC()
	seedArchive(t, path, archivedCardFixture("dddddd", "done", at))

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/dddddd/unarchive", `{"column":"ghost"}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "COLUMN_NOT_FOUND" {
		t.Fatalf("code = %q, want COLUMN_NOT_FOUND", body.Error.Code)
	}
}

func TestHandle_CardUnarchive_409OnIDCollision(t *testing.T) {
	path := writableBoard(t)
	// writableBoard's fixture already carries a live card "aaaaaa".
	at := time.Now().UTC()
	seedArchive(t, path, archivedCardFixture("aaaaaa", "done", at))

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards/aaaaaa/unarchive", "")
	defer res.Body.Close()
	if res.StatusCode != 409 {
		t.Fatalf("status = %d, want 409", res.StatusCode)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "ID_COLLISION" {
		t.Fatalf("code = %q, want ID_COLLISION", body.Error.Code)
	}
}

func TestHandle_CardUnarchive_404OnNotArchived(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	// aaaaaa is live, not archived.
	res := postJSON(t, ts.URL+"/api/cards/aaaaaa/unarchive", "")
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "CARD_NOT_ARCHIVED" {
		t.Fatalf("code = %q, want CARD_NOT_ARCHIVED", body.Error.Code)
	}
}

// --- POST /api/columns/{name}/archive ---------------------------------------

func TestHandle_ColumnArchive_LeavesColumnInPlace(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns/todo/archive", "")
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	var body struct {
		Archived []string `json:"archived"`
		Cascaded []string `json:"cascaded"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Archived) != 2 {
		t.Fatalf("archived = %v, want 2 (aaaaaa, bbbbbb)", body.Archived)
	}

	b, err := board.Load(path)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	found := false
	for _, col := range b.Board.Columns {
		if col == "todo" {
			found = true
		}
	}
	if !found {
		t.Fatal("column 'todo' removed; archive-column must leave it in place")
	}
}

func TestHandle_ColumnArchive_CascadeReachesOtherColumns(t *testing.T) {
	path := writableBoardWithBody(t, epicBoardBody)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns/todo/archive", "")
	defer res.Body.Close()
	var body struct {
		Archived []string `json:"archived"`
		Cascaded []string `json:"cascaded"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Cascaded) != 1 || body.Cascaded[0] != "wrshlo" {
		t.Fatalf("cascaded = %v, want [wrshlo]", body.Cascaded)
	}
}

func TestHandle_ColumnArchive_400OnUnknownColumn(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns/ghost/archive", "")
	defer res.Body.Close()
	// Matches the existing COLUMN_NOT_FOUND convention for every other
	// mutation route driven through httpError's generic errors.As arm
	// (handleMove, handleColumnPatch's rename target, ...) — only
	// handleCreate's deliberate departure uses 404 for this code.
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "COLUMN_NOT_FOUND" {
		t.Fatalf("code = %q, want COLUMN_NOT_FOUND", body.Error.Code)
	}
}

func TestHandle_ColumnArchive_EmptyColumnWritesNothing(t *testing.T) {
	path := writableBoard(t)
	preBoard, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pre: %v", err)
	}

	b, err := board.Load(path)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	if err := board.AddColumn(b, "review"); err != nil {
		t.Fatalf("AddColumn: %v", err)
	}
	if err := board.Save(path, b); err != nil {
		t.Fatalf("board.Save: %v", err)
	}
	preBoard, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pre (post add-column): %v", err)
	}

	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns/review/archive", "")
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	var body struct {
		Archived []string `json:"archived"`
		Cascaded []string `json:"cascaded"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Archived) != 0 || len(body.Cascaded) != 0 {
		t.Fatalf("archived/cascaded = %v / %v, want both empty", body.Archived, body.Cascaded)
	}
	postBoard, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read post: %v", err)
	}
	if !bytes.Equal(preBoard, postBoard) {
		t.Fatal("board file modified by an empty-column archive")
	}
	if _, err := os.Stat(board.ArchivePathFor(path)); !os.IsNotExist(err) {
		t.Fatal("archive file created by an empty-column archive")
	}
}

// --- SSE broadcast -----------------------------------------------------------

func TestHandle_ArchiveRoutes_BroadcastBoardChanged(t *testing.T) {
	silenceRunOutput(t)
	withStubRunner(t)

	path := writableBoard(t)
	port := freeLoopbackPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runWithContext(ctx, Options{Port: port, NoOpen: true, Board: path})
	}()
	defer func() {
		cancel()
		<-done
	}()
	waitForListen(t, port, 2*time.Second)

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	req, _ := http.NewRequest("GET", base+"/api/events", nil)
	rctx, rcancel := context.WithCancel(context.Background())
	defer rcancel()
	req = req.WithContext(rctx)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer res.Body.Close()
	br := bufio.NewReader(res.Body)
	_ = readSSEChunk(br, 1*time.Second) // drain retry directive

	time.Sleep(100 * time.Millisecond)

	archiveRes := postJSON(t, base+"/api/cards/aaaaaa/archive", "")
	defer archiveRes.Body.Close()
	if archiveRes.StatusCode != 200 {
		t.Fatalf("archive status = %d", archiveRes.StatusCode)
	}

	chunk := readSSEChunk(br, 2*time.Second)
	if !strings.Contains(chunk, "event: board-changed") {
		t.Fatalf("expected board-changed event, got: %q", chunk)
	}
}
