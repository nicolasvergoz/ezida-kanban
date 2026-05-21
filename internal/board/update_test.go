package board

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

// updateTestBoard returns a valid in-memory board with one card per
// column. The card aaaaaa carries every patchable field so tests can
// observe per-field replacement.
func updateTestBoard() *Board {
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	return &Board{
		SchemaVersion: 1,
		Board: BoardConfig{
			Columns:    []string{"todo", "done"},
			Priorities: []string{"low", "medium", "high"},
		},
		Cards: []Card{
			{
				ID:          "aaaaaa",
				Title:       "Old",
				Column:      "todo",
				Description: "old desc",
				Tags:        []string{"a", "b"},
				Priority:    "high",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "bbbbbb",
				Title:       "Second",
				Column:      "done",
				Description: "",
				Tags:        []string{},
				Priority:    "",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
}

func findCard(t *testing.T, b *Board, id string) Card {
	t.Helper()
	for _, c := range b.Cards {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("card %q not found in board", id)
	return Card{}
}

func TestUpdateCard_TitleOnly(t *testing.T) {
	b := updateTestBoard()
	if err := UpdateCard(b, "aaaaaa", CardPatch{Title: ptr("New")}); err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}
	c := findCard(t, b, "aaaaaa")
	if c.Title != "New" {
		t.Fatalf("Title = %q, want %q", c.Title, "New")
	}
	if c.Description != "old desc" {
		t.Fatalf("Description was modified: got %q", c.Description)
	}
	if len(c.Tags) != 2 || c.Tags[0] != "a" || c.Tags[1] != "b" {
		t.Fatalf("Tags mutated: %v", c.Tags)
	}
	if c.Priority != "high" {
		t.Fatalf("Priority mutated: got %q", c.Priority)
	}
}

func TestUpdateCard_ClearPriority(t *testing.T) {
	b := updateTestBoard()
	if err := UpdateCard(b, "aaaaaa", CardPatch{Priority: ptr("")}); err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}
	c := findCard(t, b, "aaaaaa")
	if c.Priority != "" {
		t.Fatalf("Priority = %q, want empty", c.Priority)
	}
}

func TestUpdateCard_ClearTags(t *testing.T) {
	b := updateTestBoard()
	empty := []string{}
	if err := UpdateCard(b, "aaaaaa", CardPatch{Tags: &empty}); err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}
	c := findCard(t, b, "aaaaaa")
	if len(c.Tags) != 0 {
		t.Fatalf("Tags = %v, want empty slice", c.Tags)
	}
}

func TestUpdateCard_EmptyTitle(t *testing.T) {
	b := updateTestBoard()
	before := findCard(t, b, "aaaaaa")
	err := UpdateCard(b, "aaaaaa", CardPatch{Title: ptr("   ")})
	if err == nil {
		t.Fatalf("UpdateCard returned nil, want *MissingTitleError")
	}
	var mte *MissingTitleError
	if !errors.As(err, &mte) {
		t.Fatalf("got %T, want *MissingTitleError", err)
	}
	after := findCard(t, b, "aaaaaa")
	if after.Title != before.Title || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("card mutated despite error: before=%+v after=%+v", before, after)
	}
}

func TestUpdateCard_UnknownPriority(t *testing.T) {
	b := updateTestBoard()
	before := findCard(t, b, "aaaaaa")
	err := UpdateCard(b, "aaaaaa", CardPatch{Priority: ptr("urgent")})
	if err == nil {
		t.Fatalf("UpdateCard returned nil, want *InvalidPriorityError")
	}
	var ipe *InvalidPriorityError
	if !errors.As(err, &ipe) {
		t.Fatalf("got %T, want *InvalidPriorityError", err)
	}
	if ipe.Priority != "urgent" {
		t.Fatalf("InvalidPriorityError.Priority = %q, want %q", ipe.Priority, "urgent")
	}
	after := findCard(t, b, "aaaaaa")
	if after.Priority != before.Priority {
		t.Fatalf("Priority mutated despite error: before=%q after=%q",
			before.Priority, after.Priority)
	}
}

func TestUpdateCard_EmptyTagInList(t *testing.T) {
	b := updateTestBoard()
	before := findCard(t, b, "aaaaaa")
	tags := []string{"good", ""}
	err := UpdateCard(b, "aaaaaa", CardPatch{Tags: &tags})
	if err == nil {
		t.Fatalf("UpdateCard returned nil, want *InvalidTagError")
	}
	var ite *InvalidTagError
	if !errors.As(err, &ite) {
		t.Fatalf("got %T, want *InvalidTagError", err)
	}
	after := findCard(t, b, "aaaaaa")
	if len(after.Tags) != len(before.Tags) {
		t.Fatalf("Tags mutated despite error: before=%v after=%v", before.Tags, after.Tags)
	}
}

func TestUpdateCard_UnknownCard(t *testing.T) {
	b := updateTestBoard()
	beforeIDs := cardIDs(b.Cards)
	err := UpdateCard(b, "zzzzzz", CardPatch{Title: ptr("nope")})
	if err == nil {
		t.Fatalf("UpdateCard returned nil, want *CardNotFoundError")
	}
	var cnf *CardNotFoundError
	if !errors.As(err, &cnf) {
		t.Fatalf("got %T, want *CardNotFoundError", err)
	}
	if cnf.ID != "zzzzzz" {
		t.Fatalf("CardNotFoundError.ID = %q, want %q", cnf.ID, "zzzzzz")
	}
	if got := cardIDs(b.Cards); !reflectStringSliceEqual(got, beforeIDs) {
		t.Fatalf("cards mutated on error: got %v, want %v", got, beforeIDs)
	}
}

func TestUpdateCard_RefreshesUpdatedAt(t *testing.T) {
	b := updateTestBoard()
	before := findCard(t, b, "aaaaaa")
	if err := UpdateCard(b, "aaaaaa", CardPatch{Title: ptr("New title")}); err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}
	after := findCard(t, b, "aaaaaa")
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("UpdatedAt not refreshed: before=%s after=%s",
			before.UpdatedAt, after.UpdatedAt)
	}
}

func TestCardPatch_JSON_AbsentVsEmpty(t *testing.T) {
	// Absent keys → nil pointers.
	var p1 CardPatch
	if err := json.Unmarshal([]byte(`{"title":"hi"}`), &p1); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p1.Title == nil || *p1.Title != "hi" {
		t.Fatalf("Title pointer = %v, want non-nil pointing to %q", p1.Title, "hi")
	}
	if p1.Description != nil {
		t.Fatalf("Description = %v, want nil", p1.Description)
	}
	if p1.Tags != nil {
		t.Fatalf("Tags = %v, want nil", p1.Tags)
	}
	if p1.Priority != nil {
		t.Fatalf("Priority = %v, want nil", p1.Priority)
	}

	// Present empty values → non-nil pointers to the empty value.
	var p2 CardPatch
	if err := json.Unmarshal([]byte(`{"tags":[],"priority":"","description":""}`), &p2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p2.Tags == nil {
		t.Fatalf("Tags pointer = nil, want non-nil pointer to empty slice")
	}
	if len(*p2.Tags) != 0 {
		t.Fatalf("Tags = %v, want empty slice", *p2.Tags)
	}
	if p2.Priority == nil || *p2.Priority != "" {
		t.Fatalf("Priority pointer = %v, want non-nil empty string", p2.Priority)
	}
	if p2.Description == nil || *p2.Description != "" {
		t.Fatalf("Description pointer = %v, want non-nil empty string", p2.Description)
	}
	if p2.Title != nil {
		t.Fatalf("Title = %v, want nil", p2.Title)
	}
}
