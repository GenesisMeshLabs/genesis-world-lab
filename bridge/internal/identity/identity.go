// Package identity links Luanti player names to GenesisMesh identities.
//
// This is a demo-grade, in-memory implementation. A production bridge would
// replace the Store below with real calls to GenesisMesh identity services,
// and would never hold GenesisMesh private keys outside an approved secret
// store.
package identity

import (
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned when a player has no linked identity yet.
var ErrNotFound = errors.New("identity: player has no linked identity")

// Identity is the GenesisMesh identity linked to a Luanti player.
type Identity struct {
	PlayerName    string    `json:"player_name"`
	GenesisMeshID string    `json:"genesismesh_id"`
	LinkedAt      time.Time `json:"linked_at"`
}

// Store links and looks up identities. Safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	byID map[string]*Identity // keyed by player name
}

// NewStore returns an empty identity store.
func NewStore() *Store {
	return &Store{byID: make(map[string]*Identity)}
}

// Link creates (or returns the existing) identity for a player.
//
// TODO: replace the deterministic local ID with a real GenesisMesh identity
// issued and verified by the GenesisMesh identity service.
func (s *Store) Link(playerName string) *Identity {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.byID[playerName]; ok {
		return existing
	}

	id := &Identity{
		PlayerName:    playerName,
		GenesisMeshID: "gm-demo:" + playerName,
		LinkedAt:      time.Now().UTC(),
	}
	s.byID[playerName] = id
	return id
}

// Get returns the identity linked to a player, if any.
func (s *Store) Get(playerName string) (*Identity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byID[playerName]
	if !ok {
		return nil, ErrNotFound
	}
	return id, nil
}
