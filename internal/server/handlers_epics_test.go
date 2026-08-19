package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// epicsBoardFixture is a board with one epic (rl4m9x, colored) holding
// three children, one of which sits in the terminal column, plus one
// unrelated card. Two terminal columns exercise the `*`-stripping and
// the done_columns ordering at once.
const epicsBoardFixture = `schema_version = 2

[board]
columns = ["todo", "shipped*", "wont-fix*"]
priorities = ["low", "medium", "high"]

[[cards]]
id = "rl4m9x"
title = "Card relations"
column = "todo"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
color = "#8b5cf6"

[[cards]]
id = "f20wbo"
title = "Card dependencies"
column = "todo"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = ["schema"]
epic = "rl4m9x"

[[cards]]
id = "wrshlo"
title = "Card due dates"
column = "shipped"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
epic = "rl4m9x"

[[cards]]
id = "q7t6z2"
title = "Card colors"
column = "todo"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
epic = "rl4m9x"

[[cards]]
id = "a3f2k9"
title = "Refactor auth"
column = "todo"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
priority = "high"
`

// plainBoardFixture carries no epic and no color, so /api/board must
// produce a payload byte-identical to the pre-epic shape.
const plainBoardFixture = `schema_version = 2

[board]
columns = ["todo", "done"]
priorities = ["low"]

[[cards]]
id = "aaaaaa"
title = "Plain"
column = "todo"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
`

// fetchBoardRaw returns GET /api/board decoded into generic maps, so
// tests can assert on key *presence* rather than on decoded zero
// values (omitempty is only observable this way).
func fetchBoardRaw(t *testing.T, url string) map[string]any {
	t.Helper()
	res, err := http.Get(url + "/api/board")
	if err != nil {
		t.Fatalf("GET /api/board: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return payload
}

// rawCards returns the payload's cards as generic maps.
func rawCards(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	raw, ok := payload["cards"].([]any)
	if !ok {
		t.Fatalf("cards is not an array: %T", payload["cards"])
	}
	out := make([]map[string]any, 0, len(raw))
	for _, c := range raw {
		m, ok := c.(map[string]any)
		if !ok {
			t.Fatalf("card is not an object: %T", c)
		}
		out = append(out, m)
	}
	return out
}

func TestHandle_Board_EpicAndColorPerCard(t *testing.T) {
	path := writeBoardFixture(t, epicsBoardFixture)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	byID := make(map[string]map[string]any)
	for _, c := range rawCards(t, fetchBoardRaw(t, ts.URL)) {
		byID[c["id"].(string)] = c
	}

	if got := byID["f20wbo"]["epic"]; got != "rl4m9x" {
		t.Errorf("f20wbo.epic = %v, want rl4m9x", got)
	}
	if got := byID["rl4m9x"]["color"]; got != "#8b5cf6" {
		t.Errorf("rl4m9x.color = %v, want #8b5cf6", got)
	}
}

func TestHandle_Board_NoEpicOmitsKeys(t *testing.T) {
	path := writeBoardFixture(t, plainBoardFixture)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	for _, c := range rawCards(t, fetchBoardRaw(t, ts.URL)) {
		for _, key := range []string{"epic", "color"} {
			if _, present := c[key]; present {
				t.Errorf("card %v carries %q on a board with no epics", c["id"], key)
			}
		}
	}
}

// The board endpoint returns every card on every request, so the client
// resolves relations itself. Denormalizing would create a second source
// of truth every mutation endpoint would have to keep correct (D1).
func TestHandle_Board_NoDenormalizedRelationData(t *testing.T) {
	path := writeBoardFixture(t, epicsBoardFixture)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	for _, c := range rawCards(t, fetchBoardRaw(t, ts.URL)) {
		for _, key := range []string{"epic_title", "epic_color", "children", "progress"} {
			if _, present := c[key]; present {
				t.Errorf("card %v carries denormalized field %q", c["id"], key)
			}
		}
	}
}

func TestHandle_Board_DoneColumnsReportedSeparately(t *testing.T) {
	path := writeBoardFixture(t, epicsBoardFixture)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	res, err := http.Get(ts.URL + "/api/board")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	var payload struct {
		Columns     []string `json:"columns"`
		DoneColumns []string `json:"done_columns"`
	}
	body := &strings.Builder{}
	if err := json.NewDecoder(io.TeeReader(res.Body, body)).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}

	wantCols := []string{"todo", "shipped", "wont-fix"}
	if !reflect.DeepEqual(payload.Columns, wantCols) {
		t.Errorf("columns = %v, want %v", payload.Columns, wantCols)
	}
	wantDone := []string{"shipped", "wont-fix"}
	if !reflect.DeepEqual(payload.DoneColumns, wantDone) {
		t.Errorf("done_columns = %v, want %v", payload.DoneColumns, wantDone)
	}
	// The `*` marker is a kanban.toml spelling; it must never reach any
	// response field, not just the columns array.
	if strings.Contains(body.String(), "*") {
		t.Errorf("response body contains a '*' character: %s", body.String())
	}
}

func TestHandle_Board_NoTerminalColumnYieldsEmptyArray(t *testing.T) {
	path := writeBoardFixture(t, plainBoardFixture)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	payload := fetchBoardRaw(t, ts.URL)
	got, present := payload["done_columns"]
	if !present {
		t.Fatal("done_columns absent; it must always be present")
	}
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("done_columns = %v (%T), want []", got, got)
	}
	if len(arr) != 0 {
		t.Errorf("done_columns = %v, want []", arr)
	}
}

