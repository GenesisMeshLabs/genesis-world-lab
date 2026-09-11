package world

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	gm "github.com/GenesisMeshLabs/sdk-go/genesismesh"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf16"
)

type Document map[string]any

func str(d Document, k string) string { s, _ := d[k].(string); return s }
func stringsOf(v any) []string {
	out := []string{}
	if a, ok := v.([]any); ok {
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}
func claims(d Document) Document { m, _ := d["claims"].(map[string]any); return Document(m) }
func timestamp(d Document, k string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, str(d, k))
	return t
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

// Python model canonical JSON uses sorted keys, compact separators and ASCII
// escaping. Preserve original timestamp strings and json.Number values.
func canonical(d Document) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(d); err != nil {
		return nil, err
	}
	s := strings.TrimSuffix(b.String(), "\n")
	var out strings.Builder
	for _, r := range s {
		if r > 127 {
			if r > 0xffff {
				a, b := utf16.EncodeRune(r)
				fmt.Fprintf(&out, "\\u%04x\\u%04x", a, b)
			} else {
				fmt.Fprintf(&out, "\\u%04x", r)
			}
		} else {
			out.WriteRune(r)
		}
	}
	return []byte(out.String()), nil
}
func VerifyDocument(d Document, a Authority) error {
	if str(d, "issuer_sovereign_id") != a.Name || str(d, "issued_by") != a.KeyID {
		return errors.New("issuer mismatch")
	}
	copy := Document{}
	for k, v := range d {
		if k != "signatures" {
			copy[k] = v
		}
	}
	b, e := canonical(copy)
	if e != nil {
		return e
	}
	key, e := base64.StdEncoding.DecodeString(a.PublicKey)
	if e != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("invalid pinned public key")
	}
	sigs, _ := d["signatures"].([]any)
	for _, v := range sigs {
		m, ok := v.(map[string]any)
		if !ok || m["key_id"] != a.KeyID {
			continue
		}
		sig, e := base64.StdEncoding.DecodeString(str(Document(m), "sig"))
		if e == nil && ed25519.Verify(key, b, sig) {
			return nil
		}
	}
	return errors.New("invalid authority signature")
}
func readDocument(path string) (Document, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	return decodeDocument(bytes.NewReader(b))
}
func decodeDocument(r io.Reader) (Document, error) {
	var d Document
	dec := json.NewDecoder(r)
	dec.UseNumber()
	e := dec.Decode(&d)
	return d, e
}

type Network struct{ HTTP *http.Client }

func NewNetwork() *Network {
	return &Network{HTTP: &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}}
}
func (n *Network) Call(ctx context.Context, origin, path string, body Document, a *Authority) (Document, error) {
	method := "GET"
	var data []byte
	var e error
	if body != nil {
		method = "POST"
		data, e = json.Marshal(body)
		if e != nil {
			return nil, e
		}
	}
	r, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(origin, "/")+path, bytes.NewReader(data))
	if e != nil {
		return nil, e
	}
	r.Header.Set("Content-Type", "application/json")
	if a != nil {
		seed, e := os.ReadFile(a.OperatorKeyFile)
		if e != nil {
			return nil, errors.New("operator key unavailable")
		}
		key, _, e := gm.LoadPrivateKey(keyText(string(seed)))
		if e != nil {
			return nil, e
		}
		h, e := gm.BuildAdminHeaders(body, a.OperatorKeyID, key)
		if e != nil {
			return nil, e
		}
		r.Header.Set("X-Admin-Key-Id", h.KeyID)
		r.Header.Set("X-Admin-Signature", h.Signature)
		r.Header.Set("X-Admin-Timestamp", h.Timestamp)
		r.Header.Set("X-Admin-Nonce", h.Nonce)
	}
	res, e := n.HTTP.Do(r)
	if e != nil {
		return nil, errors.New("authority transport unavailable")
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if e != nil || len(b) > 2*1024*1024 {
		return nil, errors.New("authority response too large")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("authority returned HTTP %d", res.StatusCode)
	}
	return decodeDocument(bytes.NewReader(b))
}
func (n *Network) Issue(ctx context.Context, a Authority, subject, key string, c Document) (Document, error) {
	d, e := n.Call(ctx, a.URL, "/admin/attestations", Document{"subject_id": subject, "subject_public_key": key, "roles": []string{"role:client"}, "validity_hours": 168, "claims": c}, &a)
	if e != nil {
		return nil, e
	}
	return d, VerifyDocument(d, a)
}

func keyText(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			b.WriteString(line)
		}
	}
	return b.String()
}
