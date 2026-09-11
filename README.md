# GenesisMesh World Lab

Native Windows Mineclonia with real GenesisMesh signed identities, expiring
delegation, revocation and durable audit. Two independent authorities sign
evidence; a Go bridge verifies it and a Luanti mod enforces short permission leases.

## Run

Windows x64, Go 1.26.8+, and the existing local gateway/authorities A+B are required.
See [setup](docs/setup.md).

```powershell
.\scripts\setup.ps1
.\scripts\start-lab.ps1 -Player alice
.\scripts\start-lab.ps1 -Player bob
```

Game: **127.0.0.1:30000 UDP**. Bridge: <http://127.0.0.1:8789/readyz>.
Try `/gm_demo` in Alice's game, then grant temporary access:

```powershell
.\scripts\demo.ps1 -Action grant -Player alice -Authority authority-b -Seconds 60
.\scripts\demo.ps1 -Action audit
```

Revoke either printed ID with `demo.ps1 -Action revoke -Authority authority-b -GrantId <id>`.
Stop gracefully: `scripts/stop-lab.ps1`. No Docker is used.

[Demo](docs/demo.md) · [Capabilities](docs/capabilities.md) ·
[API](docs/bridge-api.md) · [Security](docs/security.md) ·
[Acceptance](docs/acceptance.md) · [Tracking](TRACKING.md)

## Scope and private data

Implemented: invite-only accounts, signed memberships and grants, parent attenuation,
cascading revocation, persistent floors, in-game HUD/court/operator commands,
transactional audit, backup/restore and live two-client acceptance tests.

Keys, tokens, passwords, worlds, runtime downloads, databases, logs and backups stay
under ignored `.local/`. Never publish a backup: it contains credentials.
The committed server configuration is a credential-free template.

This is an eight-player local lab, not a public hosting service or accreditation
claim. The game is a trusted proxy for custodial player identities.

Original code uses [MIT](LICENSE). Separately downloaded Luanti 5.17.0 and
Mineclonia 0.123.1 retain their upstream engine/game/media licenses; no upstream
assets or runtime binaries are committed.
