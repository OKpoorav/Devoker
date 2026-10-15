package poker

import (
	"crypto/rand"
	"errors"
	"math/big"
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

// Shuffle uses crypto/rand: with math/rand, someone who works out the generator's state could predict the cards.
func (deck *Deck) Shuffle() {
	for i := len(deck.cards) - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1))) // crypto/rand.Reader never fails since Go 1.24
		deck.cards[i], deck.cards[j.Int64()] = deck.cards[j.Int64()], deck.cards[i]
	}
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
