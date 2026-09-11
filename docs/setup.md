# Setup

Native setup (no Docker) — run Luanti and the bridge directly on your
machine. This is the primary, supported path (see [TRACKING.md](../TRACKING.md)
for why).

Tool links: see section 5 of
[GenesisMesh-Luanti-Server-Requirements.md](../GenesisMesh-Luanti-Server-Requirements.md).

## 1. Prerequisites

| Tool | Version | Check it's installed |
| --- | --- | --- |
| [Go](https://go.dev/dl/) | 1.24+ | `go version` |
| [Luanti](https://www.luanti.org/en/) | recent stable | `luantiserver --version` (or `minetestserver --version` on older installs) |
| [Mineclonia](https://content.luanti.org/packages/ryvnf/mineclonia/) | latest | see step 3 |
| [Git](https://git-scm.com/downloads/win) | any recent | `git --version` |

### Installing Luanti

- **Linux (Debian/Ubuntu):** `sudo apt install luanti` (older distros may
  only have the package under its previous name, `minetest`:
  `sudo apt install minetest`). Building from source is also an option —
  see the [Luanti source repo](https://github.com/luanti-org/luanti) for
  build instructions if your distro's package is too old for Mineclonia.
- **macOS:** `brew install --cask luanti` (or `minetest` if `luanti` isn't
  in your Homebrew tap yet).
- **Windows:** download the installer from the
  [Luanti website](https://www.luanti.org/en/) and run it.

You need the **server** binary specifically (`luantiserver` /
`minetestserver`), which normally ships alongside the regular client
install.

### Installing Mineclonia

Easiest path: launch the Luanti client once, open **Content → Browse
Online Content**, search for "Mineclonia", and install it — this places it
in Luanti's `games/` directory automatically.

Manual alternative:

```bash
git clone https://codeberg.org/mineclonia/mineclonia.git \
  "$(luanti_games_dir)/mineclonia"
```

where `luanti_games_dir` is Luanti's games folder — see "Default
directories" below for where that is on your OS.

## 2. Default directories (per OS)

Luanti's user data directory (holds `mods/`, `games/`, `worlds/`) defaults
to:

| OS | Path |
| --- | --- |
| Linux | `~/.minetest/` |
| macOS | `~/Library/Application Support/minetest/` (some installs use `~/.minetest/`) |
| Windows | `%APPDATA%\Luanti\` (older installs: `%APPDATA%\.minetest\`) |

If you're not sure which your install uses, start the client once, then
check **Settings → All Settings → Paths** in the Luanti menu, or look for
where it just created a `worlds/` folder.

## 3. Get the repo

```bash
git clone https://github.com/GenesisMeshLabs/genesis-world-lab.git
cd genesis-world-lab
```

## 4. Run the bridge

```bash
cd bridge
go run .
```

By default it listens on `:8080`. Override with the `BRIDGE_ADDR` env var,
e.g. `BRIDGE_ADDR=":9090" go run .`. Confirm it's up:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

Leave this running in its own terminal, or:

```bash
go build -o bridge .
./bridge            # foreground
nohup ./bridge > bridge.log 2>&1 &   # background, Linux/macOS
```

Run its test suite anytime with `go test ./...` from `bridge/`.

## 5. Create a world (if you don't have one yet)

Easiest: launch the Luanti client, **New Game → Mineclonia**, name it
(e.g. `genesismesh-demo`), and create it. Quit once the world exists — you
now have a `worlds/<name>/` folder under your Luanti user data directory
(step 2).

## 6. Install the mod

Copy or symlink `mods/genesismesh/` from this repo into your world's mod
folder:

```bash
# Linux/macOS example — adjust the base path per the table in step 2
ln -s "$(pwd)/mods/genesismesh" \
  ~/.minetest/worlds/genesismesh-demo/worldmods/genesismesh
```

On Windows (PowerShell, run as the same user who owns the Luanti data
folder):

```powershell
New-Item -ItemType Junction `
  -Path "$env:APPDATA\Luanti\worlds\genesismesh-demo\worldmods\genesismesh" `
  -Target "C:\path\to\genesis-world-lab\mods\genesismesh"
```

If `worldmods/` doesn't exist yet in your world folder, create it first.
Alternatively, copy (rather than symlink) the folder if you'd rather not
deal with links — just remember to re-copy after pulling repo updates.

## 7. Configure the server

`server/minetest.conf` in this repo is a ready-to-use config. Either point
Luanti at it directly, or merge its settings into your world's own
`minetest.conf`.

**Option A — use it directly:**

```bash
luantiserver --config /path/to/genesis-world-lab/server/minetest.conf \
  --world ~/.minetest/worlds/genesismesh-demo
```

**Option B — merge into your world's config:** copy these lines into
`~/.minetest/worlds/genesismesh-demo/minetest.conf`:

```
default_game = mineclonia
load_mod_genesismesh = true
disallow_empty_password = true
enable_pvp = false
secure.http_mods = genesismesh
genesismesh.bridge_url = http://127.0.0.1:8080
genesismesh.refresh_interval = 5
genesismesh.demo_area_pos1 = -16,-16,-16
genesismesh.demo_area_pos2 = 16,16,16
```

then start with:

```bash
luantiserver --world ~/.minetest/worlds/genesismesh-demo
```

### What each setting does

- `default_game` / `load_mod_genesismesh` — runs Mineclonia with the
  GenesisMesh mod enabled.
- `secure.http_mods = genesismesh` — **required**. Luanti blocks mod HTTP
  access by default; without this the mod can't reach the bridge at all,
  and every protected action fails closed (denied).
- `genesismesh.bridge_url` — where the mod finds the bridge. `127.0.0.1`
  assumes same-machine; use the bridge host's LAN/public address if it
  runs elsewhere, e.g. `genesismesh.bridge_url = http://192.168.1.20:8080`.
- `genesismesh.refresh_interval` — seconds between the mod re-pulling a
  player's capabilities from the bridge (catches revocation/expiry even
  without a fresh boundary check).
- `genesismesh.demo_area_pos1` / `_pos2` — opposite corners of the
  protected demo area, as `x,y,z`.
- `disallow_empty_password` / `enable_pvp = false` — baseline safety per
  section 7 of the requirements (private/invited testers only).

## 8. Start the server and connect

Start it (see step 7), then in another terminal confirm it's listening:

```bash
ss -lun | grep 30000   # Linux; or: lsof -i :30000/udp
```

Connect from a Luanti client: **Join Game** → address `localhost` (or the
server's IP), port `30000`.

If clients are on another machine, open UDP port `30000` on the server's
firewall.

## 9. Verify the loop end to end

1. Join with a test account. The bridge should log an `identity.link` —
   check with:
   ```bash
   curl "http://localhost:8080/v1/identities/<your-name>/capabilities"
   ```
   You should see the default capabilities (`game.connect`, `world.read`,
   `world.build`, `world.destroy`, `chat.send`).
2. Walk into the demo area (default bounds `-16,-16,-16` to `16,16,16`) —
   you should be denied with a chat message.
3. Follow [docs/demo.md](demo.md) to delegate, confirm access, revoke, and
   inspect the audit trail.

## Stopping everything

Ctrl-C the Luanti server process and the bridge process (or `kill` the
backgrounded `./bridge` PID).

## Where logs and audit records live

- Bridge logs: stdout of the `go run .` / bridge process (or `bridge.log`
  if backgrounded as shown in step 4)
- GenesisMesh audit trail: `GET /v1/audit` on the bridge (see
  [bridge-api.md](bridge-api.md)) — this is the record referenced in the
  delegation demo (section 4.5 of the requirements)
- Luanti server logs: printed to its own stdout by default; add
  `--logfile <path>` to `luantiserver` to write to a file instead

## Troubleshooting

- **Mod never links an identity / nothing happens on join.** Check
  `secure.http_mods = genesismesh` is set — Luanti silently disables mod
  HTTP without it, and the mod logs a warning
  (`[genesismesh] HTTP API not available...`) to the server console.
- **Every protected action is denied even after delegating.** Confirm the
  bridge is reachable at the configured `genesismesh.bridge_url` from the
  machine running Luanti (`curl <that url>/healthz`). The mod fails closed
  on any bridge error, by design (section 7).
- **`address already in use` starting the bridge.** Another process is on
  `:8080` — set `BRIDGE_ADDR` to a different port and update
  `genesismesh.bridge_url` to match.
- **Luanti won't start / "game not found".** Mineclonia isn't installed
  where Luanti expects it — recheck step 1's ContentDB install, or that
  the manual clone landed in the right `games/` directory (step 2).
- **Can't connect from a second machine.** Check the server's firewall
  allows inbound UDP `30000`, and that you're using the server's real IP,
  not `localhost`.

## Optional: Docker

A `docker-compose.yml` and `bridge/Dockerfile` are also in the repo if you
ever want a containerized setup instead, but native (above) is the
primary, supported way to run this project and is what's been verified.
