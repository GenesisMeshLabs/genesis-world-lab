# Native Windows setup

## Dependencies

Windows x64, PowerShell 5.1+, Git and Go 1.26.8+ (automatic toolchain download is
supported). Gateway at http://127.0.0.1:8080 with A/B recognized and trust-ready;
A at http://127.0.0.1:20443 and B at http://127.0.0.1:20444 with signed feeds.

Configuration generation reads the adjacent
`../sandbox/nas/docker-fleet/authority-a` and `authority-b` directories:
`genesis.signed.json` and authorized `keys/operator.key`.
The historical directory name does not require Docker.
For another installation, deliberately adapt pinned keys, URLs and operator IDs
before provisioning; discovery alone must not replace a trust anchor.

## Start/stop

```powershell
.\scripts\setup.ps1
.\scripts\start-lab.ps1 -Player alice
.\scripts\start-lab.ps1 -Player bob
Invoke-RestMethod http://127.0.0.1:8789/readyz
.\scripts\stop-lab.ps1
```

Setup verifies pinned archive hashes for Luanti 5.17.0 and Mineclonia 0.123.1,
builds the bridge, creates individual keys/passwords/scoped tokens, and obtains
signed root/player memberships. Only explicit provisioning initializes a new
state store. Existing config/state are preserved. Stop before rebuilding.

Alice/North belong to A; Bob/South to B. North/South are operators.
The launcher uses private password files; unknown accounts cannot register.
Bridge: loopback TCP 8789. Game: loopback UDP 30000, eight slots.
Readiness must report mode=genesismesh and ready=true.
Stop flushes the world gracefully before terminating the bridge.
There is no tunnel, firewall change, boot service or change to shared gateway services.

## Private files

| Path | Data |
| --- | --- |
| .local/config.json | Authority pins, player bindings, token hashes and paths |
| .local/credentials/ | Keys, tokens, passwords and signed memberships |
| .local/state/bridge.bolt | Grants, links, sequence floors and audit |
| .local/world/ | World and SQLite map/player/account state |
| .local/server.conf | Private game token/config |
| .local/logs/ | Game, client and bridge logs |

Setup restricts ACLs to the current user, SYSTEM and local administrators.

## Recovery

```powershell
.\scripts\backup-lab.ps1 -Restart
.\scripts\restore-lab.ps1 -BackupPath .local/backups/<timestamp> -Destination .local/restore-check
```

Backup gracefully stops the world and copies world/state/credentials/config with
SHA-256 hashes. Restore validates hashes and rejects links, escaping paths and
existing destinations. Defaults: bridge 28789, game UDP 30001. It does not launch.

Run the bridge with the restored config and Luanti with restored --world/--config
paths. Verify readiness, saved revocations and audit. Stop clones before rebuilding.

**Never initialize a new store or re-provision to repair readiness after state loss.**
Restore trusted backup data; compare floors against an independently retained
record/current signed feeds. An old backup alone cannot prove the newest floor.
Do not roll back authority snapshots.

## Renewal/rotation

Initial root/player application claims last six days. Before expiry, stop/back up,
obtain operator approval, privately archive only expired signed memberships and
explicitly provision replacements. Keep player keys, state and audit.
Grants rooted in replaced memberships require fresh issuance.

Bearers have no automatic expiry. Add a new named principal/hash, deploy, switch
the caller, remove the old principal and verify old-token HTTP 401. Game rotation also
updates private server.conf and requires graceful game restart. Copied authority
operator keys must follow the authority's supported rotation lifecycle.
