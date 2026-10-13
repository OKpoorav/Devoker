// Command server runs Devoker: Texas Hold'em rooms over HTTP. Players get live table
// updates through Server-Sent Events and send actions with plain POST requests.
package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OKpoorav/devoker/poker"
)

// pending is a join or rebuy waiting for the host's decision.
type pending struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // "join" or "rebuy"
	Name     string `json:"name"`
	token    string // join: the token handed to the waiting player
	playerID string // rebuy: who asked
}

type room struct {
	mu       sync.Mutex
	table    *poker.Table
	owner    string            // player ID of the host, who approves joins and rebuys
	tokens   map[string]string // secret player token -> player ID, once seated
	requests []*pending
	rejected map[string]bool // tokens of declined join requests
	subs     map[chan struct{}]bool
}

// changed wakes every open event stream so it sends a fresh view. Caller holds r.mu.
func (r *room) changed() {
	for ch := range r.subs {
		select {
		case ch <- struct{}{}:
		default: // an update is already pending for this stream
		}
	}
}

// roomView is what one event stream receives. Status is "seated", "pending" or "rejected";
// the table fields are only filled in once seated.
type roomView struct {
	poker.View
	Status       string     `json:"status"`
	IsOwner      bool       `json:"isOwner"`
	Owner        string     `json:"owner"`
	Requests     []*pending `json:"requests"`
	RebuyPending bool       `json:"rebuyPending"`
}

// view builds the stream payload for a token. Caller holds r.mu.
func (r *room) view(token string) roomView {
	id, seated := r.tokens[token]
	switch {
	case seated:
		v := roomView{View: r.table.View(id), Status: "seated", IsOwner: id == r.owner, Owner: r.owner, Requests: []*pending{}}
		if v.IsOwner {
			v.Requests = append([]*pending{}, r.requests...) // a copy (decide shifts the original), never null in JSON
		}
		v.RebuyPending = slices.ContainsFunc(r.requests, func(p *pending) bool { return p.playerID == id })
		return v
	case r.rejected[token]:
		return roomView{Status: "rejected"}
	default:
		return roomView{Status: "pending"}
	}
}

// known reports whether a token belongs to this room in any state.
func (r *room) known(token string) bool {
	_, seated := r.tokens[token]
	return seated || r.rejected[token] || slices.ContainsFunc(r.requests, func(p *pending) bool { return p.token == token })
}

type server struct {
	mu                          sync.Mutex
	rooms                       map[string]*room // shortcut: rooms live in memory forever; add expiry if this runs long-term
	chips, smallBlind, bigBlind int
}

func main() {
	s := &server{
		rooms:      map[string]*room{},
		chips:      envInt("STARTING_CHIPS", 1000),
		smallBlind: envInt("SMALL_BLIND", 10),
		bigBlind:   envInt("BIG_BLIND", 20),
	}
	addr := ":" + os.Getenv("PORT")
	if addr == ":" {
		addr = ":8080"
	}
	log.Printf("devoker listening on %s (chips %d, blinds %d/%d)", addr, s.chips, s.smallBlind, s.bigBlind)
	// No WriteTimeout: event streams stay open for the whole game.
	srv := &http.Server{Addr: addr, Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("POST /api/rooms", s.create)
	mux.HandleFunc("POST /api/rooms/{code}/join", s.join)
	mux.HandleFunc("POST /api/rooms/{code}/start", s.start)
	mux.HandleFunc("POST /api/rooms/{code}/action", s.action)
	mux.HandleFunc("POST /api/rooms/{code}/rebuy", s.rebuy)
	mux.HandleFunc("POST /api/rooms/{code}/requests/{id}", s.decide)
	mux.HandleFunc("GET /api/rooms/{code}/events", s.events)
	return mux
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

type request struct {
	Name    string `json:"name"`
	Token   string `json:"token"`
	Action  string `json:"action"`
	Amount  int    `json:"amount"`
	Approve bool   `json:"approve"`
}

func readRequest(w http.ResponseWriter, r *http.Request) (request, bool) {
	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body")
		return req, false
	}
	return req, true
}

func httpError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func cleanName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	return name, name != "" && len([]rune(name)) <= 20
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(w, r)
	if !ok {
		return
	}
	name, ok := cleanName(req.Name)
	if !ok {
		httpError(w, http.StatusBadRequest, "name must be 1-20 characters")
		return
	}
	rm := &room{
		table:    poker.NewTable(s.chips, s.smallBlind, s.bigBlind),
		tokens:   map[string]string{},
		rejected: map[string]bool{},
		subs:     map[chan struct{}]bool{},
	}
	host, _ := rm.table.AddPlayer(name) // an empty table always has room
	rm.owner = host.ID
	token := rand.Text()
	rm.tokens[token] = host.ID

	s.mu.Lock()
	code := rand.Text()[:5]
	for s.rooms[code] != nil {
		code = rand.Text()[:5]
	}
	s.rooms[code] = rm
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"room": code, "token": token})
}

