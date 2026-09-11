package capability

import (
	"testing"
	"time"
)

func TestDefaultGrantsAreActive(t *testing.T) {
	s := NewStore()
	s.GrantDefault("gm-demo:alice")

	if !s.Check("gm-demo:alice", GameConnect, "") {
		t.Fatal("expected default game.connect grant to be active")
	}
	if s.Check("gm-demo:alice", RegionDemoEnter, "demo-area") {
		t.Fatal("expected no region.demo.enter grant by default")
	}
}

func TestDelegateRequiresAuthorityToHoldCapability(t *testing.T) {
	s := NewStore()

	// alice does not hold region.demo.enter, so she cannot delegate it.
	if _, err := s.Delegate("gm-demo:alice", "gm-demo:bob", RegionDemoEnter, "demo-area", 0); err != ErrNotHeld {
		t.Fatalf("expected ErrNotHeld, got %v", err)
	}
}

func TestDelegationDemoFlow(t *testing.T) {
	s := NewStore()
	const player = "gm-demo:bob"
	const authority = "gm-demo:authority-2"
	const area = "demo-area"

	// 1. Player tries to enter the protected area and is denied.
	if s.Check(player, RegionDemoEnter, area) {
		t.Fatal("expected denial before delegation")
	}

	// Bootstrap the second authority with the right to delegate entry.
	s.GrantSystem(authority, RegionDemoEnter, "")

	// 2. Authority delegates the capability for a short time.
	grant, err := s.Delegate(authority, player, RegionDemoEnter, area, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected delegate error: %v", err)
	}

	// 3. Player can now enter.
	if !s.Check(player, RegionDemoEnter, area) {
		t.Fatal("expected access after delegation")
	}

	// 4a. Explicit revocation immediately denies again.
	if _, err := s.Revoke(grant.ID); err != nil {
		t.Fatalf("unexpected revoke error: %v", err)
	}
	if s.Check(player, RegionDemoEnter, area) {
		t.Fatal("expected denial after revocation")
	}

	// 4b. A fresh grant that merely expires also denies again.
	grant2, err := s.Delegate(authority, player, RegionDemoEnter, area, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected delegate error: %v", err)
	}
	if !s.Check(player, RegionDemoEnter, area) {
		t.Fatal("expected access right after second delegation")
	}
	time.Sleep(20 * time.Millisecond)
	if s.Check(player, RegionDemoEnter, area) {
		t.Fatal("expected denial after expiry")
	}
	_ = grant2
}
