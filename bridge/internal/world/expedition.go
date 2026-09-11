package world

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Proof struct {
	At     time.Time `json:"at"`
	Detail string    `json:"detail"`
}
type Expedition struct {
	Proofs      map[string]Proof    `json:"proofs"`
	Grants      []string            `json:"grants"`
	Baselines   map[string]Document `json:"baselines"`
	ExpiryGrant string              `json:"expiry_grant,omitempty"`
	LastAction  string              `json:"last_action"`
	Message     string              `json:"message"`
}

func validateFeedAdvance(previous, next Document) error {
	seq, e := sequence(next)
	old, _ := sequence(previous)
	if e != nil || seq < old || (previous != nil && seq == old && !sameRevocations(previous, next)) {
		return errors.New("revocation sequence rollback or equivocation")
	}
	for _, id := range stringsOf(previous["revoked_attestation_ids"]) {
		if !contains(stringsOf(next["revoked_attestation_ids"]), id) {
			return errors.New("revocation removal rejected")
		}
	}
	return nil
}
func (s *Server) expeditionFor(name string) *Expedition {
	if s.state.Expeditions == nil {
		s.state.Expeditions = map[string]*Expedition{}
	}
	e := s.state.Expeditions[name]
	if e == nil {
		e = &Expedition{Proofs: map[string]Proof{}, Grants: []string{}, Baselines: map[string]Document{}}
		s.state.Expeditions[name] = e
		for a, d := range s.state.Floors {
			e.Baselines[a] = d
		}
	}
	return e
}
func prove(e *Expedition, key, detail string) {
	if _, ok := e.Proofs[key]; !ok {
		e.Proofs[key] = Proof{At: time.Now().UTC(), Detail: detail}
	}
}
func (s *Server) expeditionView(name string) Document {
	e := s.state.Expeditions[name]
	if e == nil {
		e = &Expedition{Proofs: map[string]Proof{}, Grants: []string{}, Message: "Start with your identity passport, then test the locked gate."}
	}
	records := []Document{}
	for _, id := range e.Grants {
		g, ok := s.state.Grants[id]
		if !ok {
			continue
		}
		status := "active"
		if g.Revoked {
			status = "revoked"
		} else if !time.Now().Before(g.Expires) {
			status = "expired"
		} else if !s.grantValid(g, 0) {
			status = "denied"
		}
		records = append(records, Document{"id": g.ID, "issuer": g.Authority, "capability": g.Capability, "parent": g.Parent, "expires": g.Expires, "status": status, "evidence": g.Evidence})
	}
	if len(records) > 12 {
		records = records[len(records)-12:]
	}
	floors := map[string]int64{}
	for a, d := range s.state.Floors {
		floors[a], _ = sequence(d)
	}
	return Document{"enabled": s.cfg.ExperimentsEnabled, "proofs": e.Proofs, "message": e.Message, "last_action": e.LastAction, "records": records, "floors": floors, "identity": s.identities[name], "feed_verified": s.healthy()}
}

// Called only for authenticated engine frames. Issuing a grant alone never
// counts as successful entry or building; those badges need world evidence.
func (s *Server) observeExpedition(frame engineFrame) {
	if !s.cfg.ExperimentsEnabled {
		return
	}
	for name, e := range s.state.Expeditions {
		before := len(e.Proofs)
		for _, p := range frame.Players {
			if p.Name == name && p.X >= 20 && p.X <= 36 && p.Z >= -8 && p.Z <= 8 && s.allowed(name, "region.demo.enter", s.cfg.Area) {
				prove(e, "entered", "Luanti confirmed entry while a valid scoped grant was held.")
			}
		}
		for _, r := range frame.Results {
			if r.Player != name {
				continue
			}
			if !r.OK && strings.HasPrefix(r.Message, "Entry denied") {
				prove(e, "gate_denied", "The live gate denied entry without a valid capability.")
			}
			if r.OK && r.Action == "place" && r.X >= 20 && r.X <= 36 && r.Z >= -8 && r.Z <= 8 {
				prove(e, "built", "Luanti placed a real block inside the protected court.")
			}
			if !r.OK && (r.Action == "place" || r.Action == "dig") && strings.Contains(r.Message, "Build denied") && (e.LastAction == "revoke" || e.LastAction == "cascade") {
				prove(e, "revoked_denied", "The live game rejected editing after revocation.")
			}
		}
		if g, ok := s.state.Grants[e.ExpiryGrant]; ok && !g.Revoked && !time.Now().Before(g.Expires) && !s.allowed(name, "region.demo.enter", s.cfg.Area) {
			prove(e, "expired", "The short lease expired and entry authorization became false.")
		}
		if len(e.Proofs) != before {
			if err := s.db.Save(s.state, &Event{Actor: name, Player: name, Action: "experiment.world-proof"}); err != nil {
				s.failed = true
			}
		}
	}
}

