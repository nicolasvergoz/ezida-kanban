package board

import "testing"

// TestColumnsCodec_RoundTrip exercises DecodeColumns and EncodeColumns
// as a pair: for every input that is representable, encoding the
// decoded form must reproduce the input exactly.
func TestColumnsCodec_RoundTrip(t *testing.T) {
	cases := []struct {
		name      string
		raw       []string
		wantNames []string
		wantDone  []string
	}{
		{
			name:      "no markers",
			raw:       []string{"todo", "ongoing", "done"},
			wantNames: []string{"todo", "ongoing", "done"},
			wantDone:  nil,
		},
		{
			name:      "one marker",
			raw:       []string{"backlog", "todo", "ongoing", "done*"},
			wantNames: []string{"backlog", "todo", "ongoing", "done"},
			wantDone:  []string{"done"},
		},
		{
			name:      "multiple markers",
			raw:       []string{"todo", "shipped*", "wont-fix*"},
			wantNames: []string{"todo", "shipped", "wont-fix"},
			wantDone:  []string{"shipped", "wont-fix"},
		},
		{
			// A `*` anywhere but the end is not a marker. The codec
			// leaves it alone; rule 16 is what rejects the name.
			name:      "star in the middle is part of the name",
			raw:       []string{"todo", "do*ne"},
			wantNames: []string{"todo", "do*ne"},
			wantDone:  nil,
		},
		{
			name:      "empty list",
			raw:       []string{},
			wantNames: []string{},
			wantDone:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			names, done := DecodeColumns(tc.raw)
			if !equalStringSlices(names, tc.wantNames) {
				t.Fatalf("names = %v, want %v", names, tc.wantNames)
			}
			for _, want := range tc.wantDone {
				if !done[want] {
					t.Errorf("column %q not reported as terminal", want)
				}
			}
			if len(done) != len(tc.wantDone) {
				t.Errorf("terminal count = %d, want %d", len(done), len(tc.wantDone))
			}
			if got := EncodeColumns(names, done); !equalStringSlices(got, tc.raw) {
				t.Fatalf("re-encoded = %v, want %v", got, tc.raw)
			}
		})
	}
}

// TestColumnsCodec_Collision documents the one input that is NOT
// round-trip stable: two entries collapsing to the same name. The codec
// surfaces the duplicate rather than hiding it, so rule 17 can reject
// the file.
func TestColumnsCodec_Collision(t *testing.T) {
	names, done := DecodeColumns([]string{"done", "done*"})
	if !equalStringSlices(names, []string{"done", "done"}) {
		t.Fatalf("names = %v, want two entries both named done", names)
	}
	if !done["done"] {
		t.Fatal("the marked entry did not set the terminal flag")
	}

	b := &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board:         BoardConfig{Columns: names, Priorities: []string{"low"}, doneColumns: done},
	}
	verr := Validate(b)
	if verr == nil || !hasViolationForRule(verr, 17) {
		t.Fatalf("expected rule 17 violation, got %v", verr)
	}
}

// TestEncodeColumns_DropsStaleFlags proves the encoder cannot emit a
// marker for a column that no longer exists — the property that frees
// column deletion from any propagation work.
func TestEncodeColumns_DropsStaleFlags(t *testing.T) {
	got := EncodeColumns([]string{"todo"}, map[string]bool{"todo": true, "deleted": true})
	if !equalStringSlices(got, []string{"todo*"}) {
		t.Fatalf("encoded = %v, want [todo*]", got)
	}
}

func TestBoardConfig_DoneColumnsFollowDeclarationOrder(t *testing.T) {
	cfg := BoardConfig{Columns: []string{"todo", "shipped", "wont-fix"}}
	cfg.SetDoneColumn("wont-fix", true)
	cfg.SetDoneColumn("shipped", true)
	if got := cfg.DoneColumns(); !equalStringSlices(got, []string{"shipped", "wont-fix"}) {
		t.Fatalf("DoneColumns() = %v, want [shipped wont-fix]", got)
	}
	cfg.SetDoneColumn("shipped", false)
	if cfg.IsDoneColumn("shipped") {
		t.Error("shipped still terminal after clearing")
	}
	if got := cfg.DoneColumns(); !equalStringSlices(got, []string{"wont-fix"}) {
		t.Fatalf("DoneColumns() = %v, want [wont-fix]", got)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
