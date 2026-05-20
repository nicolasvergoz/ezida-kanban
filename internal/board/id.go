package board

import (
	"crypto/rand"
	"errors"
)

const idAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
const idLen = 6
const maxIDAttempts = 10

// ErrIDExhausted is returned by NewUniqueID when the retry budget is exhausted
// without producing an ID that is absent from the provided set.
var ErrIDExhausted = errors.New("board: id generation exhausted after 10 attempts")

// NewID returns a freshly generated 6-character ID drawn from [0-9a-z].
// It uses crypto/rand for entropy.
func NewID() string {
	var buf [idLen]byte
	_, _ = rand.Read(buf[:])
	out := make([]byte, idLen)
	for i, b := range buf {
		out[i] = idAlphabet[int(b)%len(idAlphabet)]
	}
	return string(out)
}

// NewUniqueID returns a NewID that does not appear in existing.
// It retries up to 10 times before returning ErrIDExhausted.
func NewUniqueID(existing []string) (string, error) {
	return newUniqueIDWith(NewID, existing)
}

// newUniqueIDWith is the testable form that accepts an injected generator.
func newUniqueIDWith(gen func() string, existing []string) (string, error) {
	set := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		set[id] = struct{}{}
	}
	for i := 0; i < maxIDAttempts; i++ {
		candidate := gen()
		if _, found := set[candidate]; !found {
			return candidate, nil
		}
	}
	return "", ErrIDExhausted
}
