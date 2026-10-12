package poker

import (
	"errors"
	"math/rand"
)

type Deck struct {
	cards []Card
}

func NewDeck() *Deck {
	deck := &Deck{
		cards: make([]Card, 0, 52),
	}

	suits := []Suit{C, S, D, H}
	ranks := []Rank{A, K, Q, J, T, NE, ET, SN, SX, FE, FR, TE, TO}

	for _, suit := range suits {
		for _, rank := range ranks {
			deck.cards = append(deck.cards, Card{
				Suit:  suit,
				Value: rank,
			})
		}
	}

	return deck
}
func (deck *Deck) Shuffle() {
	rand.Shuffle(len(deck.cards), func(i, j int) {
		deck.cards[i], deck.cards[j] = deck.cards[j], deck.cards[i]
	})
}

func (deck *Deck) Deal(n int) ([]Card, error) {

	if n < 0 {
		return nil, errors.New("Can not deal negative cards")

	}
	if len(deck.cards) < n {
		return nil, errors.New("Not Enough Cards")

	}

	dealed := deck.cards[:n:n]
	deck.cards = deck.cards[n:]
	return dealed, nil
}

func (deck *Deck) Len() (n int) {
	return len(deck.cards)
}
