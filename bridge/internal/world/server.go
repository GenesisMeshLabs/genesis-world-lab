package world

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	mu           sync.Mutex
	experimentMu sync.Mutex
	cfg          Config
	db           *Store
	net          *Network
	state        State
	authorities  map[string]Authority
	players      map[string]Player
	identities   map[string]Document
	roots        map[string]Document
	freshUntil   time.Time
	failed       bool
	lastError    string
	mux          *http.ServeMux
	rates        map[string]rate
	browser      browserState
}
type rate struct {
	at time.Time
	n  int
}

func New(c Config) (*Server, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	db, e := OpenStore(c.StateFile)
	if e != nil {
		return nil, e
	}
	s := &Server{cfg: c, db: db, net: NewNetwork(), authorities: map[string]Authority{}, players: map[string]Player{}, identities: map[string]Document{}, roots: map[string]Document{}, rates: map[string]rate{}, mux: http.NewServeMux()}
	s.state, e = db.Load()
	if e != nil {
		db.Close()
		return nil, e
	}
	for _, a := range c.Authorities {
		s.authorities[a.Name] = a
		d, e := readDocument(a.RootFile)
		if e != nil {
			db.Close()
			return nil, e
		}
		if e = VerifyDocument(d, a); e != nil {
			db.Close()
			return nil, e
		}
		s.roots[a.Name] = d
	}
	for _, p := range c.Players {
		s.players[p.Name] = p
		d, e := readDocument(p.IdentityFile)
		if e != nil {
			db.Close()
			return nil, e
		}
		if e = VerifyDocument(d, s.authorities[p.Authority]); e != nil {
			db.Close()
			return nil, e
		}
		if str(d, "subject_public_key") != p.PublicKey || str(claims(d), "player") != p.Name {
			db.Close()
			return nil, errors.New("identity binding mismatch")
		}
		s.identities[p.Name] = d
	}
	for _, g := range s.state.Grants {
		if e := s.verifyGrant(g); e != nil {
			db.Close()
			return nil, e
		}
	}
	for name, feed := range s.state.Floors {
		if e := VerifyDocument(feed, s.authorities[name]); e != nil {
			db.Close()
			return nil, e
		}
	}
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"status": "ok", "mode": "genesismesh", "world": c.World})
	})
	s.mux.HandleFunc("GET /readyz", s.ready)
	s.mux.HandleFunc("POST /v1/identities/link", s.link)
	s.mux.HandleFunc("GET /v1/identities/{player}/capabilities", s.capabilities)
	s.mux.HandleFunc("POST /v1/check", s.check)
	s.mux.HandleFunc("POST /v1/delegate", s.delegate)
	s.mux.HandleFunc("POST /v1/revoke", s.revoke)
	s.mux.HandleFunc("GET /v1/audit", s.audit)
	s.mux.HandleFunc("GET /v1/status", s.status)
	s.initBrowser()
	return s, nil
}
func (s *Server) Close() error { return s.db.Close() }
func (s *Server) Run(ctx context.Context) {
	for {
		s.Refresh(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
func (s *Server) Refresh(ctx context.Context) {
	feeds := map[string]Document{}
	var err error
	for _, a := range s.cfg.Authorities {
		d, e := s.net.Call(ctx, a.URL, "/sovereign-revocation-feed", nil, nil)
		if e != nil {
			err = e
			break
		}
		if e = VerifyDocument(d, a); e != nil {
			err = e
			break
		}
		t := timestamp(d, "issued_at")
		if t.Before(time.Now().Add(-24*time.Hour)) || t.After(time.Now().Add(2*time.Minute)) {
			err = errors.New("stale revocation feed")
			break
		}
		feeds[a.Name] = d
	}
	if err == nil {
		var mesh Document
		mesh, err = s.net.Call(ctx, s.cfg.GatewayURL, "/v1/mesh", nil, nil)
		if err == nil {
			err = s.checkMesh(mesh)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.freshUntil = time.Time{}
		s.lastError = err.Error()
		return
	}
	for name, d := range feeds {
		previous := s.state.Floors[name]
		if e := validateFeedAdvance(previous, d); e != nil {
			s.freshUntil = time.Time{}
			s.lastError = e.Error()
			return
		}
	}
	changed := false
	for name, d := range feeds {
		old, _ := sequence(s.state.Floors[name])
		seq, _ := sequence(d)
		if s.state.Floors[name] == nil || old != seq {
			changed = true
		}
		s.state.Floors[name] = d
	}
	if changed {
		if e := s.db.Save(s.state, &Event{Actor: "bridge", Action: "trust.refresh"}); e != nil {
			s.failed = true
			s.lastError = "durable state write failed"
			return
		}
	}
	s.freshUntil = time.Now().Add(6 * time.Second)
	s.lastError = ""
	for id, g := range s.state.Grants {
		if !g.ExpiryRecorded && !time.Now().Before(g.Expires) {
			g.ExpiryRecorded = true
			s.state.Grants[id] = g
			if e := s.db.Save(s.state, &Event{Actor: "bridge", Action: "capability.expire", Player: g.Player, GrantID: id}); e != nil {
				s.failed = true
				s.lastError = "expiry audit unavailable"
				return
			}
		}
	}
}

func (s *Server) verifyGrant(g Grant) error {
	a, ok := s.authorities[g.Authority]
	if !ok {
		return errors.New("unknown grant issuer")
	}
	if e := VerifyDocument(g.Evidence, a); e != nil {
		return e
	}
	c := claims(g.Evidence)
	p, ok := s.players[g.Player]
	caps := stringsOf(c["capabilities"])
	if !ok || g.ID != str(g.Evidence, "attestation_id") || str(g.Evidence, "subject_public_key") != p.PublicKey || str(g.Evidence, "subject_id") != str(s.identities[g.Player], "subject_id") || str(c, "kind") != "grant" || str(c, "profile") != "worldlab/v1" || str(c, "world") != s.cfg.World || str(c, "player") != g.Player || str(c, "parent_id") != g.Parent || str(c, "area") != g.Area || len(caps) != 1 || caps[0] != g.Capability || !timestamp(c, "until").Equal(g.Expires) {
		return errors.New("persisted grant does not match signed scope")
	}
	return nil
}
func sequence(d Document) (int64, error) {
	if d == nil {
		return -1, nil
	}
	switch n := d["sequence"].(type) {
	case json.Number:
		return n.Int64()
	case float64:
		if n < 0 || n != float64(int64(n)) {
			return 0, errors.New("invalid sequence")
		}
		return int64(n), nil
	}
	return 0, errors.New("invalid sequence")
}
func sameRevocations(a, b Document) bool {
	x := stringsOf(a["revoked_attestation_ids"])
	y := stringsOf(b["revoked_attestation_ids"])
	sort.Strings(x)
	sort.Strings(y)
	if strings.Join(x, "\x00") != strings.Join(y, "\x00") {
		return false
	}
	xb, _ := json.Marshal(a["revocation_reasons"])
	yb, _ := json.Marshal(b["revocation_reasons"])
	return string(xb) == string(yb)
}
func (s *Server) checkMesh(d Document) error {
	ready := map[string]bool{}
	rows, _ := d["networks"].([]any)
	for _, row := range rows {
		m, _ := row.(map[string]any)
		name := str(Document(m), "id")
		if name == "" {
			name = str(Document(m), "network_name")
		}
		if m["trust_ready"] == true {
			ready[name] = true
		}
	}
	links, _ := d["links"].([]any)
	for name := range s.authorities {
		if !ready[name] {
			return errors.New("configured network is not trust-ready")
		}
		if name == s.cfg.Owner {
			continue
		}
		recognized := false
		for _, row := range links {
			m, _ := row.(map[string]any)
			if m["from"] == s.cfg.Owner && m["to"] == name && timestamp(Document(m), "expires_at").After(time.Now()) {
				recognized = true
			}
		}
		if !recognized {
			return errors.New("world owner does not recognize authority")
		}
	}
	return nil
}
func (s *Server) healthy() bool { return !s.failed && time.Now().Before(s.freshUntil) }
func (s *Server) valid(d Document, a string) bool {
	if !s.healthy() || d == nil {
		return false
	}
	now := time.Now()
	c := claims(d)
	if str(c, "profile") != "worldlab/v1" || str(c, "world") != s.cfg.World || str(d, "status") != "active" || !timestamp(d, "expires_at").After(now) || timestamp(d, "valid_from").After(now) || !timestamp(c, "until").After(now) {
		return false
	}
	return !contains(stringsOf(s.state.Floors[a]["revoked_attestation_ids"]), str(d, "attestation_id"))
}
func (s *Server) rootAllows(authority, cap, area string) bool {
	d := s.roots[authority]
	return s.valid(d, authority) && str(claims(d), "kind") == "authority" && str(claims(d), "area") == area && contains(stringsOf(claims(d)["capabilities"]), cap)
}
func (s *Server) identityValid(player string) bool {
	p, ok := s.players[player]
	return ok && s.valid(s.identities[player], p.Authority)
}
func (s *Server) grantValid(g Grant, depth int) bool {
	if depth > 8 || g.Revoked || !time.Now().Before(g.Expires) || !s.valid(g.Evidence, g.Authority) || !s.identityValid(g.Player) {
		return false
	}
	if g.Parent == str(s.roots[g.Authority], "attestation_id") {
		return s.rootAllows(g.Authority, g.Capability, g.Area)
	}
	p, ok := s.state.Grants[g.Parent]
	return ok && p.Authority == g.Authority && p.Capability == g.Capability && p.Area == g.Area && !g.Expires.After(p.Expires) && s.grantValid(p, depth+1)
}
func (s *Server) allowed(player, cap, area string) bool {
	if !s.state.Linked[player] || !s.identityValid(player) {
		return false
	}
	if contains(Baseline, cap) && contains(stringsOf(claims(s.identities[player])["capabilities"]), cap) {
		return true
	}
	for _, g := range s.state.Grants {
		if g.Player == player && g.Capability == cap && g.Area == area && s.grantValid(g, 0) {
			return true
		}
	}
	return false
}

type principalKey struct{}

func principal(r *http.Request) Principal { return r.Context().Value(principalKey{}).(Principal) }
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if s.publicBrowserRequest(r) && r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/play/") {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/play/") {
		s.serveBrowser(w, r)
		return
	}
	if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
		s.mux.ServeHTTP(w, r)
		return
	}
	if r.Header.Get("Origin") != "" {
		respond(w, 403, Document{"error": "browser origins are not accepted"})
		return
	}
	h := HashToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	var p *Principal
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		for _, v := range s.cfg.Principals {
			if subtle.ConstantTimeCompare([]byte(h), []byte(v.TokenHash)) == 1 {
				x := v
				p = &x
			}
		}
	}
	if p == nil {
		respond(w, 401, Document{"error": "authentication required"})
		return
	}
	s.mu.Lock()
	rate := s.rates[p.Name]
	if time.Since(rate.at) >= time.Minute {
		rate.at = time.Now()
		rate.n = 0
	}
	rate.n++
	s.rates[p.Name] = rate
	s.mu.Unlock()
	limit := 120
	if p.Game {
		limit = 6000
	}
	if rate.n > limit {
		w.Header().Set("Retry-After", "60")
		respond(w, 429, Document{"error": "request limit"})
		return
	}
	s.mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, *p)))
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := 503
	if s.healthy() {
		status = 200
	}
	respond(w, status, Document{"ready": s.healthy(), "mode": "genesismesh"})
}
func body(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		respond(w, 400, Document{"error": "invalid request body"})
		return false
	}
	if e := d.Decode(new(any)); e != io.EOF {
		respond(w, 400, Document{"error": "one JSON object required"})
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) commit(w http.ResponseWriter, event Event) bool {
	if e := s.db.Save(s.state, &event); e != nil {
		s.failed = true
		respond(w, 503, Document{"error": "durable audit unavailable"})
		return false
	}
	return true
}
func (s *Server) link(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Player string `json:"player_name"`
	}
	if !body(w, r, &q) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !principal(r).Game {
		respond(w, 403, Document{"error": "game credential required"})
		return
	}
	if !s.identityValid(q.Player) || !contains(stringsOf(claims(s.identities[q.Player])["capabilities"]), "game.connect") {
		respond(w, 403, Document{"error": "invited, valid identity required"})
		return
	}
	if !s.state.Linked[q.Player] {
		s.state.Linked[q.Player] = true
		if !s.commit(w, Event{Actor: principal(r).Name, Action: "identity.link", Player: q.Player}) {
			return
		}
	}
	respond(w, 200, Document{"identity": s.identity(q.Player)})
}
func (s *Server) identity(player string) Document {
	d := s.identities[player]
	return Document{"player_name": player, "genesismesh_id": str(d, "issuer_sovereign_id") + ":" + str(d, "subject_id"), "attestation_id": str(d, "attestation_id"), "authority": s.players[player].Authority}
}
func (s *Server) capabilities(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	player := r.PathValue("player")
	if !s.identityValid(player) || !s.state.Linked[player] {
		respond(w, 403, Document{"error": "identity or trust unavailable"})
		return
	}
	caps := []Document{}
	lease := 1500
	until := time.Now().Add(time.Duration(lease) * time.Millisecond)
	for _, t := range []time.Time{s.freshUntil, timestamp(s.identities[player], "expires_at"), timestamp(claims(s.identities[player]), "until")} {
		if t.Before(until) {
			until = t
		}
	}
	for _, root := range s.roots {
		for _, t := range []time.Time{timestamp(root, "expires_at"), timestamp(claims(root), "until")} {
			if t.Before(until) {
				until = t
			}
		}
	}
	for _, cap := range Baseline {
		if s.allowed(player, cap, "") {
			caps = append(caps, Document{"capability": cap, "area": ""})
		}
	}
	for _, g := range s.state.Grants {
		if g.Player == player && s.grantValid(g, 0) {
			caps = append(caps, Document{"id": g.ID, "capability": g.Capability, "area": g.Area})
			if g.Expires.Before(until) {
				until = g.Expires
			}
		}
	}
	respond(w, 200, Document{"identity": s.identity(player), "capabilities": caps, "lease_ms": max(0, time.Until(until).Milliseconds())})
}
func (s *Server) check(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Player     string `json:"player_name"`
		Capability string `json:"capability"`
		Area       string `json:"area"`
	}
	if !body(w, r, &q) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ok := s.allowed(q.Player, q.Capability, q.Area)
	if !s.commit(w, Event{Actor: principal(r).Name, Action: "boundary.check", Player: q.Player, Detail: fmt.Sprintf("%s:%s allowed=%t", q.Area, q.Capability, ok)}) {
		return
	}
	respond(w, 200, Document{"allowed": ok, "reason": map[bool]string{true: "signed capability held", false: "capability, identity or fresh trust unavailable"}[ok]})
}
func (s *Server) actor(r *http.Request, caller string) (string, bool) {
	p := principal(r)
	if !p.Game {
		return p.Authority, true
	}
	v, ok := s.players[caller]
	return v.OperatorAuthority, ok && v.OperatorAuthority != "" && s.identityValid(caller) && s.state.Linked[caller]
}
func (s *Server) delegate(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Player     string `json:"player_name"`
		Capability string `json:"capability"`
		Area       string `json:"area"`
		TTL        int    `json:"ttl_seconds"`
		Caller     string `json:"caller_name,omitempty"`
		Parent     string `json:"parent_id,omitempty"`
	}
	if !body(w, r, &q) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.actor(r, q.Caller)
	if !ok || !s.rootAllows(a, q.Capability, q.Area) || !s.identityValid(q.Player) || !s.state.Linked[q.Player] {
		respond(w, 403, Document{"error": "authority does not hold requested scope or identity is unavailable"})
		return
	}
	if q.TTL < 1 || q.TTL > 3600 || !contains(Protected, q.Capability) || q.Area != s.cfg.Area {
		respond(w, 400, Document{"error": "use a demo capability, configured area and TTL 1..3600"})
		return
	}
	until := time.Now().UTC().Add(time.Duration(q.TTL) * time.Second)
	parent := str(s.roots[a], "attestation_id")
	parentUntil := timestamp(claims(s.roots[a]), "until")
	if q.Parent != "" {
		p, exists := s.state.Grants[q.Parent]
		if !exists || p.Authority != a || p.Capability != q.Capability || p.Area != q.Area || !s.grantValid(p, 0) {
			respond(w, 403, Document{"error": "invalid parent grant"})
			return
		}
		parent = p.ID
		parentUntil = p.Expires
	}
	if until.After(parentUntil) {
		respond(w, 403, Document{"error": "delegation cannot outlive its parent"})
		return
	}
	if len(s.state.Grants) >= 10000 {
		respond(w, 409, Document{"error": "lab grant capacity reached"})
		return
	}
	c := Document{"profile": "worldlab/v1", "kind": "grant", "world": s.cfg.World, "area": q.Area, "capabilities": []string{q.Capability}, "player": q.Player, "parent_id": parent, "until": until.Format(time.RFC3339Nano)}
	d, e := s.net.Issue(r.Context(), s.authorities[a], str(s.identities[q.Player], "subject_id"), s.players[q.Player].PublicKey, c)
	if e != nil {
		respond(w, 503, Document{"error": e.Error()})
		return
	}
	id := str(d, "attestation_id")
	g := Grant{ID: id, Player: q.Player, Authority: a, Capability: q.Capability, Area: q.Area, Parent: parent, Expires: until, Evidence: d}
	s.state.Grants[id] = g
	if !s.commit(w, Event{Actor: a, Action: "capability.delegate", Player: q.Player, GrantID: id}) {
		return
	}
	respond(w, 201, g)
}
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	var q struct {
		ID     string `json:"grant_id"`
		Caller string `json:"caller_name,omitempty"`
	}
	if !body(w, r, &q) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.actor(r, q.Caller)
	g, exists := s.state.Grants[q.ID]
	if !ok || !exists || a != g.Authority {
		respond(w, 403, Document{"error": "only the issuing authority may revoke this grant"})
		return
	}
	g.Revoked = true
	s.state.Grants[q.ID] = g
	if !s.commit(w, Event{Actor: a, Action: "capability.revoke", Player: g.Player, GrantID: g.ID}) {
		return
	}
	authority := s.authorities[a]
	_, e := s.net.Call(r.Context(), authority.URL, "/admin/attestations/"+url.PathEscape(q.ID)+"/revoke", Document{"reason": "worldlab operator revocation"}, &authority)
	if e != nil {
		respond(w, 503, Document{"error": "locally denied; authority publication must be retried", "grant_id": g.ID})
		return
	}
	respond(w, 200, g)
}
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
	if after == ^uint64(0) {
		respond(w, 400, Document{"error": "invalid cursor"})
		return
	}
	events, e := s.db.Audit(after)
	if e != nil {
		respond(w, 503, Document{"error": "audit unavailable"})
		return
	}
	respond(w, 200, events)
}
func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	grants := []Grant{}
	for _, g := range s.state.Grants {
		grants = append(grants, g)
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i].ID < grants[j].ID })
	players := []Document{}
	for _, p := range s.cfg.Players {
		players = append(players, s.identity(p.Name))
	}
	respond(w, 200, Document{"world": s.cfg.World, "ready": s.healthy(), "trust_error": s.lastError, "players": players, "grants": grants})
}
