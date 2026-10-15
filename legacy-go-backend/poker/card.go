package poker

type Suit string
type Rank int

// Rank names: face cards use their letter (T = ten, standard poker notation),
// number cards use the first and last letter of the word (NE = nine, TO = two).
const (
	A  Rank = 14
	K  Rank = 13
	Q  Rank = 12
	J  Rank = 11
	T  Rank = 10
	NE Rank = 9
	ET Rank = 8
	SN Rank = 7
	SX Rank = 6
	FE Rank = 5
	FR Rank = 4
	TE Rank = 3
	TO Rank = 2
)

const (
	C Suit = "Clubs"
	S Suit = "Spades"
	D Suit = "Diamonds"
	H Suit = "Hearts"
)

type Card struct {
	Suit  Suit
	Value Rank
}

var rankChars = map[Rank]string{
	A: "A", K: "K", Q: "Q", J: "J", T: "T", NE: "9", ET: "8",
	SN: "7", SX: "6", FE: "5", FR: "4", TE: "3", TO: "2",
}

var suitChars = map[Suit]string{C: "c", S: "s", D: "d", H: "h"}

// String returns short poker notation, e.g. "As" for the Ace of Spades.
func (c Card) String() string {
	return rankChars[c.Value] + suitChars[c.Suit]
}
