package poker

import "slices"

var handNames = map[HandRank]string{
	HighCard: "High Card", OnePair: "One Pair", TwoPair: "Two Pair",
	ThreeOfAKind: "Three of a Kind", Straight: "Straight", Flush: "Flush",
	FullHouse: "Full House", FourOfAKind: "Four of a Kind",
	StraightFlush: "Straight Flush", RoyalFlush: "Royal Flush",
}

func (r HandRank) String() string { return handNames[r] }

// Score is a fully evaluated hand. Two scores are compared by Rank first, then Kickers in order.
type Score struct {
	Rank    HandRank
	Kickers []Rank
}

// Evaluate finds the best 5-card poker hand in up to 7 cards.
func Evaluate(cards []Card) Score {
	h := NewHand(cards)

	flush, isFlush := h.isFlush()
	if isFlush {
		if top, ok := NewHand(flush).isStraight(); ok {
			if top == A {
				return Score{RoyalFlush, []Rank{A}}
			}
			return Score{StraightFlush, []Rank{top}}
		}
	}

	rank, kickers := h.countHand()
	if rank == FourOfAKind || rank == FullHouse {
		return Score{rank, kickers}
	}
	if isFlush {
		ranks := make([]Rank, 5)
		for i, c := range flush[:5] {
			ranks[i] = c.Value
		}
		return Score{Flush, ranks}
	}
	if top, ok := h.isStraight(); ok {
		return Score{Straight, []Rank{top}}
	}
	return Score{rank, kickers}
}

// Compare returns >0 if s beats o, <0 if o wins, 0 for a split pot.
func (s Score) Compare(o Score) int {
	if s.Rank != o.Rank {
		return int(s.Rank - o.Rank)
	}
	return slices.Compare(s.Kickers, o.Kickers)
}
