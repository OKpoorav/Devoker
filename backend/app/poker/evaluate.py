from collections import Counter
from typing import NamedTuple

from .cards import Card

HIGH_CARD, ONE_PAIR, TWO_PAIR, THREE_OF_A_KIND, STRAIGHT, FLUSH, FULL_HOUSE, FOUR_OF_A_KIND, STRAIGHT_FLUSH, ROYAL_FLUSH = range(1, 11)

HAND_NAMES = {
    HIGH_CARD: "High Card", ONE_PAIR: "One Pair", TWO_PAIR: "Two Pair", THREE_OF_A_KIND: "Three of a Kind",
    STRAIGHT: "Straight", FLUSH: "Flush", FULL_HOUSE: "Full House", FOUR_OF_A_KIND: "Four of a Kind",
    STRAIGHT_FLUSH: "Straight Flush", ROYAL_FLUSH: "Royal Flush",
}


class Score(NamedTuple):
    """A fully evaluated hand. Tuples compare rank first, then kickers in order, which is poker's rule."""

    rank: int
    kickers: tuple[int, ...]

    @property
    def name(self) -> str:
        return HAND_NAMES[self.rank]


def straight_top(ranks: set[int]) -> int | None:
    """Highest card of a 5-card run in ranks, or None. The ace also counts low (A-2-3-4-5)."""
    for top in range(14, 4, -1):
        if all((14 if r == 1 else r) in ranks for r in range(top, top - 5, -1)):
            return top
    return None


def evaluate(cards: list[Card]) -> Score:
    """Finds the best 5-card poker hand in up to 7 cards."""
    by_suit = Counter(c.suit for c in cards)
    suit, count = by_suit.most_common(1)[0]
    # All the flush suit's cards, not just the top 5: a straight flush can use lower ones (Ah 9h 8h 7h 6h 5h).
    flush = sorted((c.value for c in cards if c.suit == suit), reverse=True) if count >= 5 else []

    if flush and (top := straight_top(set(flush))):
        return Score(ROYAL_FLUSH, (14,)) if top == 14 else Score(STRAIGHT_FLUSH, (top,))

    rank, kickers = count_hand(cards)
    if rank in (FOUR_OF_A_KIND, FULL_HOUSE):
        return Score(rank, kickers)
    if flush:
        return Score(FLUSH, tuple(flush[:5]))
    if top := straight_top({c.value for c in cards}):
        return Score(STRAIGHT, (top,))
    return Score(rank, kickers)


def count_hand(cards: list[Card]) -> tuple[int, tuple[int, ...]]:
    """Classifies the hands decided by rank counts and returns the tie-breaking ranks, most
    important first. K K 9 9 A -> (TWO_PAIR, (13, 9, 14))."""
    # Each rank with how often it appears: biggest count first, then highest rank.
    groups = sorted(Counter(c.value for c in cards).items(), key=lambda g: (g[1], g[0]), reverse=True)

    def kickers(n: int, *used: int) -> tuple[int, ...]:
        return tuple(sorted((r for r, _ in groups if r not in used), reverse=True)[:n])

    (first, n1), second = groups[0], groups[1] if len(groups) > 1 else (0, 0)
    if n1 == 4:
        return FOUR_OF_A_KIND, (first, *kickers(1, first))
    if n1 == 3 and second[1] >= 2:
        return FULL_HOUSE, (first, second[0])
    if n1 == 3:
        return THREE_OF_A_KIND, (first, *kickers(2, first))
    if n1 == 2 and second[1] == 2:
        return TWO_PAIR, (first, second[0], *kickers(1, first, second[0]))
    if n1 == 2:
        return ONE_PAIR, (first, *kickers(3, first))
    return HIGH_CARD, kickers(5)
