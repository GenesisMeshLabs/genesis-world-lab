package world

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func webCall(t *testing.T, s *Server, method, path, token, origin, raw string) (int, Document) {
	t.Helper()
	r := httptest.NewRequest(method, "http://127.0.0.1:8789"+path, strings.NewReader(raw))
	r.Host = "127.0.0.1:8789"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	var d Document
	_ = json.Unmarshal(w.Body.Bytes(), &d)
	return w.Code, d
}
func browserFixture(t *testing.T) (*fixture, string) {
	t.Helper()
	f := newFixture(t)
	f.s.cfg.Address = "127.0.0.1:8789"
	link(t, f)
	file := filepath.Join(t.TempDir(), "accounts.json")
	if e := os.WriteFile(file, []byte(`{"alice":{"password":"test-only-password"}}`), 0600); e != nil {
		t.Fatal(e)
	}
	f.s.cfg.BrowserAccountsFile = file
	code, d := webCall(t, f.s, "POST", "/play/api/login", "", "http://127.0.0.1:8789", `{"player":"alice","password":"test-only-password"}`)
	if code != 200 {
		t.Fatalf("login %d %v", code, d)
	}
	return f, str(d, "token")
}
func TestBrowserAuthenticationAndOriginIsolation(t *testing.T) {
	f, token := browserFixture(t)
	for _, q := range []struct {
		method, path, token, origin, raw string
		want                             int
	}{
		{"GET", "/play/api/state", "", "", "", 401},
		{"GET", "/play/api/state", "invalid", "", "", 401},
		{"GET", "/play/api/state", token, "https://attacker.example", "", 403},
		{"GET", "/play/api/state", token, "http://localhost:8789", "", 403},
		{"GET", "/play/api/state", token, "http://127.0.0.1:8789", "", 200},
		{"POST", "/play/api/login", "", "", `{"player":"alice","password":"wrong"}`, 401},
		{"POST", "/play/api/delegate", token, "", `{}`, 403},
		{"POST", "/play/api/command", token, "", `{"action":"move","player":"north","x":1}`, 400},
	} {
		if code, d := webCall(t, f.s, q.method, q.path, q.token, q.origin, q.raw); code != q.want {
			t.Fatalf("%s: %d %v", q.path, code, d)
		}
	}
	r := httptest.NewRequest("GET", "http://attacker.example:8789/play/", nil)
	w := httptest.NewRecorder()
	f.s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("DNS rebinding host accepted")
	}
	if code, _ := webCall(t, f.s, "POST", "/play/api/logout", token, "", `{}`); code != 200 {
		t.Fatal("logout failed")
	}
	if code, _ := webCall(t, f.s, "GET", "/play/api/state", token, "", ""); code != 401 {
		t.Fatal("logged out session accepted")
	}
}
func TestBrowserCommandsBoundToSessionAndFreshEngine(t *testing.T) {
	f, token := browserFixture(t)
	raw := `{"action":"move","x":1,"z":0}`
	if code, _ := webCall(t, f.s, "POST", "/play/api/command", token, "", raw); code != 503 {
		t.Fatal("offline engine accepted command")
	}
	f.s.browser.Updated = time.Now()
	f.s.browser.Frame.Players = []enginePlayer{{Name: "alice"}}
	if code, _ := webCall(t, f.s, "POST", "/play/api/command", token, "", `{"action":"move","x":40}`); code != 400 {
		t.Fatal("teleport accepted")
	}
	if code, _ := webCall(t, f.s, "POST", "/play/api/command", token, "", raw); code != 202 {
		t.Fatal("valid move denied")
	}
	if f.s.browser.Commands[0].Player != "alice" {
		t.Fatal("wrong actor")
	}
	if code, _ := webCall(t, f.s, "POST", "/play/api/command", token, "", raw); code != 429 {
		t.Fatal("command flood accepted")
	}
	// Re-login invalidates old queued controls, not just HTTP authorization.
	_, _ = webCall(t, f.s, "POST", "/play/api/login", "", "", `{"player":"alice","password":"test-only-password"}`)
	frame := engineFrame{Layers: []string{strings.Repeat("s", 1225), strings.Repeat("a", 1225), strings.Repeat("a", 1225), strings.Repeat("a", 1225)}, Players: []enginePlayer{{Name: "alice"}}}
	code, d := call(t, f.s, "game", "/v1/game/frame", frame)
	if code != 200 || len(d["commands"].([]any)) != 0 {
		t.Fatal("replaced session command reached engine")
	}
	if code, _ := call(t, f.s, "north", "/v1/game/frame", frame); code != 403 {
		t.Fatal("operator forged a world frame")
	}
}
func TestBrowserSessionExpiryAndLoginQuota(t *testing.T) {
	f, token := browserFixture(t)
	f.s.browser.Sessions[HashToken(token)].Expires = time.Now().Add(-time.Second)
	if code, _ := webCall(t, f.s, "GET", "/play/api/state", token, "", ""); code != 401 {
		t.Fatal("expired session accepted")
	}
	for i := 0; i < 19; i++ {
		webCall(t, f.s, "POST", "/play/api/login", "", "", `{"player":"alice","password":"wrong"}`)
	}
	if code, _ := webCall(t, f.s, "POST", "/play/api/login", "", "", `{"player":"alice","password":"wrong"}`); code != 429 {
		t.Fatal("login flood accepted")
	}
}
