# Docker Compose — quewe

Runs quewe with PostgreSQL (pgvector) in containers.

## Usage

```bash
cd services/quewe
docker compose up -d --build
```

The service is available at `http://localhost:8081` (mapped from container port 8080).

## Services

| Service | Port | Description |
| --- | --- | --- |
| `db` | 5432 | PostgreSQL 16 + pgvector |
| `quewe` | 8081 (host) → 8080 (container) | quewe service (Go) |

## Environment variables (quewe)

| Variable | Default | Description |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Listen address |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `APP_ENV` | `dev` | Environment (dev enables debug auth stub) |
| `DEBUG_AUTH_STUB` | `true` | Enables dev-only auth stub |
| `DATABASE_URL` | (auto) | PostgreSQL connection URL |
| `LEASE_DURATION` | `60s` | Lease duration |
| `HEARTBEAT_INTERVAL` | `20s` | Heartbeat interval |
| `REAPER_INTERVAL` | `15s` | Reaper interval |
| `POLL_WAIT_SECONDS` | `20` | Long polling wait time |
| `MAX_RESERVE` | `1` | Max messages per consume |

## Migrations

Migrations in `services/quewe/migrations/*.up.sql` are applied automatically
on first database initialization via `init-db.sh`.

## Stop and clean

```bash
docker compose down          # stops containers
docker compose down -v       # stops and removes volumes (data)
```