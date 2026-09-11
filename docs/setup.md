# Setup

Native setup (no Docker) — run Luanti and the bridge directly on your
machine.

## Prerequisites

- [Luanti](https://www.luanti.org/en/) (engine + server binary)
- [Mineclonia](https://content.luanti.org/packages/ryvnf/mineclonia/) — install via Luanti's ContentDB browser, or clone/place it in Luanti's `games/` directory
- [Go](https://go.dev/dl/) 1.24+ (to build/run the bridge)
- [Git](https://git-scm.com/downloads/win)

## 1. Run the bridge

```bash
cd bridge
go run .
```

By default it listens on `:8080`. Confirm it's up:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

Leave this running in its own terminal (or build a binary and run that:
`go build -o bridge . && ./bridge`).

## 2. Install the mod

Copy (or symlink) `mods/genesismesh/` into your Luanti world's mod
directory, e.g.:

```bash
ln -s "$(pwd)/mods/genesismesh" ~/.minetest/worlds/<your-world>/worldmods/genesismesh
```

(or into Luanti's global `mods/` directory if you want it available to
every world).

## 3. Configure and start the server

Use `server/minetest.conf` as a starting point — copy its settings into
your world/server's own `minetest.conf`, or point Luanti at it directly:

```bash
luantiserver --config /path/to/genesis-world-lab/server/minetest.conf
```

Key settings it sets:

- `default_game = mineclonia`, `load_mod_genesismesh = true`
- `secure.http_mods = genesismesh` — required so the mod is allowed to make
  HTTP calls to the bridge
- `genesismesh.bridge_url = http://127.0.0.1:8080` — assumes the bridge is
  running on the same machine; change this if it's on a different host
- `genesismesh.demo_area_pos1` / `_pos2` — the protected demo area bounds

Connect to the server from a Luanti client at `localhost:30000`.

## Run the bridge's tests

```bash
cd bridge
go test ./...
```

## Stop everything

Ctrl-C the Luanti server process and the bridge process.

## Where logs and audit records live

- Bridge logs: stdout of the `go run .` / bridge process
- GenesisMesh audit trail: `GET /v1/audit` on the bridge (see
  [bridge-api.md](bridge-api.md)) — this is the record referenced in the
  delegation demo (section 4.5 of the requirements)
- Luanti server logs: the Luanti server process's own stdout/log file
  (see Luanti's own docs for its default log location)

## Optional: Docker

A `docker-compose.yml` and `bridge/Dockerfile` are also in the repo if you
ever want a one-command containerized setup instead, but the native path
above is the primary, supported way to run this project.
