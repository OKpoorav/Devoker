package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func call(t *testing.T, h http.Handler, path string, body map[string]any) (int, map[string]string) {
	t.Helper()
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", path, strings.NewReader(string(b))))
	var out map[string]string
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestHostApprovesJoinsAndRebuys(t *testing.T) {
	s := &server{rooms: map[string]*room{}, chips: 1000, smallBlind: 10, bigBlind: 20}
	h := s.routes()

	_, host := call(t, h, "/api/rooms", map[string]any{"name": "host"})
	code := host["room"]
	rm := s.rooms[code]
	if b, _ := json.Marshal(rm.view(host["token"])); strings.Contains(string(b), `"requests":null`) {
		t.Error("requests must be [] not null: the frontend reads .length")
	}
	_, bob := call(t, h, "/api/rooms/"+code+"/join", map[string]any{"name": "bob"})
	_, eve := call(t, h, "/api/rooms/"+code+"/join", map[string]any{"name": "eve"})

	if st := rm.view(bob["token"]).Status; st != "pending" {
		t.Fatalf("bob should wait for the host, got %s", st)
	}
	if code, _ := call(t, h, "/api/rooms/"+code+"/action", map[string]any{"token": bob["token"], "action": "fold"}); code != http.StatusForbidden {
		t.Errorf("pending player acting: got %d, want 403", code)
	}
	reqs := rm.view(host["token"]).Requests
	if len(reqs) != 2 {
		t.Fatalf("host should see 2 requests, got %d", len(reqs))
	}
	if code, _ := call(t, h, "/api/rooms/"+code+"/requests/"+reqs[0].ID, map[string]any{"token": bob["token"], "approve": true}); code != http.StatusForbidden {
		t.Errorf("non-host deciding: got %d, want 403", code)
	}
	call(t, h, "/api/rooms/"+code+"/requests/"+reqs[0].ID, map[string]any{"token": host["token"], "approve": true})
	call(t, h, "/api/rooms/"+code+"/requests/"+reqs[1].ID, map[string]any{"token": host["token"], "approve": false})
	if st := rm.view(bob["token"]).Status; st != "seated" {
		t.Errorf("bob: got %s, want seated", st)
	}
	if st := rm.view(eve["token"]).Status; st != "rejected" {
		t.Errorf("eve: got %s, want rejected", st)
	}

	// bob busts and asks to rebuy; the host approves
	rm.table.Players[1].Chips = 0
	if code, _ := call(t, h, "/api/rooms/"+code+"/rebuy", map[string]any{"token": bob["token"]}); code != http.StatusNoContent {
		t.Fatalf("rebuy request: got %d", code)
	}
	if !rm.view(bob["token"]).RebuyPending || rm.table.Players[1].Chips != 0 {
		t.Fatal("bob's rebuy should wait for the host")
	}
	reqs = rm.view(host["token"]).Requests
	call(t, h, "/api/rooms/"+code+"/requests/"+reqs[0].ID, map[string]any{"token": host["token"], "approve": true})
	if p := rm.table.Players[1]; p.Chips != 1000 || p.Buyins != 2 {
		t.Errorf("after rebuy: chips %d buyins %d", p.Chips, p.Buyins)
	}

	// the host's own rebuy needs no approval
	rm.table.Players[0].Chips = 0
	call(t, h, "/api/rooms/"+code+"/rebuy", map[string]any{"token": host["token"]})
	if rm.table.Players[0].Chips != 1000 {
		t.Error("host rebuy should be immediate")
	}
}
