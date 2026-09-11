// Package server exposes the GenesisMesh Game Bridge HTTP API consumed by
// the `genesismesh` Luanti mod. See docs/bridge-api.md for the wire format.
package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/GenesisMeshLabs/genesis-world-lab/bridge/internal/audit"
	"github.com/GenesisMeshLabs/genesis-world-lab/bridge/internal/capability"
	"github.com/GenesisMeshLabs/genesis-world-lab/bridge/internal/identity"
)

// Server wires the identity, capability, and audit stores behind an HTTP
// API. It holds no GenesisMesh private keys; those belong in an approved
// secret store used by the real GenesisMesh client this demo stubs out.
type Server struct {
	Identities   *identity.Store
	Capabilities *capability.Store
	Audit        *audit.Log
	mux          *http.ServeMux
}

// New builds a Server with fresh in-memory stores and registers routes.
func New() *Server {
	s := &Server{
		Identities:   identity.NewStore(),
		Capabilities: capability.NewStore(),
		Audit:        audit.NewLog(),
		mux:          http.NewServeMux(),
	}
	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("POST /v1/identities/link", s.handleLink)
	s.mux.HandleFunc("GET /v1/identities/{player}/capabilities", s.handleCapabilities)
	s.mux.HandleFunc("POST /v1/check", s.handleCheck)
	s.mux.HandleFunc("POST /v1/delegate", s.handleDelegate)
	s.mux.HandleFunc("POST /v1/revoke", s.handleRevoke)
	s.mux.HandleFunc("GET /v1/audit", s.handleAudit)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- identity ---

type linkRequest struct {
	PlayerName string `json:"player_name"`
}

func (s *Server) handleLink(w http.ResponseWriter, r *http.Request) {
	var req linkRequest
	if !decodeJSON(w, r, &req) || req.PlayerName == "" {
		writeError(w, http.StatusBadRequest, "player_name is required")
		return
	}

	id := s.Identities.Link(req.PlayerName)
	grants := s.Capabilities.GrantDefault(id.GenesisMeshID)

	s.Audit.Append(audit.Record{
		Actor:    "system",
		Action:   "identity.link",
		Identity: id.GenesisMeshID,
		Detail:   "linked player " + req.PlayerName + " and granted default capabilities",
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"identity":       id,
		"default_grants": grants,
	})
}

// --- capabilities ---

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	player := r.PathValue("player")
	id, err := s.Identities.Get(player)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"identity":     id,
		"capabilities": s.Capabilities.Active(id.GenesisMeshID),
	})
}

// --- boundary checks ---

type checkRequest struct {
	PlayerName string `json:"player_name"`
	Capability string `json:"capability"`
	Area       string `json:"area,omitempty"`
}

type checkResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	var req checkRequest
	if !decodeJSON(w, r, &req) || req.PlayerName == "" || req.Capability == "" {
		writeError(w, http.StatusBadRequest, "player_name and capability are required")
		return
	}

	id, err := s.Identities.Get(req.PlayerName)
	if err != nil {
		writeJSON(w, http.StatusOK, checkResponse{Allowed: false, Reason: "no linked GenesisMesh identity"})
		return
	}

	allowed := s.Capabilities.Check(id.GenesisMeshID, req.Capability, req.Area)
	rec := audit.Record{
		Actor:      id.GenesisMeshID,
		Action:     "boundary.check",
		Identity:   id.GenesisMeshID,
		Capability: req.Capability,
		Area:       req.Area,
		Allowed:    &allowed,
	}
	s.Audit.Append(rec)

	resp := checkResponse{Allowed: allowed}
	if !allowed {
		resp.Reason = "capability " + req.Capability + " not held"
		if req.Area != "" {
			resp.Reason += " for area " + req.Area
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- delegation ---

type delegateRequest struct {
	AuthorityIdentity string `json:"authority_identity"`
	PlayerName        string `json:"player_name"`
	Capability        string `json:"capability"`
	Area              string `json:"area,omitempty"`
	TTLSeconds        int    `json:"ttl_seconds,omitempty"`
}

func (s *Server) handleDelegate(w http.ResponseWriter, r *http.Request) {
	var req delegateRequest
	if !decodeJSON(w, r, &req) || req.AuthorityIdentity == "" || req.PlayerName == "" || req.Capability == "" {
		writeError(w, http.StatusBadRequest, "authority_identity, player_name and capability are required")
		return
	}

	id, err := s.Identities.Get(req.PlayerName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var ttl time.Duration
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}

	grant, err := s.Capabilities.Delegate(req.AuthorityIdentity, id.GenesisMeshID, req.Capability, req.Area, ttl)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	s.Audit.Append(audit.Record{
		Actor:      req.AuthorityIdentity,
		Action:     "capability.delegate",
		Identity:   id.GenesisMeshID,
		Capability: req.Capability,
		Area:       req.Area,
		Detail:     grant.ID,
	})

	writeJSON(w, http.StatusOK, grant)
}

// --- revocation ---

type revokeRequest struct {
	GrantID string `json:"grant_id"`
	Actor   string `json:"actor"`
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	var req revokeRequest
	if !decodeJSON(w, r, &req) || req.GrantID == "" {
		writeError(w, http.StatusBadRequest, "grant_id is required")
		return
	}

	grant, err := s.Capabilities.Revoke(req.GrantID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.Audit.Append(audit.Record{
		Actor:      req.Actor,
		Action:     "capability.revoke",
		Identity:   grant.Identity,
		Capability: grant.Capability,
		Area:       grant.Area,
		Detail:     grant.ID,
	})

	writeJSON(w, http.StatusOK, grant)
}

// --- audit ---

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if player := r.URL.Query().Get("player"); player != "" {
		id, err := s.Identities.Get(player)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s.Audit.ForIdentity(id.GenesisMeshID))
		return
	}
	writeJSON(w, http.StatusOK, s.Audit.All())
}

// --- helpers ---

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
