package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writableBoardWithBody writes a custom kanban.toml fixture into a
// temp dir and returns the path. The column tests use this helper for
// scenarios that don't fit the shared valid_kanban.toml fixture
// (e.g. single-column board, board with `review` column for
// delete-success path).
func writableBoardWithBody(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write board: %v", err)
	}
	return path
}

// readColumnsOnDisk parses the board file at path and returns
// b.Board.Columns. Used by the column tests to verify on-disk state
// after a mutating call.
func readColumnsOnDisk(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read board: %v", err)
	}
	var snap struct {
		Board struct {
			Columns []string `toml:"columns"`
		} `toml:"board"`
	}
	if err := tomlUnmarshal(data, &snap); err != nil {
		t.Fatalf("decode board: %v", err)
	}
	return snap.Board.Columns
}

// --- POST /api/columns -----------------------------------------------------

func TestHandleColumnCreate_Success(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns", `{"name":"review"}`)
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var body struct {
		Columns []string `json:"columns"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"todo", "done", "review"}
	if !reflectStringSliceEqual(body.Columns, want) {
		t.Fatalf("columns = %v, want %v", body.Columns, want)
	}
	if got := readColumnsOnDisk(t, path); !reflectStringSliceEqual(got, want) {
		t.Fatalf("on-disk columns = %v, want %v", got, want)
	}
}

func TestHandleColumnCreate_Duplicate(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns", `{"name":"todo"}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"COLUMN_ALREADY_EXISTS"`) {
		t.Fatalf("body missing COLUMN_ALREADY_EXISTS: %s", body)
	}
	if !strings.Contains(body, `"name":"todo"`) {
		t.Fatalf("body missing name detail: %s", body)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandleColumnCreate_EmptyName(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	for _, body := range []string{`{"name":""}`, `{"name":"   "}`} {
		before, _ := os.ReadFile(path)
		res := postJSON(t, ts.URL+"/api/columns", body)
		if res.StatusCode != 400 {
			t.Fatalf("body %q: status = %d, want 400", body, res.StatusCode)
		}
		got := readString(res.Body)
		res.Body.Close()
		if !strings.Contains(got, `"INVALID_BODY"`) {
			t.Fatalf("body %q: missing INVALID_BODY: %s", body, got)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatalf("body %q: on-disk file changed despite 400 error", body)
		}
	}
}

func TestHandleColumnCreate_MalformedJSON(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns", `{not valid json`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"INVALID_BODY"`) {
		t.Fatalf("body missing INVALID_BODY")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

// --- PATCH /api/columns/{name} ---------------------------------------------

func TestHandleColumnRename_Success(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	// Seed has 2 cards in todo. Rename `todo` → `backlog`.
	res := patchJSON(t, ts.URL+"/api/columns/todo", `{"name":"backlog"}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	var body struct {
		Columns []string `json:"columns"`
		Renamed struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"renamed"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"backlog", "done"}
	if !reflectStringSliceEqual(body.Columns, want) {
		t.Fatalf("columns = %v, want %v", body.Columns, want)
	}
	if body.Renamed.From != "todo" || body.Renamed.To != "backlog" {
		t.Fatalf("renamed = %+v, want {from:todo, to:backlog}", body.Renamed)
	}

	// Verify on-disk: cards previously in `todo` are now in `backlog`.
	b, err := readBoardFile(path)
	if err != nil {
		t.Fatalf("readBoardFile: %v", err)
	}
	for _, c := range b.Cards {
		if c.ID == "aaaaaa" || c.ID == "bbbbbb" {
			if c.Column != "backlog" {
				t.Fatalf("card %s on disk has column %q, want backlog", c.ID, c.Column)
			}
		}
	}
}

func TestHandleColumnRename_SameNameNoop(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/columns/todo", `{"name":"todo"}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var body struct {
		Columns []string `json:"columns"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"todo", "done"}
	if !reflectStringSliceEqual(body.Columns, want) {
		t.Fatalf("columns = %v, want %v", body.Columns, want)
	}
}

func TestHandleColumnRename_UnknownFrom(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/columns/ghost", `{"name":"backlog"}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"COLUMN_NOT_FOUND"`) {
		t.Fatalf("body missing COLUMN_NOT_FOUND")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandleColumnRename_DuplicateTo(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/columns/todo", `{"name":"done"}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"COLUMN_ALREADY_EXISTS"`) {
		t.Fatalf("body missing COLUMN_ALREADY_EXISTS: %s", body)
	}
	if !strings.Contains(body, `"name":"done"`) {
		t.Fatalf("body missing name detail: %s", body)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandleColumnRename_EmptyTo(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	for _, body := range []string{`{"name":""}`, `{"name":"   "}`} {
		before, _ := os.ReadFile(path)
		res := patchJSON(t, ts.URL+"/api/columns/todo", body)
		if res.StatusCode != 400 {
			t.Fatalf("body %q: status = %d, want 400", body, res.StatusCode)
		}
		got := readString(res.Body)
		res.Body.Close()
		if !strings.Contains(got, `"INVALID_BODY"`) {
			t.Fatalf("body %q: missing INVALID_BODY: %s", body, got)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatalf("body %q: on-disk file changed despite 400 error", body)
		}
	}
}

// --- DELETE /api/columns/{name} --------------------------------------------

func TestHandleColumnDelete_Success(t *testing.T) {
	// Build a board with a `review` column that has no cards.
	const body = `schema_version = 2

[board]
columns = ["todo", "done", "review"]
priorities = ["low", "medium", "high"]

[[cards]]
id = "aaaaaa"
title = "First"
column = "todo"
description = ""
created_at = 2026-05-01T09:00:00Z
updated_at = 2026-05-01T09:00:00Z
tags = []
`
	path := writableBoardWithBody(t, body)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := deleteJSON(t, ts.URL+"/api/columns/review")
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	var resp struct {
		Columns []string `json:"columns"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"todo", "done"}
	if !reflectStringSliceEqual(resp.Columns, want) {
		t.Fatalf("columns = %v, want %v", resp.Columns, want)
	}
	if got := readColumnsOnDisk(t, path); !reflectStringSliceEqual(got, want) {
		t.Fatalf("on-disk columns = %v, want %v", got, want)
	}
}

func TestHandleColumnDelete_UnknownReturns404(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := deleteJSON(t, ts.URL+"/api/columns/ghost")
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"COLUMN_NOT_FOUND"`) {
		t.Fatalf("body missing COLUMN_NOT_FOUND")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 404 error")
	}
}

func TestHandleColumnDelete_LastColumnRefused(t *testing.T) {
	const body = `schema_version = 2

[board]
columns = ["todo"]
priorities = ["low"]
`
	path := writableBoardWithBody(t, body)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := deleteJSON(t, ts.URL+"/api/columns/todo")
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	got := readString(res.Body)
	if !strings.Contains(got, `"CANNOT_DELETE_LAST_COLUMN"`) {
		t.Fatalf("body missing CANNOT_DELETE_LAST_COLUMN: %s", got)
	}
	if !strings.Contains(got, `"name":"todo"`) {
		t.Fatalf("body missing name detail: %s", got)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandleColumnDelete_ColumnHasCards(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	// Seed has 2 cards (aaaaaa, bbbbbb) in `todo`.
	res := deleteJSON(t, ts.URL+"/api/columns/todo")
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				Column string `json:"column"`
				Cards  []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"cards"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error.Code != "COLUMN_HAS_CARDS" {
		t.Fatalf("error.code = %q, want COLUMN_HAS_CARDS", resp.Error.Code)
	}
	if resp.Error.Details.Column != "todo" {
		t.Fatalf("error.details.column = %q, want todo", resp.Error.Details.Column)
	}
	if len(resp.Error.Details.Cards) != 2 {
		t.Fatalf("error.details.cards has %d entries, want 2", len(resp.Error.Details.Cards))
	}
	wantIDs := map[string]string{"aaaaaa": "First card", "bbbbbb": "Second card"}
	for _, c := range resp.Error.Details.Cards {
		title, ok := wantIDs[c.ID]
		if !ok {
			t.Fatalf("unexpected blocking card id %q", c.ID)
		}
		if c.Title != title {
			t.Fatalf("card %q title = %q, want %q", c.ID, c.Title, title)
		}
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

// --- POST /api/columns/move ------------------------------------------------

func columnMoveBoard(t *testing.T) string {
	t.Helper()
	const body = `schema_version = 2

[board]
columns = ["todo", "ongoing", "done"]
priorities = ["low", "medium", "high"]

[[cards]]
id = "aaaaaa"
title = "First"
column = "todo"
description = ""
created_at = 2026-05-01T09:00:00Z
updated_at = 2026-05-01T09:00:00Z
tags = []
`
	return writableBoardWithBody(t, body)
}

func TestHandleColumnMove_Success(t *testing.T) {
	path := columnMoveBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns/move", `{"name":"done","position":0}`)
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	var resp struct {
		Columns []string `json:"columns"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"done", "todo", "ongoing"}
	if !reflectStringSliceEqual(resp.Columns, want) {
		t.Fatalf("columns = %v, want %v", resp.Columns, want)
	}
	if got := readColumnsOnDisk(t, path); !reflectStringSliceEqual(got, want) {
		t.Fatalf("on-disk columns = %v, want %v", got, want)
	}
	// Verify the [[cards]] entry is byte-identical (no card mutation).
	b, err := readBoardFile(path)
	if err != nil {
		t.Fatalf("readBoardFile: %v", err)
	}
	if len(b.Cards) != 1 || b.Cards[0].ID != "aaaaaa" || b.Cards[0].Column != "todo" {
		t.Fatalf("card state changed: %+v", b.Cards)
	}
}

func TestHandleColumnMove_ClampsPosition(t *testing.T) {
	path := columnMoveBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	// Position 999 → last index.
	res1 := postJSON(t, ts.URL+"/api/columns/move", `{"name":"todo","position":999}`)
	if res1.StatusCode != 200 {
		t.Fatalf("status1 = %d, body = %s", res1.StatusCode, readString(res1.Body))
	}
	res1.Body.Close()
	want1 := []string{"ongoing", "done", "todo"}
	if got := readColumnsOnDisk(t, path); !reflectStringSliceEqual(got, want1) {
		t.Fatalf("after clamp-up: on-disk columns = %v, want %v", got, want1)
	}

	// Position -5 → index 0.
	res2 := postJSON(t, ts.URL+"/api/columns/move", `{"name":"todo","position":-5}`)
	if res2.StatusCode != 200 {
		t.Fatalf("status2 = %d", res2.StatusCode)
	}
	res2.Body.Close()
	want2 := []string{"todo", "ongoing", "done"}
	if got := readColumnsOnDisk(t, path); !reflectStringSliceEqual(got, want2) {
		t.Fatalf("after clamp-down: on-disk columns = %v, want %v", got, want2)
	}
}

func TestHandleColumnMove_UnknownColumn(t *testing.T) {
	path := columnMoveBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns/move", `{"name":"ghost","position":0}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"COLUMN_NOT_FOUND"`) {
		t.Fatalf("body missing COLUMN_NOT_FOUND")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandleColumnMove_MalformedJSON(t *testing.T) {
	path := columnMoveBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/columns/move", `{not valid json`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"INVALID_BODY"`) {
		t.Fatalf("body missing INVALID_BODY")
	}
}

// --- SSE ---------------------------------------------------------------------

func TestHandleColumns_SSEFiresOnSuccess(t *testing.T) {
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

	req, _ := http.NewRequest("GET", "http://127.0.0.1:"+strconv.Itoa(port)+"/api/events", nil)
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

	createRes := postJSON(t, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/columns",
		`{"name":"review"}`)
	defer createRes.Body.Close()
	if createRes.StatusCode != 201 {
		t.Fatalf("create status = %d", createRes.StatusCode)
	}

	chunk := readSSEChunk(br, 2*time.Second)
	if !strings.Contains(chunk, "event: board-changed") {
		t.Fatalf("expected board-changed event, got: %q", chunk)
	}
}

// --- PATCH /api/columns/{name}: the terminal marker -------------------------

// terminalColumnsFixture is a board with one terminal column (`done*`)
// and two ordinary ones, so a single fixture exercises setting,
// clearing and carrying the marker.
const terminalColumnsFixture = `schema_version = 2

[board]
columns = ["todo", "review", "done*"]
priorities = ["low", "medium", "high"]

[[cards]]
id = "aaaaaa"
title = "First"
column = "todo"
description = ""
created_at = 2026-05-01T09:00:00Z
updated_at = 2026-05-01T09:00:00Z
tags = []
`

// TestHandleColumnPatch_TerminalMarker walks the body shapes the
// endpoint gained in add-terminal-column-ui. wantColumns is the RAW
// on-disk list, `*` suffixes included — the marker's placement is the
// whole point, so asserting the decoded names would prove nothing.
func TestHandleColumnPatch_TerminalMarker(t *testing.T) {
	cases := []struct {
		name        string
		column      string
		body        string
		wantColumns []string
		wantRenamed bool
	}{
		{
			name:        "mark terminal without renaming",
			column:      "review",
			body:        `{"done":true}`,
			wantColumns: []string{"todo", "review*", "done*"},
		},
		{
			name:        "clear the marker without renaming",
			column:      "done",
			body:        `{"done":false}`,
			wantColumns: []string{"todo", "review", "done"},
		},
		{
			name:        "setting the value already held is a no-op",
			column:      "done",
			body:        `{"done":true}`,
			wantColumns: []string{"todo", "review", "done*"},
		},
		{
			name:        "clearing a marker that was never set is a no-op",
			column:      "review",
			body:        `{"done":false}`,
			wantColumns: []string{"todo", "review", "done*"},
		},
		{
			name:        "rename and mark terminal in one request",
			column:      "review",
			body:        `{"name":"shipped","done":true}`,
			wantColumns: []string{"todo", "shipped*", "done*"},
			wantRenamed: true,
		},
		{
			// The order-of-operations case: RenameColumn carries the
			// marker across, so a handler that applied `done` first
			// would see it re-applied and answer "still terminal".
			name:        "rename and clear the marker in one request",
			column:      "done",
			body:        `{"name":"shipped","done":false}`,
			wantColumns: []string{"todo", "review", "shipped"},
			wantRenamed: true,
		},
		{
			name:        "a rename alone preserves the marker",
			column:      "done",
			body:        `{"name":"shipped"}`,
			wantColumns: []string{"todo", "review", "shipped*"},
			wantRenamed: true,
		},
		{
			name:        "a rename alone leaves a plain column plain",
			column:      "review",
			body:        `{"name":"shipped"}`,
			wantColumns: []string{"todo", "shipped", "done*"},
			wantRenamed: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writableBoardWithBody(t, terminalColumnsFixture)
			ts, cleanup := startTestServer(t, path)
			defer cleanup()

			res := patchJSON(t, ts.URL+"/api/columns/"+tc.column, tc.body)
			defer res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
			}

			var body map[string]any
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if _, present := body["renamed"]; present != tc.wantRenamed {
				t.Errorf("renamed present = %v, want %v", present, tc.wantRenamed)
			}
			if _, present := body["done_columns"]; present {
				t.Error("response carries done_columns; the marker is not echoed here")
			}
			for _, name := range body["columns"].([]any) {
				if strings.HasSuffix(name.(string), "*") {
					t.Errorf("response column %q carries the on-disk marker", name)
				}
			}

			got := readColumnsOnDisk(t, path)
			if !reflectStringSliceEqual(got, tc.wantColumns) {
				t.Fatalf("on-disk columns = %v, want %v", got, tc.wantColumns)
			}
		})
	}
}

// TestHandleColumnPatch_DoneColumnsAfterToggle closes the loop the
// viewer actually reads: the marker has to come back out of
// GET /api/board, not merely land on disk.
func TestHandleColumnPatch_DoneColumnsAfterToggle(t *testing.T) {
	cases := []struct {
		name     string
		column   string
		body     string
		wantDone []string
	}{
		{"mark terminal", "review", `{"done":true}`, []string{"review", "done"}},
		{"clear the marker", "done", `{"done":false}`, []string{}},
		{"rename carries it", "done", `{"name":"shipped"}`, []string{"shipped"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writableBoardWithBody(t, terminalColumnsFixture)
			ts, cleanup := startTestServer(t, path)
			defer cleanup()

			res := patchJSON(t, ts.URL+"/api/columns/"+tc.column, tc.body)
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatalf("status = %d", res.StatusCode)
			}

			boardRes, err := http.Get(ts.URL + "/api/board")
			if err != nil {
				t.Fatalf("GET /api/board: %v", err)
			}
			defer boardRes.Body.Close()
			var payload struct {
				DoneColumns []string `json:"done_columns"`
			}
			if err := json.NewDecoder(boardRes.Body).Decode(&payload); err != nil {
				t.Fatalf("decode board: %v", err)
			}
			if !reflectStringSliceEqual(payload.DoneColumns, tc.wantDone) {
				t.Fatalf("done_columns = %v, want %v", payload.DoneColumns, tc.wantDone)
			}
		})
	}
}

