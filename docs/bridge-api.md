# GenesisMesh Game Bridge API

The bridge (`bridge/`) is an HTTP adapter between the `genesismesh` Luanti
mod and GenesisMesh. This demo build keeps everything in memory; see the
`TODO` comments in `bridge/internal/*` for what a production build must
replace with real GenesisMesh identity, capability, delegation, and audit
calls.

Base URL: `http://<bridge-host>:8080` (default `http://127.0.0.1:8080`).

## `GET /healthz`

Returns `{"status": "ok"}` when the bridge is up.

## `POST /v1/identities/link`

Links a Luanti player name to a GenesisMesh identity, creating it on first
use, and grants the default gameplay capabilities
(`game.connect`, `world.read`, `world.build`, `world.destroy`, `chat.send`).

```json
{ "player_name": "bob" }
```

## `GET /v1/identities/{player}/capabilities`

Returns the linked identity and its currently active capability grants.

## `POST /v1/check`

Live boundary decision for a capability, optionally scoped to an area.

```json
{ "player_name": "bob", "capability": "region.demo.enter", "area": "demo-area" }
```

Returns `{"allowed": false, "reason": "..."}` or `{"allowed": true}`.
An unreachable identity or bridge failure resolves to `allowed: false`
(fail closed).

## `POST /v1/delegate`

Grants a capability from `authority_identity` to a player, optionally
scoped to an area and time-limited. Fails with `403` if the authority does
not itself hold the capability — an identity cannot delegate a right it
does not already hold.

```json
{
  "authority_identity": "gm-demo:authority-2",
  "player_name": "bob",
  "capability": "region.demo.enter",
  "area": "demo-area",
  "ttl_seconds": 120
}
```

## `POST /v1/revoke`

Immediately ends a grant by ID.

```json
{ "grant_id": "grant-3", "actor": "gm-demo:authority-2" }
```

## `GET /v1/audit?player=bob`

Returns the audit trail (optionally filtered to one player's identity):
identity links, grants, boundary checks, and revocations, each with actor,
timestamp, and outcome.
