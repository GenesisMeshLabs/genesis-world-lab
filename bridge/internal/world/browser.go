package world

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

//go:embed web/*
var browserAssets embed.FS

type browserSession struct {
	Player      string
	Expires     time.Time
	LastCommand time.Time
}
type gameCommand struct {
	ID       string `json:"id"`
	Player   string `json:"player"`
	Action   string `json:"action"`
	X        int    `json:"x"`
	Z        int    `json:"z"`
	Material string `json:"material,omitempty"`
	Expires  int64  `json:"expires"`
	Session  string `json:"-"`
}
type enginePlayer struct {
	Name string  `json:"name"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
}
type engineResult struct {
	ID      string `json:"id"`
	Player  string `json:"player"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}
type engineFrame struct {
	Layers  []string       `json:"layers"`
	Players []enginePlayer `json:"players"`
	Results []engineResult `json:"results"`
}
type browserState struct {
	Sessions  map[string]*browserSession
	Commands  []gameCommand
	Frame     engineFrame
	Updated   time.Time
	LoginRate rate
}

func (s *Server) initBrowser() {
	s.browser.Sessions = map[string]*browserSession{}
	s.mux.HandleFunc("POST /v1/game/frame", s.gameFrame)
}

func (s *Server) browserOrigin(r *http.Request) bool {
	host, port, e := net.SplitHostPort(r.Host)
	_, expectedPort, _ := net.SplitHostPort(s.cfg.Address)
	if e != nil || port != expectedPort || (host != "127.0.0.1" && host != "localhost" && host != "::1") {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || u.Scheme != "http" || u.Host != r.Host {
			return false
		}
	}
	return r.Header.Get("Sec-Fetch-Site") != "cross-site"
}

func (s *Server) serveBrowser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if !s.browserOrigin(r) {
		respond(w, 403, Document{"error": "Open the local lab address directly"})
		return
	}
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/play/", http.StatusTemporaryRedirect)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/play/api/") {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		files, _ := fs.Sub(browserAssets, "web")
		http.StripPrefix("/play/", http.FileServer(http.FS(files))).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/play/api/login" && r.Method == "POST" {
		s.browserLogin(w, r)
		return
	}
	s.mu.Lock()
	key := HashToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	session := s.browser.Sessions[key]
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || session == nil || !time.Now().Before(session.Expires) {
		delete(s.browser.Sessions, key)
		s.mu.Unlock()
		respond(w, 401, Document{"error": "Sign in to play"})
		return
	}
	name := session.Player
	requestRate := s.rates["browser:"+name]
	if time.Since(requestRate.at) >= time.Minute {
		requestRate = rateZero()
	}
	requestRate.n++
	s.rates["browser:"+name] = requestRate
	if requestRate.n > 1200 {
		s.mu.Unlock()
		respond(w, 429, Document{"error": "Browser request quota exceeded"})
		return
	}
	switch r.URL.Path {
	case "/play/api/state":
		if r.Method != "GET" {
			s.mu.Unlock()
			w.WriteHeader(405)
			return
		}
		caps := []string{}
		for _, c := range append(append([]string{}, Baseline...), Protected...) {
			if s.allowed(name, c, s.cfg.Area) {
				caps = append(caps, c)
			}
		}
		events, auditError := s.db.LatestAudit(64)
		if auditError != nil {
			s.mu.Unlock()
			respond(w, 503, Document{"error": "Audit unavailable"})
			return
		}
		visible := []Event{}
		for _, ev := range events {
			if ev.Player == name || s.players[name].OperatorAuthority != "" {
				visible = append(visible, ev)
			}
		}
		if len(visible) > 12 {
			visible = visible[len(visible)-12:]
		}
		grants := []Grant{}
		for _, g := range s.state.Grants {
			if (g.Player == name || g.Authority == s.players[name].OperatorAuthority) && s.grantValid(g, 0) {
				grants = append(grants, g)
			}
		}
		players := []string{}
		authorities := []string{}
		for _, p := range s.cfg.Players {
			players = append(players, p.Name)
		}
		for _, a := range s.cfg.Authorities {
			authorities = append(authorities, a.Name)
		}
		result := Document{"player": name, "identity": s.identity(name), "operator": s.players[name].OperatorAuthority != "", "ready": s.healthy(), "engine_online": time.Since(s.browser.Updated) < 2*time.Second, "frame": s.browser.Frame, "capabilities": caps, "events": visible, "grants": grants, "players": players, "authorities": authorities, "expires_at": session.Expires}
		s.mu.Unlock()
		respond(w, 200, result)
	case "/play/api/command":
		if r.Method != "POST" {
			s.mu.Unlock()
			w.WriteHeader(405)
			return
		}
		s.mu.Unlock()
		s.browserCommand(w, r, key)
	case "/play/api/logout":
		if r.Method != "POST" {
			s.mu.Unlock()
			w.WriteHeader(405)
			return
		}
		delete(s.browser.Sessions, key)
		s.mu.Unlock()
		respond(w, 200, Document{"ok": true})
	case "/play/api/delegate", "/play/api/revoke":
		authority := s.players[name].OperatorAuthority
		operatorRate := s.rates["browser-operator:"+name]
		if time.Since(operatorRate.at) >= time.Minute {
			operatorRate = rateZero()
		}
		operatorRate.n++
		s.rates["browser-operator:"+name] = operatorRate
		validOperator := s.identityValid(name)
		s.mu.Unlock()
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		if authority == "" || !validOperator {
			respond(w, 403, Document{"error": "An authority operator account is required"})
			return
		}
		if operatorRate.n > 120 {
			respond(w, 429, Document{"error": "Operator request quota exceeded"})
			return
		}
		// Reuse the same scoped, signed authority operations as native operators.
		ctx := context.WithValue(r.Context(), principalKey{}, Principal{Name: "browser:" + name, Authority: authority})
		if strings.HasSuffix(r.URL.Path, "/delegate") {
			s.delegate(w, r.WithContext(ctx))
		} else {
			s.revoke(w, r.WithContext(ctx))
		}
	default:
		s.mu.Unlock()
		http.NotFound(w, r)
	}
}

