package world

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"regexp"
)

type Authority struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	PublicKey       string `json:"public_key"`
	KeyID           string `json:"key_id"`
	OperatorKeyFile string `json:"operator_key_file"`
	OperatorKeyID   string `json:"operator_key_id"`
	RootFile        string `json:"root_file"`
}
type Player struct {
	Name              string `json:"name"`
	Authority         string `json:"authority"`
	IdentityFile      string `json:"identity_file"`
	PublicKey         string `json:"public_key"`
	OperatorAuthority string `json:"operator_authority,omitempty"`
}
type Principal struct {
	Name      string `json:"name"`
	TokenHash string `json:"token_hash"`
	Authority string `json:"authority,omitempty"`
	Game      bool   `json:"game,omitempty"`
}
type Config struct {
	Address              string      `json:"address"`
	StateFile            string      `json:"state_file"`
	World                string      `json:"world"`
	Area                 string      `json:"area"`
	GatewayURL           string      `json:"gateway_url"`
	Owner                string      `json:"owner"`
	Authorities          []Authority `json:"authorities"`
	Players              []Player    `json:"players"`
	Principals           []Principal `json:"principals"`
	BrowserAccountsFile  string      `json:"browser_accounts_file,omitempty"`
	BrowserPublicOrigin  string      `json:"browser_public_origin,omitempty"`
	BrowserPublicPlayers []string    `json:"browser_public_players,omitempty"`
	ExperimentsEnabled   bool        `json:"experiments_enabled,omitempty"`
}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
var Baseline = []string{"game.connect", "world.read", "world.build", "world.destroy", "chat.send"}
var Protected = []string{"region.demo.enter", "region.demo.build"}

func LoadConfig(path string) (Config, error) {
	var c Config
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e != nil {
		return c, e
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	if c.BrowserPublicOrigin != "" {
		u, err := url.Parse(c.BrowserPublicOrigin)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || len(c.BrowserPublicPlayers) == 0 {
			return errors.New("public browser origin requires an exact HTTPS origin and explicit players")
		}
		for _, name := range c.BrowserPublicPlayers {
			found := false
			for _, p := range c.Players {
				if p.Name == name && p.OperatorAuthority == "" {
					found = true
				}
			}
			if !found {
				return errors.New("public browser players must be configured non-operator accounts")
			}
		}
	}
	host, _, e := net.SplitHostPort(c.Address)
	if e != nil || (host != "127.0.0.1" && host != "::1") {
		return errors.New("bridge must bind a loopback address")
	}
	if c.World == "" || c.Area == "" || c.StateFile == "" || len(c.Players) == 0 || len(c.Players) > 64 || len(c.Authorities) < 2 || len(c.Authorities) > 8 || len(c.Principals) == 0 {
		return errors.New("incomplete lab configuration")
	}
	names := map[string]bool{}
	for _, a := range c.Authorities {
		if !namePattern.MatchString(a.Name) || names[a.Name] || a.PublicKey == "" || a.RootFile == "" || a.OperatorKeyFile == "" || a.OperatorKeyID == "" {
			return errors.New("invalid authority configuration")
		}
		names[a.Name] = true
		if e := safeURL(a.URL); e != nil {
			return e
		}
	}
	if !names[c.Owner] {
		return errors.New("unknown world owner")
	}
	if e := safeURL(c.GatewayURL); e != nil {
		return e
	}
	players := map[string]bool{}
	for _, p := range c.Players {
		if !namePattern.MatchString(p.Name) || players[p.Name] || !names[p.Authority] || p.IdentityFile == "" || p.PublicKey == "" || (p.OperatorAuthority != "" && !names[p.OperatorAuthority]) {
			return errors.New("invalid player configuration")
		}
		players[p.Name] = true
	}
	hashes := map[string]bool{}
	principalNames := map[string]bool{}
	for _, p := range c.Principals {
		h, e := hex.DecodeString(p.TokenHash)
		if e != nil || len(h) != 32 || hashes[p.TokenHash] || p.Name == "" || principalNames[p.Name] || (!p.Game && !names[p.Authority]) || (p.Game && p.Authority != "") {
			return errors.New("invalid credential scope")
		}
		hashes[p.TokenHash] = true
		principalNames[p.Name] = true
	}
	return nil
}
func safeURL(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("invalid service origin")
	}
	if u.Scheme != "https" && (u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1")) {
		return errors.New("plaintext is allowed only on loopback")
	}
	return nil
}
func HashToken(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
