package board

// DeleteCard removes the card with the given id from b.Cards in place.
// It returns *CardNotFoundError if no card has the given id, and does
// NOT mutate b.Cards on the failure path. On success the surviving
// cards retain their relative order (slice-splice). DeleteCard does
// NOT persist; callers run board.Save to commit the change.
//
// Children of the deleted card are orphaned, not refused — see
// DeleteCardOrphaning for the reasoning. Callers that want to report
// the affected ids should use that function directly.
func DeleteCard(b *Board, id string) error {
	_, err := DeleteCardOrphaning(b, id)
	return err
}

// DeleteCardOrphaning deletes the card with the given id and clears the
// Epic field of every card that referenced it, returning those ids in
// board file order.
//
// Deleting a parent orphans rather than refuses: `epic` is optional, so
// clearing it leaves a perfectly valid card, and refusing would force
// the user to unlink every child by hand for no integrity benefit
// (design D6). Orphaning does NOT refresh the children's UpdatedAt —
// losing a parent is a board-level consequence, not an edit to the
// child, on the same reasoning RenameColumn uses.
func DeleteCardOrphaning(b *Board, id string) ([]string, error) {
	idx := -1
	for i, c := range b.Cards {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, &CardNotFoundError{ID: id}
	}
	b.Cards = append(b.Cards[:idx], b.Cards[idx+1:]...)

	orphaned := []string{}
	for i := range b.Cards {
		if b.Cards[i].Epic == id {
			b.Cards[i].Epic = ""
			orphaned = append(orphaned, b.Cards[i].ID)
		}
	}
	return orphaned, nil
}