func (s *Server) browserLogin(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Player   string `json:"player"`
		Password string `json:"password"`
	}
	if !body(w, r, &q) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rate := &s.browser.LoginRate
	if time.Since(rate.at) > time.Minute {
		*rate = rateZero()
	}
	rate.n++
	if rate.n > 20 {
		respond(w, 429, Document{"error": "Too many login attempts. Wait one minute."})
		return
	}
	var accounts map[string]struct {
		Password string `json:"password"`
	}
	b, e := os.ReadFile(s.cfg.BrowserAccountsFile)
	if e != nil || json.Unmarshal(b, &accounts) != nil {
		respond(w, 503, Document{"error": "Browser accounts are not configured. Run setup."})
		return
	}
	expected := accounts[q.Player].Password
	a, z := HashToken(expected), HashToken(q.Password)
	if expected == "" || subtle.ConstantTimeCompare([]byte(a), []byte(z)) != 1 || !s.identityValid(q.Player) {
		respond(w, 401, Document{"error": "Invalid credentials or identity unavailable"})
		return
	}
	// One control session per player; replacing it invalidates queued commands.
	for k, v := range s.browser.Sessions {
		if v.Player == q.Player || !time.Now().Before(v.Expires) {
			delete(s.browser.Sessions, k)
		}
	}
	raw := make([]byte, 32)
	if _, e := rand.Read(raw); e != nil {
		respond(w, 500, Document{"error": "Session creation failed"})
		return
	}
	token := hex.EncodeToString(raw)
	s.browser.Sessions[HashToken(token)] = &browserSession{Player: q.Player, Expires: time.Now().Add(30 * time.Minute)}
	if !s.commit(w, Event{Actor: q.Player, Action: "browser.login", Player: q.Player}) {
		delete(s.browser.Sessions, HashToken(token))
		return
	}
	respond(w, 200, Document{"token": token, "player": q.Player})
}
func rateZero() rate { return rate{at: time.Now()} }

