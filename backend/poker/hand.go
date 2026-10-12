package poker

type Hand struct {
	cards     []Card
	rankCount map[Rank]int
	suitCount map[Suit]int
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

func isStraigth(cards []Card) {

}
