# Acceptance evidence — 2026-09-11

Verified on this Windows machine with native Luanti 5.17.0, Mineclonia 0.123.1,
Go 1.26.8 and the existing local GenesisMesh gateway/authority A/B services.

## Automated checks

- Nine Go tests pass on Windows; go vet passes.
- The same suite passes with the race detector in WSL Linux.
- govulncheck v1.8.0 reports no vulnerabilities after upgrading the build
  toolchain from Go 1.26.4 to 1.26.8.
- Coverage includes signed delegation, parent attenuation/cascade, expiry,
  upstream failure, restart/backup, sequence rollback/equivocation, forged
  signatures, corrupt scope, missing state, strict JSON, browser origin rejection,
  quotas and permission leases bounded by root expiry.

These use controlled signed HTTP fixtures. The following checks use live
authorities, the real engine and two actual connected clients.

## Live engine acceptance: 14/14 passed

1. Alice and Bob authenticated and connected.
2. Ungranted player ejected to lobby.
3. Ungranted block placement denied.
4. Signed grant permits entry.
5. Signed grant permits clearing the test block.
6. Actual placement changes air to stone.
7. Revoked building right prevents actual removal.
8. Revoked entry ejects the player.
9. The second authority permits its player.
10. Expiry ejects the player without intervention.
11. Outage test begins with a valid lease.
12. Bridge termination expires the cache and ejects the player.
13. Restart preserves revoked grants.
14. Durable audit contains grant, revoke and expiry events.

Latest run: 2026-09-11 21:33:40 UTC. Private result:
`.local/acceptance.json`. Engine logs corroborate actual stone placement and
protected digging denial. Connection errors during the deliberate outage are
expected. Acceptance grants are revoked or expired after the successful run.

## Recovery

The fresh consistent backup `20260911-233433` was hash-verified and restored
into a new private directory. Its bridge became ready on TCP 28789; its world
started separately on UDP 30001. The restored API retained nine grants, seven
revocations and 45 audit events. Existing source state was not overwritten.
The earlier restore also retained its five grants, four revocations and 22 events.

Go tests explicitly assert revocation sequence floors survive restart. The live
test proves persisted grant denial; it does not claim a live upstream rollback
exercise or an independent external sequence-floor archive.

## Evidence boundaries

Two real clients and gameplay operations were verified through the engine test
mod. No visual screenshot review, recorded video, independent tester onboarding,
long-duration load test or external penetration test is claimed. Hosted CI is
separate from these local results. Normal startup disables the acceptance mod.

Private keys, passwords, tokens, runtime binaries, worlds, state and backups are
ignored by Git. The requirements document's pre-existing user edits are preserved
outside this implementation commit.
