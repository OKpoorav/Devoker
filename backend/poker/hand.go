package poker

import "slices"

type Hand struct {
	cards     []Card
	rankCount map[Rank]int
	suitCount map[Suit]int
	handRank  HandRank
}

type HandRank int

// Weakest first, so a bigger HandRank is a better hand. Starts at 1 so 0 means "not evaluated".
const (
	HighCard HandRank = iota + 1
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
	RoyalFlush
)

func NewHand(cards []Card) *Hand {
	h := Hand{
		cards: cards,
	}
	h.countRanks()
	h.countSuits()
	return &h
}

func (h *Hand) countRanks() {
	res := make(map[Rank]int)
	for _, c := range h.cards {
		res[c.Value]++
	}
	h.rankCount = res
}

func (h *Hand) countSuits() {
	res := make(map[Suit]int)
	for _, c := range h.cards {
		res[c.Suit]++
	}
	h.suitCount = res
}

func (h *Hand) isStraight() (Rank, bool) {
	const window = 5
	for top := A; top >= FE; top-- {
		found := true
		for r := top; r > top-window; r-- {
			check := r
			if r == 1 {
				check = A
			}
			if h.rankCount[check] == 0 {
				found = false
				break
			}
		}
		if found {
			return top, true
		}
	}
	return 0, false
}

func (h *Hand) isFlush() ([]Card, bool) {
	for suit, count := range h.suitCount {
		if count < 5 {
			continue
		}
		var flush []Card
		for _, c := range h.cards {
			if c.Suit == suit {
				flush = append(flush, c)
			}
		}
		slices.SortFunc(flush, func(a, b Card) int { return int(b.Value - a.Value) })
		return flush[:5], true
	}
	return nil, false
}
