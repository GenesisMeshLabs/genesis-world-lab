# Playable trust expedition

Open `/play/`, sign in, and use the five expedition cards. The world is the
actual local Luanti server, viewed in 3D with a 2D fallback. Drag to orbit,
scroll or use +/- to zoom, and use Follow to track your avatar. WASD moves;
the hotbar edits nearby real blocks. The identity dock, federation gate,
gold court and recovery archive are navigation landmarks, not extra authorities.

## What you can prove

| Challenge | Evidence | Boundary |
|---|---|---|
| Identity | Verify the player's pinned Ed25519 signature | A configured lab account, not a user-owned wallet |
| Cross-authority access | Another configured authority signs an entry lease; Luanti acknowledges crossing | Two real local authorities; no claim of arbitrary global federation |
| Capability enforcement | Entry and building need separate scoped rights; game confirms the block edit | One protected area in this world |
| Attenuated delegation | A 45-second child under a 90-second parent; parent revocation invalidates child | Same player, narrower lifetime; not a player-to-player transfer |
| Expiry | A 12-second lease expires and authorization becomes false | Engine enforcement continues to use its short permission cache |
| Revocation | Revoke experiment leases, approach the gate and attempt a court edit; game rejects it | Only this player's experiment grants are modified |
| Scope rejection | Ordinary delegate handler rejects `server.admin` in another area with 403 | No privileged grant is issued |
| Tamper rejection | Changing a copied identity claim breaks signature verification | Original identity stays intact |
| Recovery | Consistent database backup loaded into a separate verifier retains denial and floors | Isolated rehearsal, not a restart of the live gateway or world |
| Rollback rejection | Authentic archived signed feed rejected below its stored sequence floor | Older feed is never installed into the live verifier |

The thirteen proof badges persist in the lab database. Issuing a lease does
not count as entering or building: those proofs require an authenticated game
frame. Evidence includes signed identity/grants, current sequence floors and
timestamps, downloadable as JSON. Historical badges are evidence of an earlier
check, not a claim that the player still holds the corresponding permission.
Alice is a shared public demo account, so its history and position are shared.

## Operator activation

Set `"experiments_enabled": true` in the ignored `.local/config.json` and
restart the bridge. Default is false. The authenticated API accepts only
`{"action":"..."}` at `POST /play/api/experiment`; target, issuer, scope,
TTL and parent are fixed by the server. Operators' unrelated grants and trust
policy are not modified. The configured foreign authority must have a valid
root and recognition path for the demo area.

Actions are limited to twelve per player per minute; revoke remains available
for cleanup. At most 240 experiment grant records are retained per player;
new issuance stops at the cap, while revoke, inspection and recovery remain
available. Archiving/resetting a long-lived public lab is an operator task.
Authority signatures and revocation calls use the existing audited handlers.

## Verification

`scripts/test-expedition-live.ps1` runs the full challenge sequence against
connected Alice and the real authorities. It creates and removes its own canary
block, revokes its experiment grants, and writes ignored acceptance evidence.
It deliberately moves Alice, so coordinate with anyone using the shared account.
Run `scripts/test-browser-live.ps1` separately for the existing movement and
editing regression checks. Unit tests cover opt-in, fixed scope, forged fields,
tampering, parent revocation, proof attribution and isolated restore.

This lab does not demonstrate every GenesisMesh feature: production identity
provisioning, operator onboarding at scale, multi-world migration, external
operator governance, secrets-provider integrations, HA failover, PQ protocols
and independent security accreditation require their own environments.

The renderer vendors Three.js 0.186.0 under its MIT license in
`bridge/internal/world/web/vendor/THREE-LICENSE.txt`; no CDN scripts are loaded.
