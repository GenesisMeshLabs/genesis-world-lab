package world

import (
	"testing"
	"time"
)

func TestExpeditionConsentAndScope(t *testing.T) {
	f, token := browserFixture(t)
	request := func(raw string, want int) {
		t.Helper()
		code, d := webCall(t, f.s, "POST", "/play/api/experiment", token, "", raw)
		if code != want {
			t.Fatalf("%s: %d %v", raw, code, d)
		}
	}
	request(`{"action":"passport"}`, 403)
	f.s.cfg.ExperimentsEnabled = true
	request(`{"action":"passport","player":"north"}`, 400)
	request(`{"action":"passport","ttl_seconds":3600}`, 400)
	request(`{"action":"unknown"}`, 400)
	request(`{"action":"identity"}`, 200)
	request(`{"action":"passport"}`, 200)
	e := f.s.state.Expeditions["alice"]
	if len(e.Grants) != 1 {
		t.Fatal("expected one owned lease")
	}
	g := f.s.state.Grants[e.Grants[0]]
	if g.Player != "alice" || g.Authority == f.s.players["alice"].Authority || g.Capability != "region.demo.enter" || time.Until(g.Expires) > 91*time.Second {
		t.Fatal("grant escaped fixed scope")
	}
	if _, ok := e.Proofs["entered"]; ok {
		t.Fatal("grant alone counted as game entry")
	}
	request(`{"action":"scope"}`, 200)
	request(`{"action":"tamper"}`, 200)
	if !f.s.identityValid("alice") {
		t.Fatal("tamper experiment modified live identity")
	}
	request(`{"action":"revoke"}`, 200)
	if f.s.grantValid(f.s.state.Grants[g.ID], 0) {
		t.Fatal("revoked lease remains valid")
	}
	request(`{"action":"recover"}`, 200)
	for _, key := range []string{"identity", "cross_authority", "scope_blocked", "tamper_rejected", "recovered", "rollback_rejected"} {
		if _, ok := e.Proofs[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}

}

func TestExpeditionCascadeAndEngineProof(t *testing.T) {
	f, token := browserFixture(t)
	f.s.cfg.ExperimentsEnabled = true
	run := func(action string) {
		t.Helper()
		if c, d := webCall(t, f.s, "POST", "/play/api/experiment", token, "", `{"action":"`+action+`"}`); c != 200 {
			t.Fatalf("%s %d %v", action, c, d)
		}
	}
	run("delegate")
	e := f.s.state.Expeditions["alice"]
	if len(e.Grants) != 2 {
		t.Fatal("expected delegation pair")
	}
	child := f.s.state.Grants[e.Grants[1]]
	if child.Parent != e.Grants[0] || !f.s.grantValid(child, 0) {
		t.Fatal("invalid child")
	}
	f.s.mu.Lock()
	f.s.observeExpedition(engineFrame{Players: []enginePlayer{{Name: "alice", X: 25, Z: 0}}})
	f.s.mu.Unlock()
	if _, ok := e.Proofs["entered"]; !ok {
		t.Fatal("actual entry missing")
	}
	run("cascade")
	if f.s.grantValid(child, 0) {
		t.Fatal("child survived revoked parent")
	}
	f.s.mu.Lock()
	f.s.observeExpedition(engineFrame{Results: []engineResult{{Player: "someone-else", Action: "place", OK: true, X: 25, Z: 0}, {Player: "alice", Action: "place", OK: true, X: 2, Z: 0}}})
	f.s.mu.Unlock()
	if _, ok := e.Proofs["built"]; ok {
		t.Fatal("unrelated edit counted as court build")
	}
	run("revoke")
	f.s.mu.Lock()
	f.s.observeExpedition(engineFrame{Results: []engineResult{{Player: "alice", Action: "dig", Message: "Build denied", OK: false, X: 25, Z: 0}}})
	f.s.mu.Unlock()
	if _, ok := e.Proofs["revoked_denied"]; !ok {
		t.Fatal("real denial missing")
	}
}
