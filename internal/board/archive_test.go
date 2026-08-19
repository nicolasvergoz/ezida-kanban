package board

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// --- ArchivedCard embedding guard -------------------------------------------

// tomlTagsOf recursively collects the toml tag names of t, descending
// into anonymous embedded struct fields the way go-toml itself does.
func tomlTagsOf(t reflect.Type) map[string]struct{} {
	tags := make(map[string]struct{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			for tag := range tomlTagsOf(f.Type) {
				tags[tag] = struct{}{}
			}
			continue
		}
		tag := f.Tag.Get("toml")
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		tags[name] = struct{}{}
	}
	return tags
}

func TestArchivedCard_EmbedsCardVerbatim(t *testing.T) {
	acType := reflect.TypeOf(ArchivedCard{})
	field0 := acType.Field(0)
	if !field0.Anonymous {
		t.Fatalf("ArchivedCard field 0 (%s) must be anonymous", field0.Name)
	}
	if field0.Type != reflect.TypeOf(Card{}) {
		t.Fatalf("ArchivedCard field 0 has type %s, want board.Card", field0.Type)
	}

	got := tomlTagsOf(acType)
	want := tomlTagsOf(reflect.TypeOf(Card{}))
	want["archived_at"] = struct{}{}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ArchivedCard toml tags = %v, want %v", got, want)
	}
}

// --- Round trip --------------------------------------------------------------

func TestArchive_TOMLRoundTrip(t *testing.T) {
	at := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC)

	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards: []ArchivedCard{
			{
				Card: Card{
					ID:          "a3f2k9",
					Title:       "Refactor auth",
					Column:      "done",
					Description: "JWT migration",
					CreatedAt:   created,
					UpdatedAt:   updated,
					Tags:        []string{"security"},
					Priority:    "high",
					Epic:        "rl4m9x",
					Color:       "#ef4444",
				},
				ArchivedAt: at,
			},
		},
	}

	data, err := toml.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var back Archive
	if err := toml.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(back.Cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(back.Cards))
	}
	got := back.Cards[0]
	want := a.Cards[0]
	if got.ID != want.ID || got.Title != want.Title || got.Column != want.Column ||
		got.Description != want.Description || !got.CreatedAt.Equal(want.CreatedAt) ||
		!got.UpdatedAt.Equal(want.UpdatedAt) || got.Priority != want.Priority ||
		got.Epic != want.Epic || got.Color != want.Color || !got.ArchivedAt.Equal(want.ArchivedAt) {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, want)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "security" {
		t.Fatalf("tags round trip = %v, want [security]", got.Tags)
	}

	text := string(data)
	if !strings.Contains(text, "archived_at") {
		t.Fatalf("marshaled archive does not contain archived_at key:\n%s", text)
	}
	// archived_at must be a flat key inside [[cards]], not a nested table.
	if strings.Contains(text, "[cards.archived") || strings.Contains(text, "[[cards.archived") {
		t.Fatalf("archived_at appears to be a nested table, not a flat key:\n%s", text)
	}
}

func TestArchive_UnsetOptionalFieldsStayAbsent(t *testing.T) {
	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards: []ArchivedCard{
			{
				Card: Card{
					ID:        "b7m1p4",
					Title:     "No optionals",
					Column:    "done",
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
				},
				ArchivedAt: time.Now().UTC(),
			},
		},
	}
	data, err := toml.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(data)
	for _, key := range []string{"epic", "color", "priority"} {
		if strings.Contains(text, key+" =") {
			t.Fatalf("marshaled archive contains unset key %q:\n%s", key, text)
		}
	}
}

// --- LoadArchive / SaveArchive -----------------------------------------------

func TestLoadArchive_MissingFileIsEmptyNotError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.archive.toml")

	a, existed, err := LoadArchive(path)
	if err != nil {
		t.Fatalf("LoadArchive(missing) returned err = %v, want nil", err)
	}
	if existed {
		t.Fatalf("LoadArchive(missing) returned existed = true, want false")
	}
	if a == nil || len(a.Cards) != 0 {
		t.Fatalf("LoadArchive(missing) = %+v, want empty archive", a)
	}
	if a.SchemaVersion != SupportedSchemaVersion {
		t.Fatalf("LoadArchive(missing) schema version = %d, want %d", a.SchemaVersion, SupportedSchemaVersion)
	}
}

