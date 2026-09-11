# Playable browser UI

Open **http://127.0.0.1:8789/play/**. A plain URL shows the sign-in screen;
use an invited player's password from its private credential file, or launch:

```powershell
.\scripts\play-web.ps1 -Player alice
```

The launcher starts the server/player if needed and opens an authenticated tab.
The URL fragment is removed immediately; the 30-minute control token stays only
in that tab's session storage. It is not an authority bearer token. A second
login for the same player invalidates the first session and its queued commands.
Do not share the private `.local/browser-url.txt` file.

## Controls

- W A S D / arrows: one-step movement; hold to continue. Losing focus stops input.
- Click in Walk mode: take a step toward that tile.
- Keys 1–5 / hotbar: Walk, Stone, Wood, Glass, Remove.
- Place/remove: click a nearby tile, within four blocks of your player.
- Lobby: return to the safe starting area. Zoom buttons adjust the map.
- Touch screens also have an on-screen directional pad.

The gold court is the protected area east of the lobby. It requires signed
entry/build grants. North/South operator accounts have an authority desk to issue
60-second grants and revoke their own grants. Ordinary players cannot use it.
The activity panel shows acknowledged world actions and recent durable events.

## What runs where

The browser is an isometric canvas controller, not a port of the full Luanti client.
Luanti sends four compact map layers and real player positions every 250 ms.
The bridge validates the player session and queues bounded, short-lived commands.
Luanti performs movement/collision checks and calls its normal place/dig functions,
preserving GenesisMesh and existing world protection. Luanti saves the resulting
world changes. No separate browser copy of the world is created.

The native player relay must stay connected. `play-web.ps1` starts it automatically;
closing it makes that player's browser controls unavailable. The browser area is
x=-8..40, z=-12..12; it edits y=6 only. Full inventory, crafting, combat, arbitrary
terrain travel and first-person camera remain native-client features.

## Security

Browser routes validate the loopback Host and exact same-origin requests, rejecting
cross-site requests and DNS rebinding hosts. CSP restricts scripts/resources to
this local application. Account passwords are read from the private account file;
only token hashes are held by the server. Login, session and operator quotas are
bounded. Sessions expire after 30 minutes and disappear on bridge restart.

The game feed still requires the game-scoped service credential; a browser token
cannot publish frames, impersonate another player or use the native trusted proxy
API. Browser operator requests reuse the existing signed authority operations and
also require the operator's currently valid identity.

## Verification

```powershell
npm ci --ignore-scripts
npm test
.\scripts\test-browser-live.ps1
```

The DOM tests cover rendering, keyboard/input boundaries, session expiry, tile
picking (including raised blocks) and safe text insertion. The live harness checks
actual Luanti movement, placement/removal, court denial, signed grants and revocation.
It creates and removes its own test blocks and revokes its test grants.

During this implementation Chrome reported `ERR_BLOCKED_BY_CLIENT` before loading
the local page. That is a browser/extension policy boundary, not an HTTP error from
the app. No bypass was applied. Visual verification in that browser needs the
operator to resolve the block; automated DOM and direct live-engine checks are
separate evidence.
