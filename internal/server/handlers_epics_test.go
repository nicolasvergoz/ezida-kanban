package server

import (
	"encoding/json"
	"io"
	"net/http"
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
// presence semantics as well as on names.
func jsonTags(t reflect.Type) []string {
	out := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}
