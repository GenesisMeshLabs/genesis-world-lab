# Setup

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (or Docker + Compose on Linux)
- [Go](https://go.dev/dl/) 1.24+ (only needed if you want to build/test the bridge outside Docker)
- [Git](https://git-scm.com/downloads/win)

## Run everything with Docker Compose

```bash
docker compose up --build
```

This starts:

- **bridge** — the GenesisMesh Game Bridge on `localhost:8080`
- **luanti** — a Luanti + Mineclonia server on UDP `localhost:30000`, with the
  `genesismesh` mod mounted in and pointed at the bridge

Check the bridge is healthy:

```bash
curl http://localhost:8080/healthz
```

Connect to the server from a Luanti/Minetest client at `localhost:30000`.

## Run the bridge standalone (for development)

```bash
cd bridge
go run .
```

Run its tests:

```bash
cd bridge
go test ./...
```

## Stop the server

```bash
docker compose down
```

Add `-v` to also remove the persisted world volume.

## Where logs and audit records live

- Bridge logs: `docker compose logs bridge`, or stdout when run with `go run .`
- GenesisMesh audit trail: `GET /v1/audit` on the bridge (see
  [bridge-api.md](bridge-api.md)) — this is the record referenced in the
  delegation demo (section 4.5 of the requirements)
- Luanti server logs: `docker compose logs luanti`