// TestWireShape_ExportMatchesBoard is the successor to the drift test
// that pinned `ezida export` behind `GET /api/board`: output.ExportCard
// and cardResponse (and their envelopes) are parallel structs, and this
// enforces that a field added to one is added to the other. Comparing
// the json tags rather than two rendered documents catches the drift at
// the point it is introduced, including omitempty divergence, which two
// same-fixture documents would not reveal.
func TestWireShape_ExportMatchesBoard(t *testing.T) {
	cases := []struct {
		what   string
		server any
		export any
	}{
		{"envelope", boardResponse{}, output.ExportEnvelope{}},
		{"card", cardResponse{}, output.ExportCard{}},
		{"archived card", archivedCardResponse{}, output.ArchivedExportCard{}},
	}
	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			got := jsonTags(reflect.TypeOf(tc.server))
			want := jsonTags(reflect.TypeOf(tc.export))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("json tags diverged\n  server: %v\n  export: %v", got, want)
			}
		})
	}
}

// jsonTags returns the sorted `json:"..."` tags of a struct type,
// including the omitempty suffix so the two shapes must agree on
// presence semantics as well as on names. An anonymous embedded field
// with no direct tag (the way archivedCardResponse embeds
// cardResponse) contributes its own type's tags in its place rather
// than being skipped — otherwise a struct that differs only in what it
// embeds would compare equal on nothing but the fields it added.
func jsonTags(t reflect.Type) []string {
	out := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" {
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				out = append(out, jsonTags(f.Type)...)
			}
			continue
		}
		if tag == "-" {
			continue
		}
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

// --- PATCH /api/cards/{id}: epic and color ---------------------------------

// patchCardRaw issues a PATCH and returns the status, the response
// body's `card` as a generic map (nil when the response carries no
// card), and the raw body for message assertions. Generic maps are
// what make omitempty observable: a cleared field is an *absent* key,
// not a zero value.
func patchCardRaw(t *testing.T, url, body string) (int, map[string]any, string) {
	t.Helper()
	res := patchJSON(t, url, body)
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	card, _ := payload["card"].(map[string]any)
	return res.StatusCode, card, string(raw)
}

// errorEnvelope extracts code, message and details from a refusal.
func errorEnvelope(t *testing.T, body string) (code, message string, details map[string]any) {
	t.Helper()
	var payload struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode error envelope %s: %v", body, err)
	}
	return payload.Error.Code, payload.Error.Message, payload.Error.Details
}

