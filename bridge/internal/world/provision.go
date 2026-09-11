package world

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Provision creates only missing, explicitly configured lab attestations.
// It never changes an authority's keys, treaties, policy or existing memberships.
func Provision(ctx context.Context, c Config) error {
	if _, e := os.Stat(c.StateFile); os.IsNotExist(e) {
		if e = CreateStore(c.StateFile); e != nil {
			return e
		}
	} else if e != nil {
		return e
	}
	n := NewNetwork()
	authorities := map[string]Authority{}
	create := func(path string, a Authority, subject, key string, claims Document) error {
		if _, e := os.Stat(path); e == nil {
			d, e := readDocument(path)
			if e != nil {
				return e
			}
			return VerifyDocument(d, a)
		} else if !os.IsNotExist(e) {
			return e
		}
		d, e := n.Issue(ctx, a, subject, key, claims)
		if e != nil {
			return fmt.Errorf("provision %s: %w", subject, e)
		}
		b, e := json.MarshalIndent(d, "", "  ")
		if e != nil {
			return e
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		defer f.Close()
		if _, e = f.Write(b); e != nil {
			return e
		}
		return f.Sync()
	}
	until := time.Now().UTC().Add(6 * 24 * time.Hour).Format(time.RFC3339Nano)
	for _, a := range c.Authorities {
		authorities[a.Name] = a
		e := create(a.RootFile, a, "worldlab:"+c.World+":"+a.Name, a.PublicKey, Document{"profile": "worldlab/v1", "kind": "authority", "world": c.World, "area": c.Area, "capabilities": Protected, "until": until})
		if e != nil {
			return e
		}
	}
	for _, p := range c.Players {
		e := create(p.IdentityFile, authorities[p.Authority], "worldlab:"+c.World+":"+p.Name, p.PublicKey, Document{"profile": "worldlab/v1", "kind": "identity", "world": c.World, "player": p.Name, "capabilities": Baseline, "until": until})
		if e != nil {
			return e
		}
	}
	return nil
}