// join asks the host for a seat. The player gets a token right away and watches their
// event stream until the host approves or declines.
func (s *server) join(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(w, r)
	if !ok {
		return
	}
	name, ok := cleanName(req.Name)
	if !ok {
		httpError(w, http.StatusBadRequest, "name must be 1-20 characters")
		return
	}
	code := strings.ToUpper(r.PathValue("code"))
	rm := s.room(code)
	if rm == nil {
		httpError(w, http.StatusNotFound, "room not found")
		return
	}
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if len(rm.table.Players) >= poker.MaxPlayers {
		httpError(w, http.StatusConflict, "table is full")
		return
	}
	token := rand.Text()
	rm.requests = append(rm.requests, &pending{ID: rand.Text()[:8], Kind: "join", Name: name, token: token})
	rm.changed()
	writeJSON(w, http.StatusOK, map[string]string{"room": code, "token": token})
}

func (s *server) room(code string) *room {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rooms[strings.ToUpper(code)]
}

// seated finds the room and the seated player behind a token. On success it returns with
// rm.mu held; the caller must unlock it. On failure it has written the error response.
func (s *server) seated(w http.ResponseWriter, code, token string) (*room, string, bool) {
	rm := s.room(code)
	if rm == nil {
		httpError(w, http.StatusNotFound, "room not found")
		return nil, "", false
	}
	rm.mu.Lock()
	id, ok := rm.tokens[token]
	if !ok {
		rm.mu.Unlock()
		httpError(w, http.StatusForbidden, "you are not seated at this table")
		return nil, "", false
	}
	return rm, id, true
}

func (s *server) start(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(w, r)
	if !ok {
		return
	}
	rm, _, ok := s.seated(w, r.PathValue("code"), req.Token)
	if !ok {
		return
	}
	defer rm.mu.Unlock()
	if err := rm.table.StartHand(); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	rm.changed()
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) action(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(w, r)
	if !ok {
		return
	}
	rm, id, ok := s.seated(w, r.PathValue("code"), req.Token)
	if !ok {
		return
	}
	defer rm.mu.Unlock()
	if err := rm.table.Act(id, req.Action, req.Amount); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	rm.changed()
	w.WriteHeader(http.StatusNoContent)
}

// rebuy tops a busted player back up. The host's own rebuy goes straight through;
// everyone else's waits for the host.
func (s *server) rebuy(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(w, r)
	if !ok {
		return
	}
	rm, id, ok := s.seated(w, r.PathValue("code"), req.Token)
	if !ok {
		return
	}
	defer rm.mu.Unlock()
	if err := rm.table.CanRebuy(id); err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	if id == rm.owner {
		rm.table.Rebuy(id)
	} else if !slices.ContainsFunc(rm.requests, func(p *pending) bool { return p.playerID == id }) {
		i := slices.IndexFunc(rm.table.Players, func(p *poker.Player) bool { return p.ID == id })
		rm.requests = append(rm.requests, &pending{ID: rand.Text()[:8], Kind: "rebuy", Name: rm.table.Players[i].Name, playerID: id})
	}
	rm.changed()
	w.WriteHeader(http.StatusNoContent)
}

// decide lets the host approve or decline a pending join or rebuy.
func (s *server) decide(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(w, r)
	if !ok {
		return
	}
	rm, id, ok := s.seated(w, r.PathValue("code"), req.Token)
	if !ok {
		return
	}
	defer rm.mu.Unlock()
	if id != rm.owner {
		httpError(w, http.StatusForbidden, "only the host can do that")
		return
	}
	i := slices.IndexFunc(rm.requests, func(p *pending) bool { return p.ID == r.PathValue("id") })
	if i < 0 {
		httpError(w, http.StatusNotFound, "request not found")
		return
	}
	p := rm.requests[i]
	rm.requests = slices.Delete(rm.requests, i, i+1)
	defer rm.changed()

	var err error
	switch {
	case p.Kind == "join" && req.Approve:
		var seat *poker.Player
		if seat, err = rm.table.AddPlayer(p.Name); err == nil {
			rm.tokens[p.token] = seat.ID
		} else {
			rm.rejected[p.token] = true
		}
	case p.Kind == "join":
		rm.rejected[p.token] = true
	case req.Approve:
		err = rm.table.Rebuy(p.playerID)
	}
	if err != nil {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// events streams the room, as this token may see it, every time it changes.
// The token is in the query string because the browser's EventSource cannot send headers.
func (s *server) events(w http.ResponseWriter, r *http.Request) {
	rm := s.room(r.PathValue("code"))
	if rm == nil {
		httpError(w, http.StatusNotFound, "room not found")
		return
	}
	token := r.URL.Query().Get("token")
	rm.mu.Lock()
	known := rm.known(token)
	rm.mu.Unlock()
	if !known {
		httpError(w, http.StatusForbidden, "unknown player")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // stop nginx / ingress-nginx buffering the stream
	rc := http.NewResponseController(w)

	ch := make(chan struct{}, 1)
	ch <- struct{}{} // send the current state straight away
	rm.mu.Lock()
	rm.subs[ch] = true
	rm.mu.Unlock()
	defer func() {
		rm.mu.Lock()
		delete(rm.subs, ch)
		rm.mu.Unlock()
	}()

	ping := time.NewTicker(25 * time.Second) // keeps proxies from closing an idle stream
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
		case <-ch:
			rm.mu.Lock()
			b, err := json.Marshal(rm.view(token))
			rm.mu.Unlock()
			if err != nil {
				log.Printf("encode view: %v", err)
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		if rc.Flush() != nil {
			return
		}
	}
}
