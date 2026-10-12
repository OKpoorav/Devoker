package poker

import "testing"

func TestNewDeck(t *testing.T) {
	d := NewDeck()

	if len(d.cards) != 52 {
		t.Fatalf("expected 52 cards, got %d", len(d.cards))
	}
}
