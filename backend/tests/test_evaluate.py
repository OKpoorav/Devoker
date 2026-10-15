import pytest

from app.poker import evaluate, parse
from app.poker.cards import new_deck
from app.poker.evaluate import (FLUSH, FOUR_OF_A_KIND, FULL_HOUSE, HIGH_CARD, ONE_PAIR, ROYAL_FLUSH, STRAIGHT,
                                STRAIGHT_FLUSH, THREE_OF_A_KIND, TWO_PAIR)


@pytest.mark.parametrize("hand, rank, kickers", [
    ("Ah Kh Qh Jh Th 2c 3d", ROYAL_FLUSH, (14,)),
    ("9h 8h 7h 6h 5h Ah 2c", STRAIGHT_FLUSH, (9,)),  # 6 hearts: uses the low five, not Ah
    ("5d 4d 3d 2d Ad Kc Qs", STRAIGHT_FLUSH, (5,)),  # steel wheel
    ("9c 9d 9h 9s 2c 2d Ah", FOUR_OF_A_KIND, (9, 14)),
    ("Kc Kd Kh 5s 5c 5d 2h", FULL_HOUSE, (13, 5)),
    ("Ah Jh 8h 6h 3h 2h Kc", FLUSH, (14, 11, 8, 6, 3)),
    ("Ah Kd Qc Js Th 9h 2h", STRAIGHT, (14,)),
    ("Ac 2d 3h 4s 5c 9d Kh", STRAIGHT, (5,)),
    ("7c 7d 7h As 9c 4d 2h", THREE_OF_A_KIND, (7, 14, 9)),
    ("Kc Kd 9h 9s 4c 4d 2h", TWO_PAIR, (13, 9, 4)),
    ("Jc Jd Ah 8s 6c 4d 2h", ONE_PAIR, (11, 14, 8, 6)),
    ("Ac Qd 9h 7s 5c 3d 2h", HIGH_CARD, (14, 12, 9, 7, 5)),
    ("Qc Kd Ah 2s 3c", HIGH_CARD, (14, 13, 12, 3, 2)),  # straights don't wrap around
])
def test_evaluate(hand, rank, kickers):
    assert evaluate(parse(hand)) == (rank, kickers)


def test_compare():
    board = "Ah Jh 6h 3h Kc"
    assert evaluate(parse(board + " 8h 2d")) > evaluate(parse(board + " 7h 9s")), "A-J-8 flush beats A-J-7 flush"
    assert evaluate(parse("As Ks Qd Jc Th 2c 2d")) == evaluate(parse("Ad Kc Qh Js Tc 3c 3d")), "two A-high straights split"
    assert evaluate(parse("2c 2d 3h 4s 7c")) > evaluate(parse("Ac Kd Qh Js 9c")), "a pair beats high card"


def test_card_string():
    assert [str(c) for c in parse("As Td 2c")] == ["As", "Td", "2c"]


def test_shuffled_deck_has_every_card_once():
    deck = new_deck()
    assert len(deck) == 52 and len(set(deck)) == 52
    assert deck != sorted(deck, key=lambda c: (c.suit, -c.value)), "shuffled, not factory order"
