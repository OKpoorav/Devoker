package poker

import (
	"errors"
	"fmt"
	"slices"
)

type Street string

const (
	Waiting Street = "waiting" // between hands
	Preflop Street = "preflop"
	Flop    Street = "flop"
	Turn    Street = "turn"
	River   Street = "river"
)

const MaxPlayers = 9

type Player struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Chips  int    `json:"chips"`
	Bet    int    `json:"bet"` // put in on this street
	InHand bool   `json:"inHand"`
	Folded bool   `json:"folded"`
	AllIn  bool   `json:"allIn"`
	Buyins int    `json:"buyins"`
	Hole   []Card `json:"-"` // private: only sent to its owner, see View
	total  int    // put in this whole hand, for side pots
	acted  bool   // has acted since the last raise
}

type Winner struct {
	PlayerID string `json:"playerId"`
	Amount   int    `json:"amount"`
	Hand     string `json:"hand,omitempty"` // empty when everyone else folded
}

type Result struct {
	Winners []Winner          `json:"winners"`
	Shown   map[string][]Card `json:"shown"` // hole cards revealed at showdown
}

// Table runs no-limit Texas Hold'em. It is not safe for concurrent use; the server locks around it.
type Table struct {
	Players    []*Player
	Board      []Card
	Street     Street
	Dealer     int // index into Players
	ToAct      int // index into Players, -1 when nobody acts
	CurrentBet int
	MinRaise   int // size of the last raise; the next raise must be at least this much
	SmallBlind int
	BigBlind   int
	StartChips int
	Result     *Result // last finished hand

	deck    *Deck
	newDeck func() *Deck // tests swap in a stacked deck
	nextID  int
}

func NewTable(startChips, smallBlind, bigBlind int) *Table {
	return &Table{
		Street: Waiting, Dealer: -1, ToAct: -1,
		StartChips: startChips, SmallBlind: smallBlind, BigBlind: bigBlind,
		newDeck: func() *Deck { d := NewDeck(); d.Shuffle(); return d },
	}
}

// AddPlayer seats a new player. Someone joining mid-hand is dealt in from the next hand.
func (t *Table) AddPlayer(name string) (*Player, error) {
	if len(t.Players) >= MaxPlayers {
		return nil, errors.New("table is full")
	}
	t.nextID++
	p := &Player{ID: fmt.Sprintf("p%d", t.nextID), Name: name, Chips: t.StartChips, Buyins: 1}
	t.Players = append(t.Players, p)
	return p, nil
}

