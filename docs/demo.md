# Running the delegation demo

This walks through section 4.5 of the requirements: a player is denied
entry to a protected area, a second authority delegates the needed
capability, the player gets in, the capability is revoked, and the audit
trail shows the whole sequence.

## Automated version

The full flow is exercised as a test against the bridge's HTTP API:

```bash
cd bridge
go test ./internal/server -run TestDemoScenario -v
```

## Interactive version (server + game client)

1. Start the bridge (`cd bridge && go run .`) and the Luanti server with the mod installed (see [setup.md](setup.md)).
2. Connect two Luanti clients to `localhost:30000` as `alice` and `bob`.
3. Have `bob` walk toward the protected demo area
   (default bounds: `-16,-16,-16` to `16,16,16`, see `server/minetest.conf`).
   He is pushed back with a chat message denying entry.
4. As a player with the `server` privilege who already holds
   `region.demo.enter` (bootstrap one via the bridge API — see below), run:
   ```
   /gm_delegate bob region.demo.enter 120
   ```
5. `bob` walks into the area again — this time he gets in.
6. Revoke it: `/gm_revoke <grant-id>` (the grant ID is printed by step 4).
7. `bob` is denied again on his next attempt (or on the mod's periodic
   capability refresh, whichever comes first).
8. View the full record:
   ```bash
   curl "http://localhost:8080/v1/audit?player=bob"
   ```

### Bootstrapping a second authority

The demo needs an identity that already holds `region.demo.enter` before it
can delegate it onward (an identity cannot delegate a right it does not
hold). For this demo-grade bridge, seed one directly:

```bash
curl -X POST localhost:8080/v1/identities/link -d '{"player_name":"alice"}'
# then grant alice region.demo.enter as "system" via the capability store —
# see bridge/internal/capability.Store.GrantSystem, exposed here only
# through Go code/tests today. A production bridge would instead model
# this as a real GenesisMesh authority identity issued out of band.
```