func TestSaveArchive_RemovesFileWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.archive.toml")

	// Seed a non-empty archive first.
	full := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards: []ArchivedCard{{
			Card: Card{
				ID: "a3f2k9", Title: "x", Column: "done",
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			},
			ArchivedAt: time.Now().UTC(),
		}},
	}
	if err := SaveArchive(path, full); err != nil {
		t.Fatalf("SaveArchive(full): %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("archive file missing after non-empty save: %v", err)
	}

	empty := &Archive{SchemaVersion: SupportedSchemaVersion, Cards: []ArchivedCard{}}
	if err := SaveArchive(path, empty); err != nil {
		t.Fatalf("SaveArchive(empty): %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("archive file still exists after emptying, err = %v", err)
	}

	// Removing an already-absent archive must not error.
	if err := SaveArchive(path, empty); err != nil {
		t.Fatalf("SaveArchive(empty) on absent file: %v", err)
	}
}

func TestSaveArchive_IsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.archive.toml")
	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards: []ArchivedCard{{
			Card: Card{
				ID: "a3f2k9", Title: "x", Column: "done",
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			},
			ArchivedAt: time.Now().UTC(),
		}},
	}
	if err := SaveArchive(path, a); err != nil {
		t.Fatalf("SaveArchive: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Fatalf("leftover temp file after successful SaveArchive: %s", e.Name())
		}
	}
}

func TestArchivePathFor(t *testing.T) {
	cases := []struct{ in, want string }{
		{"kanban.toml", "kanban.archive.toml"},
		{"/a/b/kanban.toml", "/a/b/kanban.archive.toml"},
		{"board.toml", "board.archive.toml"},
	}
	for _, c := range cases {
		got := ArchivePathFor(c.in)
		if got != c.want {
			t.Errorf("ArchivePathFor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- ReconcileArchive ---------------------------------------------------------

func TestReconcileArchive_BoardWins(t *testing.T) {
	b := &Board{Cards: []Card{{ID: "aaaaaa"}}}
	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards: []ArchivedCard{
			{Card: Card{ID: "aaaaaa"}}, // duplicate of the live card
			{Card: Card{ID: "bbbbbb"}},
		},
	}
	dropped := ReconcileArchive(b, a)
	if len(dropped) != 1 || dropped[0] != "aaaaaa" {
		t.Fatalf("dropped = %v, want [aaaaaa]", dropped)
	}
	if len(a.Cards) != 1 || a.Cards[0].ID != "bbbbbb" {
		t.Fatalf("a.Cards = %v, want only bbbbbb", a.Cards)
	}
}

func TestReconcileArchive_NoOpWhenNoDuplicate(t *testing.T) {
	b := &Board{Cards: []Card{{ID: "aaaaaa"}}}
	a := &Archive{SchemaVersion: SupportedSchemaVersion, Cards: []ArchivedCard{{Card: Card{ID: "bbbbbb"}}}}
	dropped := ReconcileArchive(b, a)
	if len(dropped) != 0 {
		t.Fatalf("dropped = %v, want none", dropped)
	}
	if len(a.Cards) != 1 {
		t.Fatalf("a.Cards mutated unexpectedly: %v", a.Cards)
	}
}

// --- ExistingIDs ---------------------------------------------------------------

func TestExistingIDs_TolerateNilArchive(t *testing.T) {
	b := &Board{Cards: []Card{{ID: "aaaaaa"}, {ID: "bbbbbb"}}}
	got := ExistingIDs(b, nil)
	if len(got) != 2 {
		t.Fatalf("ExistingIDs(nil archive) = %v, want 2 ids", got)
	}
}

func TestExistingIDs_UnionsBoardAndArchive(t *testing.T) {
	b := &Board{Cards: []Card{{ID: "aaaaaa"}}}
	a := &Archive{Cards: []ArchivedCard{{Card: Card{ID: "bbbbbb"}}}}
	got := ExistingIDs(b, a)
	set := map[string]bool{}
	for _, id := range got {
		set[id] = true
	}
	if !set["aaaaaa"] || !set["bbbbbb"] {
		t.Fatalf("ExistingIDs = %v, want both aaaaaa and bbbbbb", got)
	}
}
