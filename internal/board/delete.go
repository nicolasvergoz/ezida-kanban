package board

// DeleteCard removes the card with the given id from b.Cards in place.
// It returns *CardNotFoundError if no card has the given id, and does
// NOT mutate b.Cards on the failure path. On success the surviving
// cards retain their relative order (slice-splice). DeleteCard does
// NOT persist; callers run board.Save to commit the change.
func DeleteCard(b *Board, id string) error {
	idx := -1
	for i, c := range b.Cards {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return &CardNotFoundError{ID: id}
	}
	b.Cards = append(b.Cards[:idx], b.Cards[idx+1:]...)
	return nil
}
