package poker

import (
	"math/rand/v2"
	"testing"
)

// stacked makes the table deal these cards in order: 2 per player in seat order, then the board.
func stacked(t *Table, s string) {
	t.newDeck = func() *Deck { return &Deck{cards: cards(s)} }
}

func newTestTable(names ...string) *Table {
	t := NewTable(1000, 10, 20)
	for _, n := range names {
		t.AddPlayer(n)
	}
	return t
}

func mustAct(t *testing.T, tb *Table, id, action string, amount int) {
	t.Helper()
	if err := tb.Act(id, action, amount); err != nil {
		t.Fatalf("%s %s %d: %v", id, action, amount, err)
	}
}

func TestHeadsUpFold(t *testing.T) {
	tb := newTestTable("ann", "bob")
	if err := tb.StartHand(); err != nil {
		t.Fatal(err)
	}
	// heads-up: dealer (p1) is small blind and acts first
	if tb.Players[tb.ToAct].ID != "p1" {
		t.Fatalf("p1 should act first, got %s", tb.Players[tb.ToAct].ID)
	}
	if err := tb.Act("p2", "fold", 0); err == nil {
		t.Error("acting out of turn should fail")
	}
	mustAct(t, tb, "p1", "fold", 0)
	if tb.Street != Waiting || tb.Players[0].Chips != 990 || tb.Players[1].Chips != 1010 {
		t.Errorf("got street %s chips %d/%d", tb.Street, tb.Players[0].Chips, tb.Players[1].Chips)
	}
}

func TestShowdown(t *testing.T) {
	tb := newTestTable("ann", "bob", "cat")
	// ann: AA, bob: KK, cat: 72; board gives nobody anything better than ann's aces
	stacked(tb, "As Ad Ks Kd 7c 2h 9c 5d 3s Jh Qc")
	tb.StartHand() // dealer p1, sb p2, bb p3, p1 acts first
	mustAct(t, tb, "p1", "call", 0)
	mustAct(t, tb, "p2", "call", 0)
	mustAct(t, tb, "p3", "check", 0)
	for tb.Street != Waiting { // check it down
		mustAct(t, tb, tb.Players[tb.ToAct].ID, "check", 0)
	}
	if got := []int{tb.Players[0].Chips, tb.Players[1].Chips, tb.Players[2].Chips}; got[0] != 1040 || got[1] != 980 || got[2] != 980 {
		t.Errorf("chips = %v, want [1040 980 980]", got)
	}
	if w := tb.Result.Winners; len(w) != 1 || w[0].PlayerID != "p1" || w[0].Hand != "One Pair" {
		t.Errorf("winners = %+v", w)
	}
}

func TestSidePot(t *testing.T) {
	tb := newTestTable("ann", "bob", "cat")
	tb.Players[0].Chips = 100 // ann is short and has the best hand
	stacked(tb, "As Ad Ks Kd 7c 2h 9c 5d 3s Jh Qc")
	tb.StartHand()
	mustAct(t, tb, "p1", "raise", 100) // ann all-in
	mustAct(t, tb, "p2", "raise", 500)
	mustAct(t, tb, "p3", "call", 0)
	for tb.Street != Waiting {
		mustAct(t, tb, tb.Players[tb.ToAct].ID, "check", 0)
	}
	// main pot 3x100 = 300 to ann; side pot 2x400 = 800 to bob (KK beats 72)
	if got := []int{tb.Players[0].Chips, tb.Players[1].Chips, tb.Players[2].Chips}; got[0] != 300 || got[1] != 1300 || got[2] != 500 {
		t.Errorf("chips = %v, want [300 1300 500]", got)
	}
}

// TestRandomHandsKeepChips plays thousands of random legal hands and checks that chips are
// never created or lost and that every hand finishes.
func TestRandomHandsKeepChips(t *testing.T) {
	tb := newTestTable("a", "b", "c", "d", "e")
	const total = 5000
	for hand := 0; hand < 2000; hand++ {
		if tb.StartHand() != nil {
			break // one player has everything
		}
		for steps := 0; tb.Street != Waiting; steps++ {
			if steps > 200 {
				t.Fatal("hand never finished")
			}
			p := tb.Players[tb.ToAct]
			v := tb.View(p.ID)
			switch r := rand.IntN(10); {
			case r == 0:
				mustAct(t, tb, p.ID, "fold", 0)
			case r < 3 && v.MaxRaiseTo > v.CurrentBet:
				mustAct(t, tb, p.ID, "raise", v.MinRaiseTo+rand.IntN(v.MaxRaiseTo-v.MinRaiseTo+1))
			default:
				mustAct(t, tb, p.ID, "call", 0)
			}
		}
		sum := 0
		for _, p := range tb.Players {
			sum += p.Chips
		}
		if sum != total {
			t.Fatalf("hand %d: chips total %d, want %d", hand, sum, total)
		}
	}
}

func TestRebuy(t *testing.T) {
	tb := newTestTable("ann", "bob")
	if err := tb.Rebuy("p1"); err == nil {
		t.Error("rebuy with chips left should fail")
	}
	tb.StartHand()
	tb.Players[0].Chips = 0 // ann is all-in in a live hand
	if err := tb.Rebuy("p1"); err == nil {
		t.Error("rebuy during your own hand should fail")
	}
	mustAct(t, tb, "p1", "fold", 0)
	if err := tb.Rebuy("p1"); err != nil {
		t.Fatalf("rebuy between hands: %v", err)
	}
	if p := tb.Players[0]; p.Chips != 1000 || p.Buyins != 2 {
		t.Errorf("chips %d buyins %d, want 1000 and 2", p.Chips, p.Buyins)
	}
	tb.StartHand()
	if tb.Players[0].Buyins != 2 {
		t.Error("starting a hand must keep the buy-in count")
	}
}
