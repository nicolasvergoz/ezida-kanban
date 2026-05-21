package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// deleteJSON is the DELETE counterpart of postJSON/patchJSON.
func deleteJSON(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	return res
}

// --- POST /api/cards tests -------------------------------------------------

var createIDPattern = regexp.MustCompile(`^[0-9a-z]{6}$`)

func TestHandle_Create_Success_TitleOnly(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards",
		`{"column":"todo","title":"Draft v1"}`)
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var body struct {
		Card struct {
			ID        string    `json:"id"`
			Title     string    `json:"title"`
			Column    string    `json:"column"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"card"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !createIDPattern.MatchString(body.Card.ID) {
		t.Fatalf("id = %q, want [0-9a-z]{6}", body.Card.ID)
	}
	if body.Card.Title != "Draft v1" {
		t.Fatalf("title = %q, want %q", body.Card.Title, "Draft v1")
	}
	if body.Card.Column != "todo" {
		t.Fatalf("column = %q, want todo", body.Card.Column)
	}
	if !body.Card.CreatedAt.Equal(body.Card.UpdatedAt) {
		t.Fatalf("created_at (%s) != updated_at (%s)", body.Card.CreatedAt, body.Card.UpdatedAt)
	}

	disk := findCardOnDisk(t, path, body.Card.ID)
	if disk == nil {
		t.Fatalf("card not on disk")
	}
	if disk.Column != "todo" {
		t.Fatalf("on-disk column = %q, want todo", disk.Column)
	}
}

func TestHandle_Create_Success_AllFields(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards",
		`{"column":"todo","title":"Refactor auth","description":"split out tokens","priority":"high","tags":["security","tech-debt"]}`)
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}

	var body struct {
		Card struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Column      string   `json:"column"`
			Description string   `json:"description"`
			Priority    string   `json:"priority"`
			Tags        []string `json:"tags"`
		} `json:"card"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Card.Description != "split out tokens" {
		t.Fatalf("description = %q", body.Card.Description)
	}
	if body.Card.Priority != "high" {
		t.Fatalf("priority = %q, want high", body.Card.Priority)
	}
	if len(body.Card.Tags) != 2 || body.Card.Tags[0] != "security" || body.Card.Tags[1] != "tech-debt" {
		t.Fatalf("tags = %v", body.Card.Tags)
	}

	disk := findCardOnDisk(t, path, body.Card.ID)
	if disk == nil {
		t.Fatalf("card not on disk")
	}
	if disk.Description != "split out tokens" {
		t.Fatalf("on-disk description = %q", disk.Description)
	}
	if disk.Priority != "high" {
		t.Fatalf("on-disk priority = %q, want high", disk.Priority)
	}
}

func TestHandle_Create_UnknownColumn(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards", `{"column":"ghost","title":"x"}`)
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"COLUMN_NOT_FOUND"`) {
		t.Fatalf("body missing COLUMN_NOT_FOUND: %s", body)
	}
	if !strings.Contains(body, `"column":"ghost"`) {
		t.Fatalf("body missing column detail: %s", body)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 404 error")
	}
}

// TestHandle_Create_UnknownColumn_Returns404 is a dedicated guard for
// design.md §D2: the create handler emits 404 for COLUMN_NOT_FOUND even
// though httpError maps the same typed error to 400 on the move
// endpoint. Would fail if the handler accidentally went through
// httpError.
func TestHandle_Create_UnknownColumn_Returns404(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards", `{"column":"ghost","title":"x"}`)
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("status = %d, want 404 (design.md §D2)", res.StatusCode)
	}
	// Sanity: the move endpoint still returns 400 for the same typed
	// error condition.
	resMove := postJSON(t, ts.URL+"/api/cards/aaaaaa/move",
		`{"column":"ghost","position":0}`)
	defer resMove.Body.Close()
	if resMove.StatusCode != 400 {
		t.Fatalf("move status = %d, want 400 (parity check)", resMove.StatusCode)
	}
}

func TestHandle_Create_EmptyTitle(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards", `{"column":"todo","title":"   "}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"MISSING_TITLE"`) {
		t.Fatalf("body missing MISSING_TITLE")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandle_Create_MissingTitleKey(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards", `{"column":"todo"}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"MISSING_TITLE"`) {
		t.Fatalf("body missing MISSING_TITLE")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandle_Create_UnknownPriority(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards",
		`{"column":"todo","title":"x","priority":"urgent"}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"INVALID_PRIORITY"`) {
		t.Fatalf("body missing INVALID_PRIORITY: %s", body)
	}
	if !strings.Contains(body, `"priority":"urgent"`) {
		t.Fatalf("body missing priority detail: %s", body)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandle_Create_EmptyTag(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards",
		`{"column":"todo","title":"x","tags":["good",""]}`)
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	if !strings.Contains(readString(res.Body), `"INVALID_TAG"`) {
		t.Fatalf("body missing INVALID_TAG")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 400 error")
	}
}

func TestHandle_Create_MalformedBody(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards", `{not valid json`)
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

func TestHandle_Create_AppendsToEndOfColumn(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	// Seed has aaaaaa, bbbbbb in todo (2 cards). Add a 3rd, then the
	// create should make a 4th card with column-relative position 3.
	res1 := postJSON(t, ts.URL+"/api/cards", `{"column":"todo","title":"three"}`)
	res1.Body.Close()
	if res1.StatusCode != 201 {
		t.Fatalf("seed create 1 status = %d", res1.StatusCode)
	}

	res := postJSON(t, ts.URL+"/api/cards", `{"column":"todo","title":"four"}`)
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("status = %d", res.StatusCode)
	}

	b, err := readBoardFile(path)
	if err != nil {
		t.Fatalf("readBoardFile: %v", err)
	}
	var todoOrder []string
	for _, c := range b.Cards {
		if c.Column == "todo" {
			todoOrder = append(todoOrder, c.Title)
		}
	}
	if len(todoOrder) != 4 {
		t.Fatalf("todo has %d cards, want 4", len(todoOrder))
	}
	if todoOrder[len(todoOrder)-1] != "four" {
		t.Fatalf("last todo card = %q, want %q", todoOrder[len(todoOrder)-1], "four")
	}
}

func TestHandle_Create_CreatedAtEqualsUpdatedAt(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := postJSON(t, ts.URL+"/api/cards", `{"column":"todo","title":"ts check"}`)
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var body struct {
		Card struct {
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"card"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Card.CreatedAt.Equal(body.Card.UpdatedAt) {
		t.Fatalf("created_at (%s) != updated_at (%s)", body.Card.CreatedAt, body.Card.UpdatedAt)
	}
}

// TestHandle_Create_BroadcastsBoardChanged exercises the full
// runWithContext bring-up (which wires the fsnotify watcher to the
// broker) so the SSE pipeline receives a board-changed event after a
// successful POST /api/cards. Mirrors TestRun_HotReload_Smoke.
func TestHandle_Create_BroadcastsBoardChanged(t *testing.T) {
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

	createRes := postJSON(t, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/cards",
		`{"column":"todo","title":"sse check"}`)
	defer createRes.Body.Close()
	if createRes.StatusCode != 201 {
		t.Fatalf("create status = %d", createRes.StatusCode)
	}

	chunk := readSSEChunk(br, 2*time.Second)
	if !strings.Contains(chunk, "event: board-changed") {
		t.Fatalf("expected board-changed event, got: %q", chunk)
	}
}

// --- DELETE /api/cards/{id} tests ------------------------------------------

func TestHandle_Delete_Success(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := deleteJSON(t, ts.URL+"/api/cards/aaaaaa")
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, body = %s", res.StatusCode, readString(res.Body))
	}
	if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var body struct {
		Deleted string `json:"deleted"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Deleted != "aaaaaa" {
		t.Fatalf("deleted = %q, want aaaaaa", body.Deleted)
	}

	if got := findCardOnDisk(t, path, "aaaaaa"); got != nil {
		t.Fatalf("card aaaaaa still on disk: %+v", got)
	}
	// Surviving cards (bbbbbb, cccccc) order preserved.
	b, err := readBoardFile(path)
	if err != nil {
		t.Fatalf("readBoardFile: %v", err)
	}
	if len(b.Cards) != 2 {
		t.Fatalf("len(cards) = %d, want 2", len(b.Cards))
	}
	if b.Cards[0].ID != "bbbbbb" || b.Cards[1].ID != "cccccc" {
		t.Fatalf("survivor order = [%s, %s], want [bbbbbb, cccccc]", b.Cards[0].ID, b.Cards[1].ID)
	}
}

func TestHandle_Delete_UnknownCard(t *testing.T) {
	path := writableBoard(t)
	before, _ := os.ReadFile(path)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res := deleteJSON(t, ts.URL+"/api/cards/zzzzzz")
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	body := readString(res.Body)
	if !strings.Contains(body, `"CARD_NOT_FOUND"`) {
		t.Fatalf("body missing CARD_NOT_FOUND: %s", body)
	}
	if !strings.Contains(body, `"id":"zzzzzz"`) {
		t.Fatalf("body missing id detail: %s", body)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatalf("on-disk file changed despite 404 error")
	}
}

func TestHandle_Delete_NonDeleteMethodRejected(t *testing.T) {
	path := writableBoard(t)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	// POST /api/cards/<id> without /move suffix — the router should
	// not deliver to handleDelete (which only registers DELETE).
	// v1 latitude: 405 or 404 both acceptable.
	res := postJSON(t, ts.URL+"/api/cards/aaaaaa", `{}`)
	defer res.Body.Close()
	if res.StatusCode != 405 && res.StatusCode != 404 {
		t.Fatalf("status = %d, want 405 or 404", res.StatusCode)
	}
}

func TestHandle_Delete_BroadcastsBoardChanged(t *testing.T) {
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

	deleteRes := deleteJSON(t, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/cards/aaaaaa")
	defer deleteRes.Body.Close()
	if deleteRes.StatusCode != 200 {
		t.Fatalf("delete status = %d", deleteRes.StatusCode)
	}

	chunk := readSSEChunk(br, 2*time.Second)
	if !strings.Contains(chunk, "event: board-changed") {
		t.Fatalf("expected board-changed event, got: %q", chunk)
	}
}

// Silence unused-import warnings if helpers above don't reach io.
var _ = io.EOF
