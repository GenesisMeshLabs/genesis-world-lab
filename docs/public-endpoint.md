# Local server through Cloudflare

Public browser endpoint: https://world.genesismesh.org/play/

This named tunnel forwards only `/` and `/play/` to the loopback bridge on 8789.
All other ingress paths return 404. The application independently rejects internal
API access on the public hostname. Public HTTP redirects to HTTPS and HTTPS
responses carry HSTS. Existing authority/gateway tunnels are separate.

Only Alice is enabled publicly. Operator accounts remain local-only, including
requests made with an otherwise valid local operator browser session. Public
requests must use the exact configured origin; unrelated browser origins fail.
Account passwords and tunnel credentials are kept under ignored `.local/`.

The public host and allowed players are explicit private configuration fields:
`browser_public_origin` and `browser_public_players`. Operator accounts cannot
be placed in the public list. Tunnel configuration is `.local/tunnel.json`;
credentials are `.local/credentials/cloudflare-tunnel.json`.

`scripts/start-lab.ps1 -Player alice` starts the local server, player relay and
the configured tunnel. `scripts/start-tunnel.ps1` can start the tunnel independently.
These are background processes, not a new Windows boot service. The machine,
local game, player relay, bridge and tunnel must keep running for public play.

The operator-selected Alice password is a demonstration credential. It is not
an authority credential and does not grant delegation/revocation privileges.
The private account file is authoritative for native password rotation at startup;
existing account privileges are preserved. Restarting the bridge retires all
previous browser sessions.
