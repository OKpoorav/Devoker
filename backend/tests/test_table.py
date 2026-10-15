import random

import pytest

from app.poker import WAITING, PokerError, Table, parse


def table(*names: str) -> Table:
    t = Table(start_chips=1000, small_blind=10, big_blind=20)
    for n in names:
        t.add_player(n)
    return t


def stack(t: Table, cards: str) -> None:
    """Deals these cards in order: 2 per player in seat order, then the board."""
    t.deck_factory = lambda: parse(cards)


def check_down(t: Table) -> None:
    while t.street != WAITING:
        t.act(t.players[t.to_act].id, "check")


def chips(t: Table) -> list[int]:
    return [p.chips for p in t.players]


def test_heads_up_fold():
    t = table("ann", "bob")
    t.start_hand()
    assert t.players[t.to_act].id == "p1", "heads-up: the dealer is small blind and acts first"
    with pytest.raises(PokerError, match="not your turn"):
        t.act("p2", "fold")
    t.act("p1", "fold")
    assert t.street == WAITING and chips(t) == [990, 1010]


def test_showdown():
    t = table("ann", "bob", "cat")
    stack(t, "As Ad Ks Kd 7c 2h 9c 5d 3s Jh Qc")  # ann AA, bob KK, cat 72
    t.start_hand()  # dealer p1, sb p2, bb p3; p1 acts first
    t.act("p1", "call")
    t.act("p2", "call")
    t.act("p3", "check")
    check_down(t)
    assert chips(t) == [1040, 980, 980]
    assert t.result["winners"] == [{"playerId": "p1", "amount": 60, "hand": "One Pair"}]


def test_side_pot():
    t = table("ann", "bob", "cat")
    t.players[0].chips = 100  # ann is short and has the best hand
    stack(t, "As Ad Ks Kd 7c 2h 9c 5d 3s Jh Qc")
    t.start_hand()
    t.act("p1", "raise", 100)  # ann all-in
    t.act("p2", "raise", 500)
    t.act("p3", "call")
    check_down(t)
    assert chips(t) == [300, 1300, 500], "main pot 3x100 to ann; side pot 2x400 to bob (KK beats 72)"


def test_random_hands_keep_chips():
    """Plays many random legal hands: chips are never created or lost and every hand finishes."""
    t = table("a", "b", "c", "d", "e")
    rng = random.Random(7)
    for hand in range(500):
        try:
            t.start_hand()
        except PokerError:
            break  # one player has everything
        steps = 0
        while t.street != WAITING:
            steps += 1
            assert steps < 200, "hand never finished"
            p = t.players[t.to_act]
            v = t.view(p.id)
            r = rng.randrange(10)
            if r == 0:
                t.act(p.id, "fold")
            elif r < 3 and v["maxRaiseTo"] > v["currentBet"]:
                t.act(p.id, "raise", rng.randint(v["minRaiseTo"], v["maxRaiseTo"]))
            else:
                t.act(p.id, "call")
        assert sum(chips(t)) == 5000, f"hand {hand}: chips not conserved"


def test_rebuy():
    t = table("ann", "bob")
    with pytest.raises(PokerError, match="still have chips"):
        t.rebuy("p1")
    t.start_hand()
    t.players[0].chips = 0  # ann is all-in in a live hand
    with pytest.raises(PokerError, match="wait for this hand"):
        t.rebuy("p1")
    t.act("p1", "fold")
    t.rebuy("p1")
    assert (t.players[0].chips, t.players[0].buyins) == (1000, 2)
    t.start_hand()
    assert t.players[0].buyins == 2, "starting a hand keeps the buy-in count"


def test_round_trips_through_json_mid_hand():
    t = table("ann", "bob", "cat")
    t.start_hand()
    t.act(t.players[t.to_act].id, "call")
    copy = Table.from_dict(t.to_dict())
    assert copy.to_dict() == t.to_dict()
    assert copy.view("p1") == t.view("p1")


def test_view_hides_other_hole_cards():
    t = table("ann", "bob")
    t.start_hand()
    v = t.view("p1")
    assert len(v["hole"]) == 2
    assert all("hole" not in p for p in v["players"]), "players list never carries hole cards"
