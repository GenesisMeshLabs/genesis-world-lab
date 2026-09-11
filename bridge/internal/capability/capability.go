// Package capability implements the small capability model described in
// section 4.4 of the project requirements: named rights that can be
// granted, delegated for a limited time, checked, and revoked.
//
// This is a demo-grade, in-memory implementation intended to make the
// delegation/revocation demo (section 4.5) work end to end. A production
// bridge would replace the Store below with real GenesisMesh capability,
// delegation, and boundary operations.
package capability

import (
	"errors"
	"sync"
	"time"
)

// Well-known capabilities from section 4.4 of the requirements.
const (
	GameConnect     = "game.connect"
	WorldRead       = "world.read"
	WorldBuild      = "world.build"
	WorldDestroy    = "world.destroy"
	ChatSend        = "chat.send"
	RegionDemoEnter = "region.demo.enter"
	RegionDemoBuild = "region.demo.build"
	AgentControl    = "agent.control"
	ServerAdmin     = "server.admin"
)

// DefaultCapabilities are granted automatically once a player's identity is
// linked, matching baseline Mineclonia gameplay (section 4.1).
var DefaultCapabilities = []string{GameConnect, WorldRead, WorldBuild, WorldDestroy, ChatSend}

var (
	// ErrNotHeld is returned when an authority tries to delegate a
	// capability it does not itself hold.
	ErrNotHeld = errors.New("capability: authority does not hold this capability and cannot delegate it")
	// ErrGrantNotFound is returned when revoking an unknown grant.
	ErrGrantNotFound = errors.New("capability: grant not found")
)

// Grant is a single capability held by an identity, optionally scoped to an
// area and optionally time-limited.
type Grant struct {
	ID         string     `json:"id"`
	Identity   string     `json:"identity"`   // GenesisMesh identity holding the grant
	Capability string     `json:"capability"` // e.g. "region.demo.build"
	Area       string     `json:"area,omitempty"`
	GrantedBy  string     `json:"granted_by"` // "system" for bootstrap/default grants
	GrantedAt  time.Time  `json:"granted_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"` // nil = does not expire
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// Active reports whether the grant currently confers its capability.
func (g *Grant) Active(now time.Time) bool {
	if g.RevokedAt != nil && !g.RevokedAt.After(now) {
		return false
	}
	if g.ExpiresAt != nil && !g.ExpiresAt.After(now) {
		return false
	}
	return true
}

// Store holds capability grants for identities. Safe for concurrent use.
type Store struct {
	mu     sync.RWMutex
	grants map[string]*Grant   // by grant ID
	byIden map[string][]string // identity -> grant IDs
	nextID int
}

// NewStore returns an empty capability store.
func NewStore() *Store {
	return &Store{
		grants: make(map[string]*Grant),
		byIden: make(map[string][]string),
	}
}

// GrantDefault gives an identity the baseline gameplay capabilities. It is
// the "system" authority and bypasses the already-held check, matching
// bootstrap of a freshly linked identity.
func (s *Store) GrantDefault(identity string) []*Grant {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []*Grant
	for _, cap := range DefaultCapabilities {
		out = append(out, s.grantLocked(identity, cap, "", "system", nil))
	}
	return out
}

// GrantSystem gives identity a capability directly, as the "system"
// authority. It is used to bootstrap authorities (e.g. the second authority
// in the delegation demo) that must already hold a capability before they
// can delegate it onward.
func (s *Store) GrantSystem(identity, cap, area string) *Grant {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.grantLocked(identity, cap, area, "system", nil)
}

// Delegate grants capability (optionally scoped to area, optionally with a
// ttl) from authority to identity. The authority must already hold an
// active, unscoped-or-covering grant for the same capability — an identity
// must not be able to delegate a right it does not already hold.
func (s *Store) Delegate(authority, identity, cap, area string, ttl time.Duration) (*Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if authority != "system" && !s.hasActiveLocked(authority, cap, area, time.Now()) {
		return nil, ErrNotHeld
	}

	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().UTC().Add(ttl)
		expiresAt = &t
	}
	return s.grantLocked(identity, cap, area, authority, expiresAt), nil
}

// Revoke immediately ends a grant.
func (s *Store) Revoke(grantID string) (*Grant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.grants[grantID]
	if !ok {
		return nil, ErrGrantNotFound
	}
	now := time.Now().UTC()
	g.RevokedAt = &now
	return g, nil
}

// Check reports whether identity currently holds cap, optionally scoped to
// area (an ungapped grant with no area covers every area).
func (s *Store) Check(identity, cap, area string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasActiveLocked(identity, cap, area, time.Now())
}

// Active lists every currently-active grant held by identity.
func (s *Store) Active(identity string) []*Grant {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var out []*Grant
	for _, id := range s.byIden[identity] {
		g := s.grants[id]
		if g.Active(now) {
			out = append(out, g)
		}
	}
	return out
}

func (s *Store) hasActiveLocked(identity, cap, area string, now time.Time) bool {
	for _, id := range s.byIden[identity] {
		g := s.grants[id]
		if g.Capability != cap || !g.Active(now) {
			continue
		}
		if g.Area == "" || g.Area == area {
			return true
		}
	}
	return false
}

func (s *Store) grantLocked(identity, cap, area, grantedBy string, expiresAt *time.Time) *Grant {
	s.nextID++
	g := &Grant{
		ID:         idFor(s.nextID),
		Identity:   identity,
		Capability: cap,
		Area:       area,
		GrantedBy:  grantedBy,
		GrantedAt:  time.Now().UTC(),
		ExpiresAt:  expiresAt,
	}
	s.grants[g.ID] = g
	s.byIden[identity] = append(s.byIden[identity], g.ID)
	return g
}

func idFor(n int) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if n == 0 {
		return "grant-0"
	}
	buf := make([]byte, 0, 8)
	for n > 0 {
		buf = append([]byte{digits[n%36]}, buf...)
		n /= 36
	}
	return "grant-" + string(buf)
}