func TestHandleColumnPatch_EmptyPatchRejected(t *testing.T) {
	path := writableBoardWithBody(t, terminalColumnsFixture)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := patchJSON(t, ts.URL+"/api/columns/todo", `{}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if got := readString(res.Body); !strings.Contains(got, `"INVALID_BODY"`) {
		t.Fatalf("missing INVALID_BODY: %s", got)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("on-disk file changed despite 400 error")
	}
}

// TestHandleColumnPatch_UnknownColumnEvenWhenNoop covers the edge the
// up-front membership check tightened: RenameColumn short-circuits on
// from == to before looking anything up, so this used to answer 200.
func TestHandleColumnPatch_UnknownColumnEvenWhenNoop(t *testing.T) {
	path := writableBoardWithBody(t, terminalColumnsFixture)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	for _, body := range []string{`{"name":"ghost"}`, `{"done":true}`} {
		res := patchJSON(t, ts.URL+"/api/columns/ghost", body)
		if res.StatusCode != 400 {
			t.Fatalf("body %q: status = %d, want 400", body, res.StatusCode)
		}
		got := readString(res.Body)
		res.Body.Close()
		if !strings.Contains(got, `"COLUMN_NOT_FOUND"`) {
			t.Fatalf("body %q: missing COLUMN_NOT_FOUND: %s", body, got)
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatalf("body %q: on-disk file changed despite 400 error", body)
		}
	}
}