func (s *Server) browserCommand(w http.ResponseWriter, r *http.Request, key string) {
	var q struct {
		Action   string `json:"action"`
		X        int    `json:"x"`
		Z        int    `json:"z"`
		Material string `json:"material"`
	}
	if !body(w, r, &q) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.browser.Sessions[key]
	if session == nil || !time.Now().Before(session.Expires) {
		respond(w, 401, Document{"error": "Session expired"})
		return
	}
	if time.Since(session.LastCommand) < 100*time.Millisecond {
		respond(w, 429, Document{"error": "Move one step at a time"})
		return
	}
	if !s.allowed(session.Player, "world.read", "") || time.Since(s.browser.Updated) > 2*time.Second {
		respond(w, 503, Document{"error": "Controls paused: world or trust unavailable"})
		return
	}
	connected := false
	for _, p := range s.browser.Frame.Players {
		if p.Name == session.Player {
			connected = true
		}
	}
	if !connected {
		respond(w, 409, Document{"error": "Start this player's native relay using scripts/play-web.ps1"})
		return
	}
	if !contains([]string{"move", "place", "dig", "lobby"}, q.Action) || q.X < -8 || q.X > 40 || q.Z < -12 || q.Z > 12 {
		respond(w, 400, Document{"error": "Unsupported world action"})
		return
	}
	if q.Action == "move" && (q.X*q.X+q.Z*q.Z != 1) {
		respond(w, 400, Document{"error": "Movement must be one cardinal step"})
		return
	}
	if q.Action == "place" && !contains([]string{"stone", "wood", "glass"}, q.Material) {
		respond(w, 400, Document{"error": "Choose a hotbar material"})
		return
	}
	if len(s.browser.Commands) >= 32 {
		respond(w, 429, Document{"error": "World command queue is full"})
		return
	}
	raw := make([]byte, 12)
	if _, e := rand.Read(raw); e != nil {
		w.WriteHeader(500)
		return
	}
	id := hex.EncodeToString(raw)
	s.browser.Commands = append(s.browser.Commands, gameCommand{ID: id, Player: session.Player, Action: q.Action, X: q.X, Z: q.Z, Material: q.Material, Expires: time.Now().Add(time.Second).UnixMilli(), Session: key})
	session.LastCommand = time.Now()
	respond(w, 202, Document{"id": id})
}

func (s *Server) gameFrame(w http.ResponseWriter, r *http.Request) {
	if !principal(r).Game {
		respond(w, 403, Document{"error": "Game credential required"})
		return
	}
	var frame engineFrame
	if !body(w, r, &frame) {
		return
	}
	if len(frame.Layers) != 4 || len(frame.Players) > 8 || len(frame.Results) > 32 {
		respond(w, 400, Document{"error": "Invalid world frame"})
		return
	}
	for _, layer := range frame.Layers {
		if len(layer) != 49*25 {
			respond(w, 400, Document{"error": "Invalid world dimensions"})
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, result := range frame.Results {
		if !s.commit(w, Event{Actor: "luanti", Action: "browser.action", Player: result.Player, Detail: result.Message}) {
			return
		}
	}
	// Preserve recent acknowledged actions when a frame has no new results.
	frame.Results = append(s.browser.Frame.Results, frame.Results...)
	if len(frame.Results) > 24 {
		frame.Results = frame.Results[len(frame.Results)-24:]
	}
	s.browser.Frame = frame
	s.browser.Updated = time.Now()
	commands := []gameCommand{}
	for _, c := range s.browser.Commands {
		v := s.browser.Sessions[c.Session]
		if v != nil && time.Now().Before(v.Expires) && time.Now().UnixMilli() < c.Expires && s.allowed(c.Player, "world.read", "") {
			commands = append(commands, c)
		}
	}
	s.browser.Commands = nil
	respond(w, 200, Document{"commands": commands})
}
