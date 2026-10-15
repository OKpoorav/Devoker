package poker

import (
	"slices"
	"testing"
)

func TestNewDeck(t *testing.T) {
	d := NewDeck()

	if len(d.cards) != 52 {
		t.Fatalf("expected 52 cards, got %d", len(d.cards))
	}
}

func TestShuffleKeepsEveryCardOnce(t *testing.T) {
	d := NewDeck()
	d.Shuffle()
	seen := map[Card]bool{}
	for _, c := range d.cards {
		seen[c] = true
	}
	if len(d.cards) != 52 || len(seen) != 52 {
		t.Fatalf("got %d cards, %d distinct; want 52 and 52", len(d.cards), len(seen))
	}
	if fresh := NewDeck(); slices.Equal(d.cards, fresh.cards) {
		t.Error("shuffled deck is still in factory order")
	}
}
