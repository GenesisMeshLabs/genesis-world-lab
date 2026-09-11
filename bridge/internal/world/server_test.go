package world

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fixture struct {
	mu      sync.Mutex
	keys    map[string]ed25519.PrivateKey
	seq     map[string]int
	rev     map[string][]string
	bad     bool
	counter int
	c       Config
	s       *Server
}

func sign(t *testing.T, d Document, a Authority, key ed25519.PrivateKey) Document {
	t.Helper()
	d["issuer_sovereign_id"] = a.Name
	d["issued_by"] = a.KeyID
	delete(d, "signatures")
	b, e := canonical(d)
	if e != nil {
		t.Fatal(e)
	}
	d["signatures"] = []any{map[string]any{"key_id": a.KeyID, "sig": base64.StdEncoding.EncodeToString(ed25519.Sign(key, b))}}
	return d
}
func membership(c Config, subject, key string, claim Document) Document {
	now := time.Now().UTC()
	return Document{"attestation_id": subject, "subject_id": subject, "subject_public_key": key, "status": "active", "roles": []string{"role:client"}, "valid_from": now.Add(-time.Minute).Format(time.RFC3339Nano), "issued_at": now.Format(time.RFC3339Nano), "expires_at": now.Add(time.Hour).Format(time.RFC3339Nano), "claims": claim}
}
func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	f := &fixture{keys: map[string]ed25519.PrivateKey{}, seq: map[string]int{}, rev: map[string][]string{}}
	f.c = Config{Address: "127.0.0.1:8789", StateFile: filepath.Join(dir, "state.bolt"), World: "test", Area: "demo-area", Owner: "a", Principals: []Principal{{Name: "game", TokenHash: HashToken("game"), Game: true}, {Name: "north", TokenHash: HashToken("north"), Authority: "a"}, {Name: "south", TokenHash: HashToken("south"), Authority: "b"}}}
	until := time.Now().UTC().Add(30 * time.Minute).Format(time.RFC3339Nano)
	for _, name := range []string{"a", "b"} {
		pub, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		f.keys[name] = key
		f.seq[name] = 1
		f.rev[name] = []string{}
		a := Authority{Name: name, PublicKey: base64.StdEncoding.EncodeToString(pub), KeyID: "na", OperatorKeyID: "op", OperatorKeyFile: filepath.Join(dir, name+".key"), RootFile: filepath.Join(dir, name+".json")}
		if e = os.WriteFile(a.OperatorKeyFile, []byte(base64.StdEncoding.EncodeToString(key.Seed())), 0600); e != nil {
			t.Fatal(e)
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.bad {
				http.Error(w, "offline", 503)
				return
			}
			switch {
			case r.URL.Path == "/sovereign-revocation-feed":
				d := Document{"feed_id": "feed-" + name, "sequence": f.seq[name], "issued_at": time.Now().UTC().Format(time.RFC3339Nano), "revoked_attestation_ids": f.rev[name], "revocation_reasons": map[string]string{}}
				respond(w, 200, sign(t, d, a, key))
			case r.URL.Path == "/admin/attestations":
				var q Document
				_ = json.NewDecoder(r.Body).Decode(&q)
				if r.Header.Get("X-Admin-Signature") == "" {
					http.Error(w, "missing signature", 401)
					return
				}
				f.counter++
				d := membership(f.c, str(q, "subject_id"), str(q, "subject_public_key"), claims(q))
				d["attestation_id"] = fmt.Sprintf("grant-%d", f.counter)
				respond(w, 201, sign(t, d, a, key))
			case strings.HasSuffix(r.URL.Path, "/revoke"):
				parts := strings.Split(r.URL.Path, "/")
				id := parts[len(parts)-2]
				f.rev[name] = append(f.rev[name], id)
				f.seq[name]++
				respond(w, 200, Document{"status": "revoked"})
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(server.Close)
		a.URL = server.URL
		f.c.Authorities = append(f.c.Authorities, a)
		root := membership(f.c, "root-"+name, a.PublicKey, Document{"profile": "worldlab/v1", "kind": "authority", "world": "test", "area": "demo-area", "capabilities": Protected, "until": until})
		b, _ := json.Marshal(sign(t, root, a, key))
		_ = os.WriteFile(a.RootFile, b, 0600)
	}
	mesh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, Document{"networks": []Document{{"id": "a", "trust_ready": true}, {"id": "b", "trust_ready": true}}, "links": []Document{{"from": "a", "to": "b", "expires_at": until}}})
	}))
	t.Cleanup(mesh.Close)
	f.c.GatewayURL = mesh.URL
	p := Player{Name: "alice", Authority: "b", IdentityFile: filepath.Join(dir, "alice.json"), PublicKey: f.c.Authorities[1].PublicKey}
	f.c.Players = []Player{p}
	d := membership(f.c, "player-alice", p.PublicKey, Document{"profile": "worldlab/v1", "kind": "identity", "world": "test", "player": "alice", "capabilities": Baseline, "until": until})
	b, _ := json.Marshal(sign(t, d, f.c.Authorities[1], f.keys["b"]))
	_ = os.WriteFile(p.IdentityFile, b, 0600)
	if e := CreateStore(f.c.StateFile); e != nil {
		t.Fatal(e)
	}
	var e error
	f.s, e = New(f.c)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { f.s.Close() })
	f.s.Refresh(context.Background())
	if !f.s.healthy() {
		t.Fatal(f.s.lastError)
	}
	return f
}
func call(t *testing.T, s *Server, token, path string, v any) (int, Document) {
	t.Helper()
	method := "GET"
	var b []byte
	if v != nil {
		method = "POST"
		b, _ = json.Marshal(v)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	var d Document
	_ = json.Unmarshal(w.Body.Bytes(), &d)
	return w.Code, d
}
func link(t *testing.T, f *fixture) {
	t.Helper()
	code, d := call(t, f.s, "game", "/v1/identities/link", Document{"player_name": "alice"})
	if code != 200 {
		t.Fatalf("link %d %v", code, d)
	}
}
func grant(t *testing.T, f *fixture, ttl int, parent string) Document {
	t.Helper()
	q := Document{"player_name": "alice", "capability": "region.demo.enter", "area": "demo-area", "ttl_seconds": ttl}
	if parent != "" {
		q["parent_id"] = parent
	}
	code, d := call(t, f.s, "north", "/v1/delegate", q)
	if code != 201 {
		t.Fatalf("grant %d %v", code, d)
	}
	return d
}
func check(t *testing.T, f *fixture, want bool) {
	t.Helper()
	code, d := call(t, f.s, "game", "/v1/check", Document{"player_name": "alice", "capability": "region.demo.enter", "area": "demo-area"})
	if code != 200 || d["allowed"] != want {
		t.Fatalf("check %d %v", code, d)
	}
}
func TestSignedDelegationRevocationRestartAndBackup(t *testing.T) {
	f := newFixture(t)
	link(t, f)
	check(t, f, false)
	g := grant(t, f, 60, "")
	check(t, f, true)
	if code, _ := call(t, f.s, "south", "/v1/revoke", Document{"grant_id": g["id"]}); code != 403 {
		t.Fatal("cross-authority revocation accepted")
	}
	if code, d := call(t, f.s, "north", "/v1/revoke", Document{"grant_id": g["id"]}); code != 200 {
		t.Fatal(code, d)
	}
	check(t, f, false)
	f.s.Refresh(context.Background())
	backup := filepath.Join(t.TempDir(), "backup.bolt")
	if e := f.s.db.Backup(backup); e != nil {
		t.Fatal(e)
	}
	events, e := f.s.db.Audit(0)
	if e != nil || len(events) < 5 {
		t.Fatal("missing audit")
	}
	f.s.Close()
	f.c.StateFile = backup
	f.s, e = New(f.c)
	if e != nil {
		t.Fatal(e)
	}
	f.s.Refresh(context.Background())
	check(t, f, false)
	if seq, _ := sequence(f.s.state.Floors["a"]); seq != 2 {
		t.Fatal("sequence floor lost")
	}
}
func TestParentAttenuationAndCascadingRevoke(t *testing.T) {
	f := newFixture(t)
	link(t, f)
	parent := grant(t, f, 60, "")
	child := grant(t, f, 20, str(parent, "id"))
	if code, _ := call(t, f.s, "north", "/v1/delegate", Document{"player_name": "alice", "capability": "region.demo.enter", "area": "demo-area", "ttl_seconds": 120, "parent_id": parent["id"]}); code != 403 {
		t.Fatal("TTL amplification")
	}
	call(t, f.s, "north", "/v1/revoke", Document{"grant_id": parent["id"]})
	if f.s.grantValid(f.s.state.Grants[str(child, "id")], 0) {
		t.Fatal("child survived parent revocation")
	}
	check(t, f, false)
}
func TestExpiryAndTrustFailureClosePermissions(t *testing.T) {
	f := newFixture(t)
	link(t, f)
	grant(t, f, 1, "")
	check(t, f, true)
	time.Sleep(1100 * time.Millisecond)
	f.s.Refresh(context.Background())
	check(t, f, false)
	events, _ := f.s.db.Audit(0)
	seen := false
	for _, e := range events {
		seen = seen || e.Action == "capability.expire"
	}
	if !seen {
		t.Fatal("expiry not audited")
	}
	grant(t, f, 60, "")
	f.mu.Lock()
	f.bad = true
	f.mu.Unlock()
	f.s.Refresh(context.Background())
	check(t, f, false)
	if code, _ := call(t, f.s, "game", "/v1/identities/alice/capabilities", nil); code != 403 {
		t.Fatal("stale lease issued")
	}
}
func TestRollbackAndSameSequenceMutationRejected(t *testing.T) {
	f := newFixture(t)
	link(t, f)
	f.mu.Lock()
	f.seq["a"] = 3
	f.mu.Unlock()
	f.s.Refresh(context.Background())
	if !f.s.healthy() {
		t.Fatal(f.s.lastError)
	}
	f.mu.Lock()
	f.seq["a"] = 2
	f.mu.Unlock()
	f.s.Refresh(context.Background())
	if f.s.healthy() {
		t.Fatal("rollback accepted")
	}
	f.mu.Lock()
	f.seq["a"] = 3
	f.rev["a"] = []string{"unexpected"}
	f.mu.Unlock()
	f.s.Refresh(context.Background())
	if f.s.healthy() {
		t.Fatal("same sequence mutation accepted")
	}
}
func TestAuthAndInputBoundaries(t *testing.T) {
	f := newFixture(t)
	for _, q := range []struct {
		token, path string
		body        Document
		want        int
	}{{"", "/v1/status", nil, 401}, {"invalid", "/v1/status", nil, 401}, {"north", "/v1/identities/link", Document{"player_name": "alice"}, 403}, {"game", "/v1/identities/link", Document{"player_name": "stranger"}, 403}, {"game", "/v1/delegate", Document{"authority_identity": "system", "player_name": "alice", "ttl_seconds": 60}, 400}, {"game", "/v1/delegate", Document{"caller_name": "alice", "player_name": "alice", "capability": "region.demo.enter", "area": "demo-area", "ttl_seconds": 60}, 403}} {
		if code, d := call(t, f.s, q.token, q.path, q.body); code != q.want {
			t.Fatalf("%s: %d %v", q.path, code, d)
		}
	}
	link(t, f)
	for _, ttl := range []int{-1, 0, 3601, 2147483647} {
		if code, _ := call(t, f.s, "north", "/v1/delegate", Document{"player_name": "alice", "capability": "region.demo.enter", "area": "demo-area", "ttl_seconds": ttl}); code != 400 {
			t.Fatal("bad TTL", ttl, code)
		}
	}
}
func TestCorruptScopeAndMissingStateRefused(t *testing.T) {
	f := newFixture(t)
	link(t, f)
	g := grant(t, f, 60, "")
	v := f.s.state.Grants[str(g, "id")]
	v.Area = "other"
	f.s.state.Grants[v.ID] = v
	if e := f.s.db.Save(f.s.state, nil); e != nil {
		t.Fatal(e)
	}
	f.s.Close()
	if s, e := New(f.c); e == nil {
		s.Close()
		t.Fatal("tampered grant accepted")
	}
	if _, e := OpenStore(filepath.Join(t.TempDir(), "lost.bolt")); e == nil {
		t.Fatal("lost state silently initialized")
	}
}
func TestStrictJSONBrowserOriginAndQuota(t *testing.T) {
	f := newFixture(t)
	for _, raw := range []string{`{"player_name":"alice"} {}`, `{"player_name":"alice","extra":true}`, `{"player_name":"` + strings.Repeat("x", 17000) + `"}`} {
		r := httptest.NewRequest("POST", "/v1/identities/link", strings.NewReader(raw))
		r.Header.Set("Authorization", "Bearer game")
		w := httptest.NewRecorder()
		f.s.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("malformed input accepted: %d", w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/v1/status", nil)
	r.Header.Set("Authorization", "Bearer north")
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	f.s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("browser origin accepted")
	}
	for i := 0; i < 120; i++ {
		if code, _ := call(t, f.s, "north", "/v1/status", nil); code != 200 {
			t.Fatalf("unexpected quota response: %d", code)
		}
	}
	if code, _ := call(t, f.s, "north", "/v1/status", nil); code != 429 {
		t.Fatal("operator quota not enforced")
	}
}

func TestLeaseCannotOutliveRoot(t *testing.T) {
	f := newFixture(t)
	link(t, f)
	grant(t, f, 60, "")
	root := f.s.roots["a"]
	root["expires_at"] = time.Now().Add(300 * time.Millisecond).UTC().Format(time.RFC3339Nano)
	code, d := call(t, f.s, "game", "/v1/identities/alice/capabilities", nil)
	if code != 200 || d["lease_ms"].(float64) > 300 {
		t.Fatalf("lease exceeded root expiry: %d %v", code, d)
	}
}

func TestCanonicalUnicodeAndSignatureMutation(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	a := Authority{Name: "a", KeyID: "na", PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))}
	d := sign(t, Document{"subject": "å😀<&>"}, a, key)
	if e := VerifyDocument(d, a); e != nil {
		t.Fatal(e)
	}
	b, _ := canonical(Document{"s": "å😀<&>"})
	if string(b) != `{"s":"\u00e5\ud83d\ude00<&>"}` {
		t.Fatal(string(b))
	}
	d["subject"] = "attacker"
	if VerifyDocument(d, a) == nil {
		t.Fatal("forged signature accepted")
	}
}
