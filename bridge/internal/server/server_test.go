package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GenesisMeshLabs/genesis-world-lab/bridge/internal/capability"
)

func postJSON(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return v
}

// TestDemoScenario walks through section 4.5 of the requirements end to end
// against the bridge's HTTP API: join, deny, delegate, allow, revoke, deny,
// audit.
func TestDemoScenario(t *testing.T) {
	s := New()

	// 1. Player joins with a valid identity.
	linkRec := postJSON(t, s, "/v1/identities/link", linkRequest{PlayerName: "bob"})
	if linkRec.Code != http.StatusOK {
		t.Fatalf("link: expected 200, got %d: %s", linkRec.Code, linkRec.Body)
	}
	linkResp := decode[map[string]json.RawMessage](t, linkRec)
	var bobIdentity struct {
		GenesisMeshID string `json:"genesismesh_id"`
	}
	if err := json.Unmarshal(linkResp["identity"], &bobIdentity); err != nil {
		t.Fatal(err)
	}

	// Bootstrap a second authority that already holds region.demo.enter, so
	// it is allowed to delegate that capability onward.
	const authority = "gm-demo:authority-2"
	s.Capabilities.GrantSystem(authority, capability.RegionDemoEnter, "")

	// 2. Player tries to enter the protected area and is denied.
	checkRec := postJSON(t, s, "/v1/check", checkRequest{
		PlayerName: "bob",
		Capability: capability.RegionDemoEnter,
		Area:       "demo-area",
	})
	checkResp := decode[checkResponse](t, checkRec)
	if checkResp.Allowed {
		t.Fatal("expected initial check to be denied")
	}

	// 3. Another authority delegates the required capability for a short time.
	delegateRec := postJSON(t, s, "/v1/delegate", delegateRequest{
		AuthorityIdentity: authority,
		PlayerName:        "bob",
		Capability:        capability.RegionDemoEnter,
		Area:              "demo-area",
		TTLSeconds:        60,
	})
	if delegateRec.Code != http.StatusOK {
		t.Fatalf("delegate: expected 200, got %d: %s", delegateRec.Code, delegateRec.Body)
	}
	grant := decode[capability.Grant](t, delegateRec)

	// 4. The player can now enter.
	checkRec = postJSON(t, s, "/v1/check", checkRequest{
		PlayerName: "bob",
		Capability: capability.RegionDemoEnter,
		Area:       "demo-area",
	})
	checkResp = decode[checkResponse](t, checkRec)
	if !checkResp.Allowed {
		t.Fatal("expected check to be allowed after delegation")
	}

	// 5. The capability is revoked.
	revokeRec := postJSON(t, s, "/v1/revoke", revokeRequest{GrantID: grant.ID, Actor: authority})
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke: expected 200, got %d: %s", revokeRec.Code, revokeRec.Body)
	}

	// 6. The same action is denied again.
	checkRec = postJSON(t, s, "/v1/check", checkRequest{
		PlayerName: "bob",
		Capability: capability.RegionDemoEnter,
		Area:       "demo-area",
	})
	checkResp = decode[checkResponse](t, checkRec)
	if checkResp.Allowed {
		t.Fatal("expected check to be denied after revocation")
	}

	// 7. The audit record shows who granted the right, what was allowed, and when it ended.
	auditReq := httptest.NewRequest(http.MethodGet, "/v1/audit?player=bob", nil)
	auditRec := httptest.NewRecorder()
	s.ServeHTTP(auditRec, auditReq)
	records := decode[[]map[string]any](t, auditRec)

	var sawGrant, sawRevoke bool
	for _, r := range records {
		switch r["action"] {
		case "capability.delegate":
			if r["actor"] != authority {
				t.Fatalf("expected delegate actor %s, got %v", authority, r["actor"])
			}
			sawGrant = true
		case "capability.revoke":
			if r["actor"] != authority {
				t.Fatalf("expected revoke actor %s, got %v", authority, r["actor"])
			}
			sawRevoke = true
		}
	}
	if !sawGrant || !sawRevoke {
		t.Fatalf("expected audit trail to include delegate and revoke, got %+v", records)
	}
	_ = bobIdentity
}

func TestDelegateWithoutHoldingCapabilityIsForbidden(t *testing.T) {
	s := New()
	postJSON(t, s, "/v1/identities/link", linkRequest{PlayerName: "carol"})

	rec := postJSON(t, s, "/v1/delegate", delegateRequest{
		AuthorityIdentity: "gm-demo:carol", // carol only has default capabilities
		PlayerName:        "carol",
		Capability:        capability.ServerAdmin,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body)
	}
}