type operationReply struct {
	bytes.Buffer
	header http.Header
	status int
}

func (r *operationReply) Header() http.Header  { return r.header }
func (r *operationReply) WriteHeader(code int) { r.status = code }

// Invoke the ordinary audited authority handler with server-fixed arguments.
// No actor, target, scope, TTL, parent or grant ID is accepted from the player.
func (s *Server) expeditionOperation(ctx context.Context, authority, path string, body Document) (Grant, int, error) {
	raw, _ := json.Marshal(body)
	r, _ := http.NewRequestWithContext(context.WithValue(ctx, principalKey{}, Principal{Name: "experiment", Authority: authority}), "POST", path, bytes.NewReader(raw))
	out := &operationReply{header: http.Header{}}
	if path == "/v1/delegate" {
		s.delegate(out, r)
	} else {
		s.revoke(out, r)
	}
	if out.status < 200 || out.status >= 300 {
		var d Document
		_ = json.Unmarshal(out.Bytes(), &d)
		return Grant{}, out.status, errors.New(str(d, "error"))
	}
	var g Grant
	err := json.Unmarshal(out.Bytes(), &g)
	return g, out.status, err
}
func (s *Server) experiment(w http.ResponseWriter, r *http.Request, name string) {
	var q struct {
		Action string `json:"action"`
	}
	if !body(w, r, &q) {
		return
	}
	if !contains([]string{"identity", "passport", "build", "delegate", "cascade", "revoke", "expiry", "scope", "tamper", "recover"}, q.Action) {
		respond(w, 400, Document{"error": "Unknown experiment"})
		return
	}
	if !s.experimentMu.TryLock() {
		respond(w, 409, Document{"error": "Another experiment is finishing; try again shortly."})
		return
	}
	defer s.experimentMu.Unlock()
	s.mu.Lock()
	if !s.cfg.ExperimentsEnabled || !s.identityValid(name) || !s.state.Linked[name] {
		s.mu.Unlock()
		respond(w, 403, Document{"error": "Experiments require an enabled lab and a linked, valid player."})
		return
	}
	rate := s.rates["experiment:"+name]
	if time.Since(rate.at) >= time.Minute {
		rate = rateZero()
	}
	rate.n++
	s.rates["experiment:"+name] = rate
	e := s.expeditionFor(name)
	issuing := contains([]string{"passport", "build", "delegate", "expiry"}, q.Action)
	if (rate.n > 12 && q.Action != "revoke") || (issuing && len(e.Grants) >= 239) {
		s.mu.Unlock()
		respond(w, 429, Document{"error": "Experiment budget reached. Wait a minute, or ask the operator to archive this run."})
		return
	}
	sponsor := ""
	for _, a := range s.cfg.Authorities {
		if a.Name != s.players[name].Authority {
			sponsor = a.Name
			break
		}
	}
	ids := append([]string{}, e.Grants...)
	s.mu.Unlock()
	fail := func(err error) { respond(w, 503, Document{"error": err.Error()}) }
	record := func(key, detail string) { s.mu.Lock(); defer s.mu.Unlock(); prove(e, key, detail) }
	issue := func(cap string, ttl int, parent string) (Grant, error) {
		g, _, err := s.expeditionOperation(r.Context(), sponsor, "/v1/delegate", Document{"player_name": name, "capability": cap, "area": s.cfg.Area, "ttl_seconds": ttl, "parent_id": parent})
		if err == nil {
			s.mu.Lock()
			e.Grants = append(e.Grants, g.ID)
			if err = s.db.Save(s.state, &Event{Actor: name, Player: name, Action: "experiment.grant", GrantID: g.ID}); err != nil {
				s.failed = true
			}
			s.mu.Unlock()
		}
		return g, err
	}
	revoke := func(id string) error {
		s.mu.Lock()
		g, ok := s.state.Grants[id]
		s.mu.Unlock()
		if !ok {
			return errors.New("Experiment grant missing")
		}
		_, _, err := s.expeditionOperation(r.Context(), g.Authority, "/v1/revoke", Document{"grant_id": id})
		return err
	}
	clear := func() error {
		for _, id := range ids {
			s.mu.Lock()
			g := s.state.Grants[id]
			needed := !g.Revoked && time.Now().Before(g.Expires)
			s.mu.Unlock()
			if needed {
				if err := revoke(id); err != nil {
					return err
				}
			}
		}
		return nil
	}
	message := ""
	switch q.Action {
	case "identity":
		s.mu.Lock()
		err := VerifyDocument(s.identities[name], s.authorities[s.players[name].Authority])
		s.mu.Unlock()
		if err != nil {
			fail(err)
			return
		}
		record("identity", "Player membership signature verified against the pinned issuer key.")
		message = "Identity verified. Walk to the federation gate and try entering without a grant."
	case "passport":
		if err := clear(); err != nil {
			fail(err)
			return
		}
		g, err := issue("region.demo.enter", 90, "")
		if err != nil {
			fail(err)
			return
		}
		record("cross_authority", fmt.Sprintf("%s signed entry for %s's player. Grant %s", sponsor, s.players[name].Authority, g.ID))
		message = "Cross-authority passport issued for 90 seconds. Enter the gold court; building remains locked."
	case "build":
		if _, err := issue("region.demo.build", 60, ""); err != nil {
			fail(err)
			return
		}
		message = "Building lease issued for 60 seconds. Enter the court and place a real block."
	case "delegate":
		if err := clear(); err != nil {
			fail(err)
			return
		}
		parent, err := issue("region.demo.enter", 90, "")
		if err != nil {
			fail(err)
			return
		}
		child, err := issue("region.demo.enter", 45, parent.ID)
		if err != nil {
			fail(err)
			return
		}
		record("delegation", fmt.Sprintf("Child %s is bound to parent %s, with a shorter expiry and identical scope.", child.ID, parent.ID))
		message = "A 45-second child lease now depends on a 90-second parent. Inspect the chain, then revoke its parent."
	case "cascade":
		parentID := ""
		childID := ""
		s.mu.Lock()
		for _, id := range ids {
			g := s.state.Grants[id]
			if _, ok := s.state.Grants[g.Parent]; ok && !g.Revoked && time.Now().Before(g.Expires) {
				parentID = g.Parent
				childID = id
			}
		}
		s.mu.Unlock()
		if parentID == "" {
			respond(w, 409, Document{"error": "Create a delegated lease first."})
			return
		}
		if err := revoke(parentID); err != nil {
			fail(err)
			return
		}
		s.mu.Lock()
		denied := !s.grantValid(s.state.Grants[childID], 0)
		s.mu.Unlock()
		if denied {
			record("cascade_revoked", "Revoking the parent denied the still-unexpired child without separately revoking it.")
		}
		message = "Parent revoked. Its child is denied immediately by the authorization chain."
	case "revoke":
		if err := clear(); err != nil {
			fail(err)
			return
		}
		message = "Your experiment grants were revoked locally and published to their issuers. Try editing inside the court to prove denial."
	case "expiry":
		if err := clear(); err != nil {
			fail(err)
			return
		}
		g, err := issue("region.demo.enter", 12, "")
		if err != nil {
			fail(err)
			return
		}
		s.mu.Lock()
		e.ExpiryGrant = g.ID
		s.mu.Unlock()
		message = "A 12-second entry lease is ticking. Watch it expire; no revoke command is needed."
	case "scope":
		_, code, _ := s.expeditionOperation(r.Context(), sponsor, "/v1/delegate", Document{"player_name": name, "capability": "server.admin", "area": "other-world", "ttl_seconds": 3600})
		if code != 403 {
			fail(errors.New("Unexpected scope-test result"))
			return
		}
		record("scope_blocked", "The ordinary authority handler rejected server.admin in another area (HTTP 403).")
		message = "Privilege escalation rejected: a demo sponsor cannot grant server.admin or rights in another area."
	case "tamper":
		s.mu.Lock()
		d := s.identities[name]
		a := s.authorities[s.players[name].Authority]
		raw, _ := json.Marshal(d)
		s.mu.Unlock()
		var altered Document
		_ = json.Unmarshal(raw, &altered)
		claims(altered)["world"] = "forged-world"
		if VerifyDocument(altered, a) == nil {
			fail(errors.New("Tamper rejection failed"))
			return
		}
		record("tamper_rejected", "Changing the signed world claim invalidated the Ed25519 signature. The original identity was not changed.")
		message = "Tampered evidence rejected. The check used an altered copy, preserving your real identity."
	case "recover":
		if err := s.rehearseRecovery(name); err != nil {
			respond(w, 409, Document{"error": err.Error()})
			return
		}
		message = "An isolated consistent backup retained revocations and sequence floors. The live world was not restarted."
	}
	s.mu.Lock()
	e.LastAction = q.Action
	e.Message = message
	ok := s.commit(w, Event{Actor: name, Player: name, Action: "experiment." + q.Action})
	s.mu.Unlock()
	if ok {
		respond(w, 200, Document{"message": message})
	}
}
func (s *Server) rehearseRecovery(name string) error {
	// Capture the freshly published revocation before taking the backup.
	s.Refresh(context.Background())
	s.mu.Lock()
	e := s.expeditionFor(name)
	revokedID := ""
	for _, id := range e.Grants {
		if s.state.Grants[id].Revoked {
			revokedID = id
			break
		}
	}
	if revokedID == "" {
		s.mu.Unlock()
		return errors.New("Revoke an experiment lease before testing recovery.")
	}
	path := filepath.Join(filepath.Dir(s.cfg.StateFile), fmt.Sprintf("recovery-%d.bolt", time.Now().UnixNano()))
	err := s.db.Backup(path)
	c := s.cfg
	c.StateFile = path
	floors := map[string]int64{}
	for a, d := range s.state.Floors {
		floors[a], _ = sequence(d)
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	defer os.Remove(path)
	copyServer, err := New(c)
	if err != nil {
		return err
	}
	defer copyServer.Close()
	copyServer.Refresh(context.Background())
	if !copyServer.healthy() {
		return errors.New("Restored copy could not verify fresh upstream trust")
	}
	for a, minimum := range floors {
		seq, _ := sequence(copyServer.state.Floors[a])
		if seq < minimum {
			return errors.New("Restored sequence floor decreased")
		}
	}
	if !copyServer.state.Grants[revokedID].Revoked || copyServer.grantValid(copyServer.state.Grants[revokedID], 0) {
		return errors.New("Restored grant did not remain denied")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prove(e, "recovered", "A consistent isolated database copy retained signed sequence floors and denied a revoked grant after reload.")
	for a, old := range e.Baselines {
		oldSeq, _ := sequence(old)
		currentSeq, _ := sequence(s.state.Floors[a])
		if oldSeq < currentSeq && VerifyDocument(old, s.authorities[a]) == nil && validateFeedAdvance(s.state.Floors[a], old) != nil {
			prove(e, "rollback_rejected", fmt.Sprintf("An authentic archived feed at %d was rejected below %s's held floor %d; live trust was unchanged.", oldSeq, a, currentSeq))
		}
	}
	return nil
}
