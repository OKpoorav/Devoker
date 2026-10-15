import secrets
from dataclasses import dataclass

SUITS = ("Clubs", "Spades", "Diamonds", "Hearts")
RANKS = range(14, 1, -1)  # Ace (14) high down to two

RANK_CHARS = {14: "A", 13: "K", 12: "Q", 11: "J", 10: "T", 9: "9", 8: "8", 7: "7", 6: "6", 5: "5", 4: "4", 3: "3", 2: "2"}
SUIT_CHARS = {"Clubs": "c", "Spades": "s", "Diamonds": "d", "Hearts": "h"}


@dataclass(frozen=True)
class Card:
    suit: str
    value: int

    def __str__(self) -> str:
        return RANK_CHARS[self.value] + SUIT_CHARS[self.suit]

    def to_json(self) -> dict:
        return {"Suit": self.suit, "Value": self.value}

    @staticmethod
    def from_json(d: dict) -> "Card":
        return Card(d["Suit"], d["Value"])


def parse(text: str) -> list[Card]:
    """Parses "Ah Td 2c" into cards, for readable tests and stacked decks."""
    ranks = {c: r for r, c in RANK_CHARS.items()}
    suits = {c: s for s, c in SUIT_CHARS.items()}
    return [Card(suits[f[1]], ranks[f[0]]) for f in text.split()]


def new_deck() -> list[Card]:
    deck = [Card(s, r) for s in SUITS for r in RANKS]
    # SystemRandom draws from the OS's cryptographic source: with a seeded PRNG, someone who
    # works out the generator's state could predict the cards.
    secrets.SystemRandom().shuffle(deck)
    return deck
