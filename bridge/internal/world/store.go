package world

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	bolt "go.etcd.io/bbolt"
	"os"
	"time"
)

type Grant struct {
	ID             string    `json:"id"`
	Player         string    `json:"player_name"`
	Authority      string    `json:"authority"`
	Capability     string    `json:"capability"`
	Area           string    `json:"area"`
	Parent         string    `json:"parent_id"`
	Expires        time.Time `json:"expires_at"`
	Revoked        bool      `json:"revoked"`
	ExpiryRecorded bool      `json:"expiry_recorded"`
	Evidence       Document  `json:"evidence"`
}
type State struct {
	Grants map[string]Grant    `json:"grants"`
	Floors map[string]Document `json:"floors"`
	Linked map[string]bool     `json:"linked"`
}
type Event struct {
	ID      uint64    `json:"id"`
	Time    time.Time `json:"time"`
	Actor   string    `json:"actor"`
	Action  string    `json:"action"`
	Player  string    `json:"player_name,omitempty"`
	GrantID string    `json:"grant_id,omitempty"`
	Detail  string    `json:"detail,omitempty"`
}
type Store struct{ db *bolt.DB }

func OpenStore(path string) (*Store, error) {
	info, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	if info.Size() < 16384 {
		return nil, errors.New("missing or truncated state; restore a verified backup")
	}
	return openStore(path)
}
func CreateStore(path string) error {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	f.Close()
	s, e := openStore(path)
	if e != nil {
		return e
	}
	return s.Close()
}
func openStore(path string) (*Store, error) {
	d, e := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if e != nil {
		return nil, e
	}
	s := &Store{d}
	e = d.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{"state", "audit"} {
			if _, e := tx.CreateBucketIfNotExists([]byte(name)); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		d.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Load() (State, error) {
	st := State{Grants: map[string]Grant{}, Floors: map[string]Document{}, Linked: map[string]bool{}}
	e := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("state")).Get([]byte("v1"))
		if b == nil {
			return nil
		}
		return json.Unmarshal(b, &st)
	})
	if st.Grants == nil || st.Floors == nil || st.Linked == nil {
		return st, errors.New("invalid persisted lab state")
	}
	return st, e
}
func (s *Store) Save(st State, event *Event) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, e := json.Marshal(st)
		if e != nil {
			return e
		}
		if e = tx.Bucket([]byte("state")).Put([]byte("v1"), b); e != nil {
			return e
		}
		if event != nil {
			bucket := tx.Bucket([]byte("audit"))
			id, e := bucket.NextSequence()
			if e != nil {
				return e
			}
			event.ID = id
			event.Time = time.Now().UTC()
			key := make([]byte, 8)
			binary.BigEndian.PutUint64(key, id)
			b, e = json.Marshal(event)
			if e != nil {
				return e
			}
			return bucket.Put(key, b)
		}
		return nil
	})
}
func (s *Store) Audit(after uint64) ([]Event, error) {
	out := []Event{}
	e := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte("audit")).Cursor()
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, after+1)
		for k, v := c.Seek(key); k != nil && len(out) < 200; k, v = c.Next() {
			var r Event
			if e := json.Unmarshal(v, &r); e != nil {
				return e
			}
			out = append(out, r)
		}
		return nil
	})
	return out, e
}
func (s *Store) Backup(path string) error {
	return s.db.View(func(tx *bolt.Tx) error { return tx.CopyFile(path, 0600) })
}

// LatestAudit serves a bounded live activity feed without scanning old history.
func (s *Store) LatestAudit(limit int) ([]Event, error) {
	out := []Event{}
	if limit < 1 || limit > 200 {
		limit = 200
	}
	err := s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket([]byte("audit")).Cursor()
		for k, v := cursor.Last(); k != nil && len(out) < limit; k, v = cursor.Prev() {
			var event Event
			if err := json.Unmarshal(v, &event); err != nil {
				return err
			}
			out = append(out, event)
		}
		return nil
	})
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, err
}
