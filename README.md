# GenesisMesh Luanti Server Lab

A playable Mineclonia (Luanti) server with **GenesisMesh** added as its trust
and authority layer — identity, capabilities, delegation, revocation, and
audit, made visible through normal gameplay.

Full requirements and rationale: [GenesisMesh-Luanti-Server-Requirements.md](GenesisMesh-Luanti-Server-Requirements.md).
Progress against those requirements: [TRACKING.md](TRACKING.md).

This is a GenesisMesh demonstration and test environment, not a commercial
game.

## What's here

| Path | What it is |
| --- | --- |
| `bridge/` | GenesisMesh Game Bridge — a Go HTTP service translating game requests into identity/capability/delegation/audit operations (section 4.3) |
| `mods/genesismesh/` | Luanti server mod that calls the bridge and enforces its decisions in-game (section 4.2) |
| `server/minetest.conf` | Luanti server config for the demo world, including the protected demo area |
| `docker-compose.yml` | Runs the bridge and a Luanti + Mineclonia server together |
| `docs/` | Setup, bridge API reference, capability list, and demo walkthrough |

## Quick start

```bash
docker compose up --build
```

See [docs/setup.md](docs/setup.md) for prerequisites and details, and
[docs/demo.md](docs/demo.md) to run the delegation/revocation demo.

## Capability model

See [docs/capabilities.md](docs/capabilities.md) for the initial capability
list and the rules the bridge enforces (scoping, delegation, revocation,
audit).

## Safety

- GenesisMesh credentials live only in the bridge, never in the Luanti mod
  or the repository.
- The bridge fails closed: if it can't be reached, protected actions are
  denied rather than allowed.
- See section 7 of the requirements doc for the full list of safety
  requirements this project follows.

## Licensing and reuse

| Component | License | Source |
| --- | --- | --- |
| Luanti | LGPL-2.1 (engine), various (assets) | <https://github.com/luanti-org/luanti> |
| Mineclonia | LGPL-3.0 (code), CC-BY-SA-4.0 (media, per-asset) | <https://codeberg.org/mineclonia/mineclonia> |
| `mods/genesismesh` (this repo) | See repository license | — |
| `bridge` (this repo) | See repository license | — |

Check each upstream project's own LICENSE file for the authoritative terms
before redistributing.

## Status

Early scaffold: Phase 1–2 foundations (bridge service with a working
identity/capability/audit model and its test suite, mod skeleton, compose
stack). See [TRACKING.md](TRACKING.md) for what's done and what's next.
