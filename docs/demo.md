# Two-authority demo

Start Alice(A) and Bob(B). North/South are A/B operator accounts.

1. Alice: /gm_status then /gm_demo for guidance. Walk east into the court;
   entry is denied and Alice returns to the lobby.
2. Run scripts/demo.ps1 -Action grant -Player alice -Authority authority-b -Seconds 60.
   B signs entry/build rights for A's player.
3. Alice walks east into the gold court and places/removes a block.
4. Revoke building using scripts/demo.ps1 -Action revoke -Authority authority-b
   -GrantId <id>. Editing stops. Revoke entry; player returns to lobby.
5. Grant Bob short-lived rights; wait for automatic expiry denial.
6. scripts/demo.ps1 -Action audit shows link/grant/revoke/expiry/trust and sampled boundary events.

In-game operators use /gm_delegate <player> <capability> <seconds> and
/gm_revoke <grant_id>. /gm_lobby exits the court.

## Live acceptance

```powershell
.\scripts\stop-lab.ps1
.\scripts\start-lab.ps1 -Acceptance -Player alice
.\scripts\start-lab.ps1 -Acceptance -Player bob
# Wait for both clients to finish joining.
.\scripts\test-live.ps1
.\scripts\stop-lab.ps1
.\scripts\start-lab.ps1 -Player alice
```

Tests modify only lab grants/world and restart only the lab bridge.
The test-only engine mod moves real players and invokes actual node placement/digging.
Results: ignored .local/acceptance.json. Normal startup removes the test-control
mod from active world mods. Never publish an acceptance-enabled world.
