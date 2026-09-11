# Build tracking

Progress against [GenesisMesh-Luanti-Server-Requirements.md](GenesisMesh-Luanti-Server-Requirements.md).
Checked = built and verified in this repo (compiled/tested where possible).
Unchecked items either haven't been started, or are scaffolded but not yet
verified — see the note under each for which.

## Phase 1: Playable base (section 8)

- [x] Docker Compose stack defined for a Luanti + Mineclonia server (`docker-compose.yml`, `server/minetest.conf`)
- [x] Persistent multiplayer world volume configured (`luanti-world` volume in `docker-compose.yml`)
- [x] Protected demo area defined (bounds in `server/minetest.conf`, enforced in `mods/genesismesh/init.lua`)
- [ ] Actually run Luanti + Mineclonia end to end — **needs a real Luanti install/Docker daemon**; this sandbox has no Docker daemon and no Luanti binary, so the stack has not been started or played

## Phase 2: GenesisMesh connection (section 8)

- [x] `genesismesh` Luanti mod created (`mods/genesismesh/`) — Lua syntax-checked with `luac5.1 -p` (LuaJIT/5.1 compatible)
- [x] GenesisMesh Game Bridge created (`bridge/`) — Go service, `go build ./...` and `go vet ./...` clean
- [x] Link test players to GenesisMesh identities — `POST /v1/identities/link`, called from the mod's `on_joinplayer`
- [x] First capability checks applied — default capabilities on link, live `POST /v1/check` for the protected demo area, enforced in the mod's `is_protected` override and join-area `globalstep`
- [ ] Verified against a running Luanti server — **not run in this sandbox** (no Docker daemon, no Luanti binary); only the bridge's own HTTP API is exercised by automated tests

## Phase 3: Delegation demo (section 8, section 4.5)

- [x] Capability delegation requires the delegating authority to already hold the capability (`bridge/internal/capability`: `Store.Delegate` returns `ErrNotHeld` otherwise)
- [x] Full deny → delegate → allow → revoke → deny → audit flow automated and passing:
      `go test ./internal/server -run TestDemoScenario -v` (see `bridge/internal/server/server_test.go`)
      and `go test ./internal/capability -run TestDelegationDemoFlow -v` (expiry variant)
- [x] Audit evidence viewable via `GET /v1/audit` (who granted/revoked what, and when)
- [ ] Same flow demonstrated live against a running Luanti server with real game clients — **not done**, needs Phase 1/2's "actually run" item first
- [ ] Two independent authorities modeled as real, separate GenesisMesh authority identities — currently only bootstrapped in-memory via `Store.GrantSystem`; see `docs/demo.md` note on this being a stand-in until real GenesisMesh authority onboarding exists

## Phase 4: Public-quality demo (section 8)

- [ ] One-command startup process — `docker compose up --build` is written but **unverified**: no Docker daemon in this sandbox to actually run it
- [x] Demo guide written (`docs/demo.md`)
- [ ] Record a short demonstration video
- [ ] Publish the project under the GenesisMeshLabs organization

## Later phases (section 8) — not started

- [ ] AI villagers with GenesisMesh identities and limited capabilities
- [ ] Trust between multiple Luanti servers
- [ ] Organization- or country-themed authorities
- [ ] Simple visual trust and audit dashboard
- [ ] Controlled economy, trading, or land ownership
- [ ] Public test server

## Basic project documentation (section 4.6)

- [x] Short project overview (`README.md`)
- [x] Setup instructions (`docs/setup.md`)
- [x] How to start and stop the server (`docs/setup.md`)
- [x] How to run the demo (`docs/demo.md`)
- [x] Initial capability list (`docs/capabilities.md`)
- [x] Where logs and audit records can be viewed (`docs/setup.md`)
- [x] Licensing and reuse information for each main component (`README.md`)

## Minimum acceptance criteria (section 9)

- [ ] A new tester can start the project using the written guide — guide exists, **unverified end to end** (no Docker daemon here)
- [ ] Two players can join and play — needs a running server
- [x] A player without the required capability is denied access to the demo area — proven by `TestDemoScenario` against the bridge; mod-side enforcement written but not run against real Luanti
- [x] An authorized identity can grant a limited, temporary capability — `POST /v1/delegate` with `ttl_seconds`, tested
- [x] The player is allowed after the grant — tested
- [x] Access stops after revocation or expiry — tested (`TestDelegationDemoFlow` covers both)
- [x] The full change is visible in the audit record — tested
- [x] No GenesisMesh private key is stored in the Lua mod or source repository — bridge holds no keys yet at all (demo-grade, in-memory); mod only calls the bridge over HTTP
- [ ] Normal gameplay remains responsive when GenesisMesh is available — needs a running server to measure
- [x] Failure of the bridge produces a safe and clear result for protected actions — mod's `genesismesh.check` fails closed on bridge errors, and `/v1/check` on an unlinked identity returns `allowed: false`

## Known gaps / next steps

- No real GenesisMesh backend exists yet to integrate with; the bridge's identity/capability/audit stores are demo-grade in-memory stand-ins with `TODO` markers at every point a real GenesisMesh call belongs (`bridge/internal/identity`, `bridge/internal/capability`, `bridge/internal/audit`).
- Nothing in this environment can install/run Luanti or start a Docker daemon, so every "run it and see" item above needs to be verified on real hardware (the ASUS machine per section 6) before being checked off.
