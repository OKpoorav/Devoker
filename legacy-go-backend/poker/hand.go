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

// isFlush returns every card of the flush suit (5 to 7), highest first. All of them, not
// just the top 5, because a straight flush can use the lower ones (Ah 9h 8h 7h 6h 5h).
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
		return flush, true
	}
	return nil, false
}

type group struct {
	rank  Rank
	count int
}

// groups lists each rank with how often it appears: biggest count first, then highest rank.
// K K 5 5 5 9 2 -> [{5 3} {K 2} {9 1} {2 1}]
func (h *Hand) groups() []group {
	var gs []group
	for r, n := range h.rankCount {
		gs = append(gs, group{rank: r, count: n})
	}
	slices.SortFunc(gs, func(a, b group) int {
		if a.count != b.count {
			return b.count - a.count
		}
		return int(b.rank - a.rank)
	})
	return gs
}

// countHand classifies the hands decided by rank counts and returns the ranks that
// break ties, most important first. Example: K K 9 9 A -> TwoPair, [K 9 A].
func (h *Hand) countHand() (HandRank, []Rank) {
	gs := h.groups()

	kickers := func(n int, used ...Rank) []Rank {
		var ks []Rank
		for _, g := range gs { // gs has one entry per rank, so no duplicates
			if !slices.Contains(used, g.rank) {
				ks = append(ks, g.rank)
			}
		}
		slices.SortFunc(ks, func(a, b Rank) int { return int(b - a) })
		return ks[:min(n, len(ks))]
	}

	first := gs[0]
	switch {
	case first.count == 4:
		return FourOfAKind, append([]Rank{first.rank}, kickers(1, first.rank)...)
	case first.count == 3 && len(gs) > 1 && gs[1].count >= 2:
		return FullHouse, []Rank{first.rank, gs[1].rank}
	case first.count == 3:
		return ThreeOfAKind, append([]Rank{first.rank}, kickers(2, first.rank)...)
	case first.count == 2 && len(gs) > 1 && gs[1].count == 2:
		return TwoPair, append([]Rank{first.rank, gs[1].rank}, kickers(1, first.rank, gs[1].rank)...)
	case first.count == 2:
		return OnePair, append([]Rank{first.rank}, kickers(3, first.rank)...)
	default:
		return HighCard, kickers(5)
	}
}