func (t *Table) player(id string) *Player {
	for _, p := range t.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// CanRebuy reports why a player can't buy back in, or nil if they can: they must be out of
// chips and not still part of a hand in progress.
func (t *Table) CanRebuy(playerID string) error {
	p := t.player(playerID)
	switch {
	case p == nil:
		return errors.New("unknown player")
	case p.Chips > 0:
		return errors.New("you still have chips")
	case p.InHand && t.Street != Waiting:
		return errors.New("wait for this hand to finish")
	}
	return nil
}

// Rebuy gives a busted player a fresh starting stack.
func (t *Table) Rebuy(playerID string) error {
	if err := t.CanRebuy(playerID); err != nil {
		return err
	}
	p := t.player(playerID)
	p.Chips = t.StartChips
	p.Buyins++
	return nil
}

func inHand(p *Player) bool { return p.InHand }
func live(p *Player) bool   { return p.InHand && !p.Folded }
func canAct(p *Player) bool { return p.InHand && !p.Folded && !p.AllIn }

// next returns the first seat after from (going round the table) that matches ok, or -1.
func (t *Table) next(from int, ok func(*Player) bool) int {
	n := len(t.Players)
	for k := 1; k <= n; k++ {
		if i := (from + k + n) % n; ok(t.Players[i]) {
			return i
		}
	}
	return -1
}

func (t *Table) StartHand() error {
	if t.Street != Waiting {
		return errors.New("a hand is already in progress")
	}
	seated := 0
	for _, p := range t.Players {
		*p = Player{ID: p.ID, Name: p.Name, Chips: p.Chips, Buyins: p.Buyins, InHand: p.Chips > 0}
		if p.InHand {
			seated++
		}
	}
	if seated < 2 {
		return errors.New("need at least 2 players with chips")
	}

	t.Board, t.Result = nil, nil
	t.deck = t.newDeck()
	t.Dealer = t.next(t.Dealer, inHand)
	sb := t.next(t.Dealer, inHand)
	if seated == 2 {
		sb = t.Dealer // heads-up: the dealer posts the small blind and acts first preflop
	}
	bb := t.next(sb, inHand)

	for _, p := range t.Players {
		if p.InHand {
			p.Hole, _ = t.deck.Deal(2) // 9 players use 23 of 52 cards, so this cannot fail
		}
	}
	t.post(t.Players[sb], t.SmallBlind)
	t.post(t.Players[bb], t.BigBlind)
	t.Street = Preflop
	t.CurrentBet, t.MinRaise = t.BigBlind, t.BigBlind
	t.afterAction(bb)
	return nil
}

// post moves chips from a player into the pot, capped at what they have (all-in).
func (t *Table) post(p *Player, n int) {
	n = min(n, p.Chips)
	p.Chips -= n
	p.Bet += n
	p.total += n
	if p.Chips == 0 {
		p.AllIn = true
	}
}

// Act applies one player action. For "raise", amount is the total bet to raise to on this street.
func (t *Table) Act(playerID, action string, amount int) error {
	if t.Street == Waiting {
		return errors.New("no hand in progress")
	}
	p := t.Players[t.ToAct]
	if p.ID != playerID {
		return errors.New("not your turn")
	}

	switch action {
	case "fold":
		p.Folded = true
	case "check":
		if p.Bet != t.CurrentBet {
			return errors.New("cannot check, you must call or fold")
		}
	case "call":
		t.post(p, t.CurrentBet-p.Bet)
	case "raise":
		maxTo := p.Bet + p.Chips
		switch {
		case amount > maxTo:
			return fmt.Errorf("you can raise to at most %d", maxTo)
		case amount <= t.CurrentBet:
			return fmt.Errorf("raise must be more than %d", t.CurrentBet)
		case amount < t.CurrentBet+t.MinRaise && amount != maxTo: // short all-in is allowed
			return fmt.Errorf("minimum raise is to %d", t.CurrentBet+t.MinRaise)
		}
		if amount-t.CurrentBet >= t.MinRaise {
			t.MinRaise = amount - t.CurrentBet
		}
		t.CurrentBet = amount
		t.post(p, amount-p.Bet)
		for _, o := range t.Players {
			o.acted = false // everyone has to respond to the raise
		}
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	p.acted = true
	t.afterAction(t.ToAct)
	return nil
}

func (t *Table) afterAction(from int) {
	if t.count(live) == 1 {
		t.payout() // everyone else folded
		return
	}
	if t.roundDone() {
		t.nextStreet()
		return
	}
	t.ToAct = t.next(from, canAct)
}

func (t *Table) count(ok func(*Player) bool) int {
	n := 0
	for _, p := range t.Players {
		if ok(p) {
			n++
		}
	}
	return n
}

// roundDone reports whether the betting round is over: everyone who can still bet has
// acted and matched the current bet, or at most one such player is left with nothing to call.
func (t *Table) roundDone() bool {
	var waiting []*Player
	for _, p := range t.Players {
		if canAct(p) {
			waiting = append(waiting, p)
		}
	}
	if len(waiting) == 0 || len(waiting) == 1 && waiting[0].Bet >= t.CurrentBet {
		return true
	}
	for _, p := range waiting {
		if !p.acted || p.Bet != t.CurrentBet {
			return false
		}
	}
	return true
}

func (t *Table) nextStreet() {
	for _, p := range t.Players {
		p.Bet, p.acted = 0, false
	}
	t.CurrentBet, t.MinRaise = 0, t.BigBlind

	switch t.Street {
	case Preflop:
		t.deal(3)
		t.Street = Flop
	case Flop:
		t.deal(1)
		t.Street = Turn
	case Turn:
		t.deal(1)
		t.Street = River
	case River:
		t.payout()
		return
	}
	if t.roundDone() {
		t.nextStreet() // nobody left who can bet: run out the board
		return
	}
	t.ToAct = t.next(t.Dealer, canAct)
}

func (t *Table) deal(n int) {
	cards, _ := t.deck.Deal(n)
	t.Board = append(t.Board, cards...)
}

// payout ends the hand and splits the pot, including side pots when someone was all-in.
func (t *Table) payout() {
	var players []*Player // still in at the end, in seat order
	for _, p := range t.Players {
		if live(p) {
			players = append(players, p)
		}
	}
	res := &Result{Shown: map[string][]Card{}}
	scores := map[*Player]Score{}
	showdown := len(players) > 1
	if showdown {
		for _, p := range players {
			scores[p] = Evaluate(append(slices.Clone(p.Hole), t.Board...))
			res.Shown[p.ID] = p.Hole
		}
	}

	// Every distinct amount a remaining player put in marks the top of a pot. Only players
	// who put in at least that much can win it, so a short all-in can't win chips it never matched.
	var levels []int
	for _, p := range players {
		levels = append(levels, p.total)
	}
	slices.Sort(levels)
	levels = slices.Compact(levels)

	won := map[*Player]int{}
	prev := 0
	for i, lvl := range levels {
		pot := 0
		for _, p := range t.Players {
			pot += min(p.total, lvl) - min(p.total, prev)
			if i == len(levels)-1 {
				pot += max(p.total-lvl, 0) // folded players' chips above the top level
			}
		}
		var best []*Player
		for _, p := range players {
			if p.total < lvl {
				continue
			}
			switch c := scores[p].Compare(scoreOf(best, scores)); {
			case len(best) == 0 || c > 0:
				best = []*Player{p}
			case c == 0:
				best = append(best, p)
			}
		}
		for j, p := range best {
			won[p] += pot / len(best)
			if j == 0 {
				won[p] += pot % len(best) // odd chip
			}
		}
		prev = lvl
	}

	for _, p := range players {
		if won[p] == 0 {
			continue
		}
		p.Chips += won[p]
		w := Winner{PlayerID: p.ID, Amount: won[p]}
		if showdown {
			w.Hand = scores[p].Rank.String()
		}
		res.Winners = append(res.Winners, w)
	}
	for _, p := range t.Players {
		p.Bet = 0
	}
	t.Result = res
	t.Street = Waiting
	t.ToAct = -1
}

func scoreOf(best []*Player, scores map[*Player]Score) Score {
	if len(best) == 0 {
		return Score{}
	}
	return scores[best[0]]
}

func (t *Table) Pot() int {
	pot := 0
	for _, p := range t.Players {
		pot += p.total
	}
	return pot
}

// View is the table as one player is allowed to see it: their own hole cards, nobody else's
// until showdown.
type View struct {
	You        string    `json:"you"`
	Hole       []Card    `json:"hole"`
	Players    []*Player `json:"players"`
	Board      []Card    `json:"board"`
	Street     Street    `json:"street"`
	Dealer     string    `json:"dealer"`
	ToAct      string    `json:"toAct"`
	Pot        int       `json:"pot"`
	CurrentBet int       `json:"currentBet"`
	CallAmount int       `json:"callAmount"`
	MinRaiseTo int       `json:"minRaiseTo"`
	MaxRaiseTo int       `json:"maxRaiseTo"`
	SmallBlind int       `json:"smallBlind"`
	BigBlind   int       `json:"bigBlind"`
	StartChips int       `json:"startChips"`
	Result     *Result   `json:"result"`
}

func (t *Table) View(playerID string) View {
	v := View{
		You: playerID, Players: t.Players, Board: t.Board, Street: t.Street,
		Pot: t.Pot(), CurrentBet: t.CurrentBet, SmallBlind: t.SmallBlind, BigBlind: t.BigBlind,
		StartChips: t.StartChips, Result: t.Result,
	}
	if t.Dealer >= 0 {
		v.Dealer = t.Players[t.Dealer].ID
	}
	if t.ToAct >= 0 {
		v.ToAct = t.Players[t.ToAct].ID
	}
	for _, p := range t.Players {
		if p.ID == playerID {
			v.Hole = p.Hole
			v.CallAmount = min(t.CurrentBet-p.Bet, p.Chips)
			v.MaxRaiseTo = p.Bet + p.Chips
			v.MinRaiseTo = min(t.CurrentBet+t.MinRaise, v.MaxRaiseTo)
		}
	}
	return v
}
