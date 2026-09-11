// Package audit records trust decisions (identity links, grants, checks,
// denials, and revocations) so the demo scenario in section 4.5 of the
// requirements can show "who granted the right, what was allowed, and when
// it ended."
package audit

import (
	"sync"
	"time"
)

// Record is one auditable trust decision.
type Record struct {
	ID         int       `json:"id"`
	Time       time.Time `json:"time"`
	Actor      string    `json:"actor"`    // who performed the action (identity or "system")
	Action     string    `json:"action"`   // e.g. "identity.link", "capability.grant", "boundary.check"
	Identity   string    `json:"identity"` // identity the action concerns
	Capability string    `json:"capability,omitempty"`
	Area       string    `json:"area,omitempty"`
	Allowed    *bool     `json:"allowed,omitempty"` // set for boundary checks
	Detail     string    `json:"detail,omitempty"`
}

// Log is an append-only, in-memory audit trail. Safe for concurrent use.
//
// TODO: replace with a durable GenesisMesh audit sink; this in-memory log
// is lost when the bridge restarts.
type Log struct {
	mu      sync.RWMutex
	records []Record
	nextID  int
}

// NewLog returns an empty audit log.
func NewLog() *Log {
	return &Log{}
}

// Append adds a record, stamping it with the current time and next ID.
func (l *Log) Append(r Record) Record {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.nextID++
	r.ID = l.nextID
	r.Time = time.Now().UTC()
	l.records = append(l.records, r)
	return r
}

// All returns every record, oldest first.
func (l *Log) All() []Record {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := make([]Record, len(l.records))
	copy(out, l.records)
	return out
}

// ForIdentity returns every record concerning identity, oldest first.
func (l *Log) ForIdentity(identity string) []Record {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var out []Record
	for _, r := range l.records {
		if r.Identity == identity {
			out = append(out, r)
		}
	}
	return out
}
