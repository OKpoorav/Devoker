package poker

import (
	"fmt"
	"strings"
	"testing"
)

// cards parses "Ah Td 2c" into cards, to keep tests readable.
func cards(s string) []Card {
	ranks := map[byte]Rank{'A': A, 'K': K, 'Q': Q, 'J': J, 'T': T, '9': NE, '8': ET, '7': SN, '6': SX, '5': FE, '4': FR, '3': TE, '2': TO}
	suits := map[byte]Suit{'c': C, 's': S, 'd': D, 'h': H}
	var cs []Card
	for _, f := range strings.Fields(s) {
		cs = append(cs, Card{Suit: suits[f[1]], Value: ranks[f[0]]})
	}
	return cs
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		hand    string
		rank    HandRank
		kickers string
	}{
		{"Ah Kh Qh Jh Th 2c 3d", RoyalFlush, "[14]"},
		{"9h 8h 7h 6h 5h Ah 2c", StraightFlush, "[9]"}, // 6 hearts: uses the low five, not Ah
		{"5d 4d 3d 2d Ad Kc Qs", StraightFlush, "[5]"}, // steel wheel
		{"9c 9d 9h 9s 2c 2d Ah", FourOfAKind, "[9 14]"},
		{"Kc Kd Kh 5s 5c 5d 2h", FullHouse, "[13 5]"},
		{"Ah Jh 8h 6h 3h 2h Kc", Flush, "[14 11 8 6 3]"},
		{"Ah Kd Qc Js Th 9h 2h", Straight, "[14]"},
		{"Ac 2d 3h 4s 5c 9d Kh", Straight, "[5]"},
		{"7c 7d 7h As 9c 4d 2h", ThreeOfAKind, "[7 14 9]"},
		{"Kc Kd 9h 9s 4c 4d 2h", TwoPair, "[13 9 4]"},
		{"Jc Jd Ah 8s 6c 4d 2h", OnePair, "[11 14 8 6]"},
		{"Ac Qd 9h 7s 5c 3d 2h", HighCard, "[14 12 9 7 5]"},
		{"Qc Kd Ah 2s 3c", HighCard, "[14 13 12 3 2]"}, // straights don't wrap around
	}
	for _, tc := range tests {
		got := Evaluate(cards(tc.hand))
		if got.Rank != tc.rank || fmt.Sprint(got.Kickers) != tc.kickers {
			t.Errorf("%s: got %v %v, want %v %s", tc.hand, got.Rank, got.Kickers, tc.rank, tc.kickers)
		}
	}
}

func TestCompare(t *testing.T) {
	board := "Ah Jh 6h 3h Kc"
	p1 := Evaluate(cards(board + " 8h 2d"))
	p2 := Evaluate(cards(board + " 7h 9s"))
	if p1.Compare(p2) <= 0 {
		t.Errorf("A-J-8 flush should beat A-J-7 flush")
	}
	split := Evaluate(cards("As Ks Qd Jc Th 2c 2d"))
	if split.Compare(Evaluate(cards("Ad Kc Qh Js Tc 3c 3d"))) != 0 {
		t.Errorf("two Ace-high straights should split")
	}
	if Evaluate(cards("2c 2d 3h 4s 7c")).Compare(Evaluate(cards("Ac Kd Qh Js 9c"))) <= 0 {
		t.Errorf("a pair should beat high card")
	}
}

func TestCardString(t *testing.T) {
	if s := fmt.Sprint(cards("As Td 2c")); s != "[As Td 2c]" {
		t.Errorf("got %s", s)
	}
}
