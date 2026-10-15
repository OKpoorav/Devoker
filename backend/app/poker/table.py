from collections.abc import Callable
from dataclasses import dataclass, field

from .cards import Card, new_deck
from .evaluate import Score, evaluate

WAITING, PREFLOP, FLOP, TURN, RIVER = "waiting", "preflop", "flop", "turn", "river"
MAX_PLAYERS = 9


class PokerError(ValueError):
    """An action the rules don't allow; the message is shown to the player."""


@dataclass
class Player:
    id: str
    name: str
    chips: int
    buyins: int = 1
    bet: int = 0  # put in on this street
    in_hand: bool = False
    folded: bool = False
    all_in: bool = False
    hole: list[Card] = field(default_factory=list)  # private: only shown to its owner, see view()
    total: int = 0  # put in this whole hand, for side pots
    acted: bool = False  # has acted since the last raise

    def public(self) -> dict:
        return {"id": self.id, "name": self.name, "chips": self.chips, "bet": self.bet, "inHand": self.in_hand,
                "folded": self.folded, "allIn": self.all_in, "buyins": self.buyins}


def live(p: Player) -> bool:
    return p.in_hand and not p.folded


def can_act(p: Player) -> bool:
    return p.in_hand and not p.folded and not p.all_in


@dataclass
class Table:
    """No-limit Texas Hold'em. Not safe for concurrent use: the room store locks the row around it."""

    start_chips: int
    small_blind: int
    big_blind: int
    players: list[Player] = field(default_factory=list)
    board: list[Card] = field(default_factory=list)
    street: str = WAITING
    dealer: int = -1  # index into players
    to_act: int = -1  # index into players, -1 when nobody acts
    current_bet: int = 0
    min_raise: int = 0  # size of the last raise; the next raise must be at least this much
    result: dict | None = None  # last finished hand
    deck: list[Card] = field(default_factory=list)
    next_id: int = 0
    deck_factory: Callable[[], list[Card]] = field(default=new_deck, repr=False, compare=False)  # tests stack it

    def add_player(self, name: str) -> Player:
        """Seats a new player. Someone joining mid-hand is dealt in from the next hand."""
        if len(self.players) >= MAX_PLAYERS:
            raise PokerError("table is full")
        self.next_id += 1
        p = Player(id=f"p{self.next_id}", name=name, chips=self.start_chips)
        self.players.append(p)
        return p

    def player(self, pid: str) -> Player | None:
        return next((p for p in self.players if p.id == pid), None)

    def check_rebuy(self, pid: str) -> None:
        """Raises unless the player is out of chips and not still part of a hand in progress."""
        p = self.player(pid)
        if p is None:
            raise PokerError("unknown player")
        if p.chips > 0:
            raise PokerError("you still have chips")
        if p.in_hand and self.street != WAITING:
            raise PokerError("wait for this hand to finish")

    def rebuy(self, pid: str) -> None:
        self.check_rebuy(pid)
        p = self.player(pid)
        p.chips = self.start_chips
        p.buyins += 1

    def remove_player(self, pid: str) -> None:
        """Takes a player off the table between hands, keeping the dealer button in place."""
        if self.street != WAITING:
            raise PokerError("wait for this hand to finish")
        i = next(i for i, p in enumerate(self.players) if p.id == pid)
        del self.players[i]
        if i <= self.dealer:
            self.dealer -= 1  # the button moves on from the seat before, as if the leaver had dealt

    def _next(self, start: int, ok: Callable[[Player], bool]) -> int:
        """The first seat after start (going round the table) that matches ok, or -1."""
        n = len(self.players)
        for k in range(1, n + 1):
            i = (start + k + n) % n
            if ok(self.players[i]):
                return i
        return -1

    def start_hand(self) -> None:
        if self.street != WAITING:
            raise PokerError("a hand is already in progress")
        self.players = [Player(p.id, p.name, p.chips, p.buyins, in_hand=p.chips > 0) for p in self.players]
        seated = sum(p.in_hand for p in self.players)
        if seated < 2:
            raise PokerError("need at least 2 players with chips")

        self.board, self.result = [], None
        self.deck = self.deck_factory()
        in_hand = lambda p: p.in_hand  # noqa: E731
        self.dealer = self._next(self.dealer, in_hand)
        # Heads-up, the dealer posts the small blind and acts first preflop.
        sb = self.dealer if seated == 2 else self._next(self.dealer, in_hand)
        bb = self._next(sb, in_hand)

        for p in self.players:
            if p.in_hand:
                p.hole = self._deal(2)  # 9 players use 23 of 52 cards, so this cannot run out
        self._post(self.players[sb], self.small_blind)
        self._post(self.players[bb], self.big_blind)
        self.street = PREFLOP
        self.current_bet = self.min_raise = self.big_blind
        self._after_action(bb)

    def _deal(self, n: int) -> list[Card]:
        cards, self.deck = self.deck[:n], self.deck[n:]
        return cards

    @staticmethod
    def _post(p: Player, n: int) -> None:
        """Moves chips from a player into the pot, capped at what they have (all-in)."""
        n = min(n, p.chips)
        p.chips -= n
        p.bet += n
        p.total += n
        if p.chips == 0:
            p.all_in = True

    def act(self, pid: str, action: str, amount: int = 0) -> None:
        """Applies one player action. For "raise", amount is the total bet to raise to on this street."""
        if self.street == WAITING:
            raise PokerError("no hand in progress")
        p = self.players[self.to_act]
        if p.id != pid:
            raise PokerError("not your turn")

        if action == "fold":
            p.folded = True
        elif action == "check":
            if p.bet != self.current_bet:
                raise PokerError("cannot check, you must call or fold")
        elif action == "call":
            self._post(p, self.current_bet - p.bet)
        elif action == "raise":
            max_to = p.bet + p.chips
            if amount > max_to:
                raise PokerError(f"you can raise to at most {max_to}")
            if amount <= self.current_bet:
                raise PokerError(f"raise must be more than {self.current_bet}")
            if amount < self.current_bet + self.min_raise and amount != max_to:  # a short all-in is allowed
                raise PokerError(f"minimum raise is to {self.current_bet + self.min_raise}")
            if amount - self.current_bet >= self.min_raise:
                self.min_raise = amount - self.current_bet
            self.current_bet = amount
            self._post(p, amount - p.bet)
            for o in self.players:
                o.acted = False  # everyone has to respond to the raise
        else:
            raise PokerError(f"unknown action {action!r}")
        p.acted = True
        self._after_action(self.to_act)

    def _after_action(self, start: int) -> None:
        if sum(live(p) for p in self.players) == 1:
            self._payout()  # everyone else folded
        elif self._round_done():
            self._next_street()
        else:
            self.to_act = self._next(start, can_act)

    def _round_done(self) -> bool:
        """Everyone who can still bet has acted and matched the current bet, or at most one such
        player is left with nothing to call."""
        waiting = [p for p in self.players if can_act(p)]
        if not waiting or (len(waiting) == 1 and waiting[0].bet >= self.current_bet):
            return True
        return all(p.acted and p.bet == self.current_bet for p in waiting)

    def _next_street(self) -> None:
        for p in self.players:
            p.bet, p.acted = 0, False
        self.current_bet, self.min_raise = 0, self.big_blind

        if self.street == RIVER:
            self._payout()
            return
        self.board += self._deal(3 if self.street == PREFLOP else 1)
        self.street = {PREFLOP: FLOP, FLOP: TURN, TURN: RIVER}[self.street]
        if self._round_done():
            self._next_street()  # nobody left who can bet: run out the board
        else:
            self.to_act = self._next(self.dealer, can_act)

    def _payout(self) -> None:
        """Ends the hand and splits the pot, including side pots when someone was all-in."""
        players = [p for p in self.players if live(p)]  # still in at the end, in seat order
        showdown = len(players) > 1
        scores: dict[str, Score] = {p.id: evaluate(p.hole + self.board) for p in players} if showdown else {}

        # Every distinct amount a remaining player put in marks the top of a pot. Only players who
        # put in at least that much can win it, so a short all-in can't win chips it never matched.
        levels = sorted({p.total for p in players})
        won: dict[str, int] = {}
        prev = 0
        for i, level in enumerate(levels):
            pot = sum(min(p.total, level) - min(p.total, prev) for p in self.players)
            if i == len(levels) - 1:
                pot += sum(max(p.total - level, 0) for p in self.players)  # folded players' chips above the top level
            eligible = [p for p in players if p.total >= level]
            best_score = max((scores.get(p.id, Score(0, ())) for p in eligible), default=None)
            best = [p for p in eligible if scores.get(p.id, Score(0, ())) == best_score]
            for j, p in enumerate(best):
                won[p.id] = won.get(p.id, 0) + pot // len(best) + (pot % len(best) if j == 0 else 0)  # odd chip
            prev = level

        winners = []
        for p in players:
            if won.get(p.id):
                p.chips += won[p.id]
                w = {"playerId": p.id, "amount": won[p.id]}
                if showdown:
                    w["hand"] = scores[p.id].name
                winners.append(w)
        for p in self.players:
            p.bet = 0
        self.result = {"winners": winners, "shown": {p.id: [c.to_json() for c in p.hole] for p in players} if showdown else {}}
        self.street = WAITING
        self.to_act = -1

    def pot(self) -> int:
        return sum(p.total for p in self.players)

    def view(self, pid: str) -> dict:
        """The table as one player may see it: their own hole cards, nobody else's until showdown."""
        me = self.player(pid)
        v = {
            "you": pid, "hole": None, "players": [p.public() for p in self.players],
            "board": [c.to_json() for c in self.board], "street": self.street,
            "dealer": self.players[self.dealer].id if 0 <= self.dealer < len(self.players) else "",
            "toAct": self.players[self.to_act].id if self.to_act >= 0 else "",
            "pot": self.pot(), "currentBet": self.current_bet, "callAmount": 0, "minRaiseTo": 0, "maxRaiseTo": 0,
            "smallBlind": self.small_blind, "bigBlind": self.big_blind, "startChips": self.start_chips,
            "result": self.result,
        }
        if me:
            v["hole"] = [c.to_json() for c in me.hole]
            v["callAmount"] = min(self.current_bet - me.bet, me.chips)
            v["maxRaiseTo"] = me.bet + me.chips
            v["minRaiseTo"] = min(self.current_bet + self.min_raise, v["maxRaiseTo"])
        return v

    def to_dict(self) -> dict:
        """Everything, including the deck and every hole card: this is stored, never sent to players."""
        d = {k: getattr(self, k) for k in ("start_chips", "small_blind", "big_blind", "street", "dealer", "to_act",
                                           "current_bet", "min_raise", "result", "next_id")}
        d["board"] = [c.to_json() for c in self.board]
        d["deck"] = [c.to_json() for c in self.deck]
        d["players"] = [{**vars(p), "hole": [c.to_json() for c in p.hole]} for p in self.players]
        return d

    @staticmethod
    def from_dict(d: dict) -> "Table":
        cards = lambda xs: [Card.from_json(x) for x in xs]  # noqa: E731
        t = Table(**{k: v for k, v in d.items() if k not in ("board", "deck", "players")})
        t.board, t.deck = cards(d["board"]), cards(d["deck"])
        t.players = [Player(**{**p, "hole": cards(p["hole"])}) for p in d["players"]]
        return t
