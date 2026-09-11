# Security boundaries

Local invite-only demonstration, not a zero-vulnerability or accreditation claim.

Pinned Ed25519 signatures, issuer revocation, durable local denial, persistent
sequence floors and rejection of rollback/equivocation/revocation removal are
implemented. Normal startup refuses missing/truncated/inconsistent state.
bbolt synchronous transactions persist state and audit together.

Scoped hashed bearers use constant-time comparison, quotas, HTTP deadlines and
bounded requests/responses. No open enrollment, arbitrary actor impersonation,
unsigned system grants or unlimited stale cache. Downloads are hash verified;
private files are ignored and ACL protected.

The game is a trusted identity proxy: its credential can assert configured
operator player names. The bridge holds copied authority operator keys. Before
shared deployment use dedicated narrowly privileged operators. Player keys are
custodial. Administrators can read secrets and modify local data.

Audit is durable, not WORM. No external collector, HA, secret provider, automatic
bearer expiry or database encryption is implemented. Monitor audit/world size;
there is no automatic pruning. Historical grants are capped at 10,000, game players
at 8. Capacity/long-duration performance are not signed off.

Old backups cannot prove the latest floor. Retain an independent floor record and
compare current upstream evidence. Never initialize replacement state to bypass readiness.

```powershell
go -C bridge test ./...
go -C bridge vet ./...
go -C bridge run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

Race tests use Linux/WSL with a C toolchain. Live engine tests cover authorization,
expiry, outage and restart. Minimum Go 1.26.8 is the patched baseline. A clean
vulnerability scan is a dated observation, not proof against undisclosed issues.