// Attaching a card to a colorless target colors that target in the
// same write: acquiring a child is what makes a card an epic, and an
// epic with no color has nothing to lend its children.
func TestHandle_Patch_AttachToEpicColorsTheTarget(t *testing.T) {
	path := writeBoardFixture(t, epicsBoardFixture)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	status, card, body := patchCardRaw(t, ts.URL+"/api/cards/q7t6z2", `{"epic":"a3f2k9"}`)
	if status != 200 {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	if got := card["epic"]; got != "a3f2k9" {
		t.Errorf("response epic = %v, want a3f2k9", got)
	}
	target := findCardOnDisk(t, path, "a3f2k9")
	if target == nil || target.Color == "" {
		t.Fatalf("target acquired no color: %+v", target)
	}
}

func TestHandle_Patch_DetachClearsTheEpicKey(t *testing.T) {
	path := writeBoardFixture(t, epicsBoardFixture)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	status, card, body := patchCardRaw(t, ts.URL+"/api/cards/f20wbo", `{"epic":""}`)
	if status != 200 {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	if _, present := card["epic"]; present {
		t.Errorf("detached card still carries an epic key: %s", body)
	}
	if disk := findCardOnDisk(t, path, "f20wbo"); disk == nil || disk.Epic != "" {
		t.Errorf("on-disk epic = %q, want empty", disk.Epic)
	}
}

// The four epic refusals must be 400s carrying a code a client can
// branch on. Before this change every one of them left through
// httpError's catch-all as 500 IO_ERROR.
func TestHandle_Patch_EpicRefusals(t *testing.T) {
	cases := []struct {
		name   string
		id     string
		body   string
		wants  string
		target string
	}{
		{"unknown target", "a3f2k9", `{"epic":"zzzzzz"}`, "no card on this board carries that id", "zzzzzz"},
		{"self reference", "a3f2k9", `{"epic":"a3f2k9"}`, "cannot be its own epic", "a3f2k9"},
		{"target already a child", "a3f2k9", `{"epic":"f20wbo"}`, "already belongs", "f20wbo"},
		{"card has children", "rl4m9x", `{"epic":"a3f2k9"}`, "children of its own", "a3f2k9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeBoardFixture(t, epicsBoardFixture)
			before, _ := os.ReadFile(path)
			ts, cleanup := startTestServer(t, path)
			defer cleanup()

			status, _, body := patchCardRaw(t, ts.URL+"/api/cards/"+tc.id, tc.body)
			if status != 400 {
				t.Fatalf("status = %d, want 400, body = %s", status, body)
			}
			code, message, details := errorEnvelope(t, body)
			if code != "INVALID_EPIC" {
				t.Errorf("code = %s, want INVALID_EPIC", code)
			}
			if !strings.Contains(message, tc.wants) {
				t.Errorf("message %q does not explain the refusal (%q)", message, tc.wants)
			}
			if details["epic"] != tc.target {
				t.Errorf("details = %v, want epic %q", details, tc.target)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Error("a refused PATCH modified kanban.toml")
			}
		})
	}
}

func TestHandle_Patch_SetAndClearColor(t *testing.T) {
	path := writeBoardFixture(t, epicsBoardFixture)
	ts, cleanup := startTestServer(t, path)
	defer cleanup()

	status, card, body := patchCardRaw(t, ts.URL+"/api/cards/rl4m9x", `{"color":"#10b981"}`)
	if status != 200 {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	if got := card["color"]; got != "#10b981" {
		t.Errorf("response color = %v, want #10b981", got)
	}

	status, card, body = patchCardRaw(t, ts.URL+"/api/cards/rl4m9x", `{"color":""}`)
	if status != 200 {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	if _, present := card["color"]; present {
		t.Errorf("cleared card still carries a color key: %s", body)
	}
}

// Palette names are a CLI convenience; only hex reaches the file, so
// only hex is accepted on the wire.
func TestHandle_Patch_ColorRefusals(t *testing.T) {
	for _, value := range []string{"blue", "#12"} {
		t.Run(value, func(t *testing.T) {
			path := writeBoardFixture(t, epicsBoardFixture)
			before, _ := os.ReadFile(path)
			ts, cleanup := startTestServer(t, path)
			defer cleanup()

			status, _, body := patchCardRaw(t, ts.URL+"/api/cards/rl4m9x",
				`{"color":"`+value+`"}`)
			if status != 400 {
				t.Fatalf("status = %d, want 400, body = %s", status, body)
			}
			code, _, details := errorEnvelope(t, body)
			if code != "INVALID_COLOR" {
				t.Errorf("code = %s, want INVALID_COLOR", code)
			}
			if details["color"] != value {
				t.Errorf("details = %v, want color %q", details, value)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Error("a refused PATCH modified kanban.toml")
			}
		})
	}
}
