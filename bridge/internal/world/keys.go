package world

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

func PlayerKey(path string) (string, error) {
	b, e := os.ReadFile(path)
	var key ed25519.PrivateKey
	if os.IsNotExist(e) {
		_, key, e = ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return "", e
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return "", e
		}
		_, e = f.WriteString(base64.StdEncoding.EncodeToString(key.Seed()))
		if e == nil {
			e = f.Sync()
		}
		f.Close()
		if e != nil {
			return "", e
		}
	} else if e != nil {
		return "", e
	} else {
		seed, e := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
		if e != nil || len(seed) != 32 {
			return "", errors.New("invalid player seed")
		}
		key = ed25519.NewKeyFromSeed(seed)
	}
	return base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), nil
}
