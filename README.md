# quewe



Go HTTP service for managing scheduled tasks, executions, distributed workers,
task groups, and observability. Implements Clean Architecture + Hexagonal
(Ports & Adapters) per `AGENTS.md` and the ADRs in `spec/decisions/`.

## Get started

```bash
cp configs/.env.example .env
make run          # starts on :8080
make test         # unit tests
make test-race    # tests with race detector
make arch-check   # validates dependency rules
```

Full documentation is in [`docs/`](docs/README.md). The navigation hub is `docs/README.md`.

| Document | Content |
| --- | --- |
| [Overview](docs/visao-geral.md) | what the service does and how to run it |
| [Architecture](docs/arquitetura.md) | layers, dependencies, and flow |
| [Structure](docs/estrutura.md) | where each type of code lives |
| [Domain](docs/dominio.md) | aggregates and invariants |
| [API](docs/api.md) | routes, errors, and contracts |
| [Data](docs/dados.md) | tables, migrations, and transactions |
| [Configuration](docs/configuracao.md) | environment variables |
| [Operations](docs/operacao.md) | build, deploy, and health |
| [Tests](docs/testes.md) | what to test in each layer |
| [Decisions](docs/decisoes.md) | architectural decisions |

## How it works (integrated components)

```text
                                    ┌──────────────────────────────────────────────┐
                                    │                  quewe service               │
                                    │                                              │
  Client / CLI / AI Gateway         │  ┌──────────────┐      ┌─────────────────┐   │
  ───────────────────────────────►  │  │  HTTP API    │      │    Scheduler    │   │
  POST /api/v1/tasks                │  │  /api/v1/*   │      │ (cron/once tick)│   │
  GET  /api/v1/executions           │  └──────┬───────┘      └────────┬────────┘   │
  POST /api/v1/groups               │         │                       │            │
                                    │         ▼                       ▼            │
                                    │  ┌────────────────────────────────────────┐  │
                                    │  │           Application / Domain         │  │
                                    │  │  tasks · scheduling · consuming        │  │
                                    │  │  leasing · workers · groups · retry    │  │
                                    │  └──────────────────┬─────────────────────┘  │
                                    │                     │                        │
                                    │         ┌───────────┴───────────┐            │
                                    │         ▼                       ▼            │
                                    │  ┌──────────────┐       ┌──────────────┐     │
                                    │  │ Task/Exec    │       │   Broker /   │     │
                                    │  │ Repos        │       │   Queue      │     │
                                    │  └──────┬───────┘       └──────┬───────┘     │
                                    │         │    PostgreSQL        │             │
                                    │         └──────────┬───────────┘             │
                                    │                    ▼                         │
                                    │           ┌─────────────────┐                │
                                    │           │   PostgreSQL    │                │
                                    │           │ tasks/executions│                │
                                    │           │ queue_messages  │                │
                                    │           │ workers/groups  │                │
                                    │           └─────────────────┘                │
                                    │                    ▲                         │
                                    │         ┌──────────┴──────────┐              │
                                    │         ▼                     ▼              │
                                    │  ┌──────────────┐     ┌──────────────┐       │
                                    │  │ Reaper loop  │     │  Worker API  │       │
                                    │  │ (expired     │     │ register/    │       │
                                    │  │  leases)     │     │ consume/     │       │
                                    │  └──────────────┘     │ ack/nack/    │       │
                                    │                       │ heartbeat    │       │
                                    │                       └──────┬───────┘       │
                                    └──────────────────────────────┼───────────────┘
                                                                   │
                                         ┌─────────────────────────┼─────────────────────────┐
                                         │                         │                         │
                                         ▼                         ▼                         ▼
                                  ┌─────────────┐           ┌─────────────┐           ┌─────────────┐
                                  │  Worker A   │           │  Worker B   │           │  Worker N   │
                                  │ capability: │           │ capability: │           │ capability: │
                                  │ system.cmd  │           │ research.web│           │ ...         │
                                  └─────────────┘           └─────────────┘           └─────────────┘
```

**Flow (high level):**
1. A client creates a **task** (`POST /api/v1/tasks`) with a schedule and target capability.
2. The **scheduler** evaluates due schedules and creates **executions** + enqueues **queue messages**.
3. **Workers** register their capabilities, then `consume` messages matching those capabilities (long polling). The broker reserves messages with a lease (`FOR UPDATE SKIP LOCKED`).
4. Workers execute the task, send heartbeats/lease renewals, and report `result`/`ack` or `nack` (retry/dead-letter).
5. The **reaper** periodically reclaims expired leases and requeues/retries as needed.
6. **Groups** aggregate multiple executions under policies (all_completed, minimum_success, etc.).

## Adding workers

> **Pending:** A packaged worker binary/installer for remote machines is not yet shipped in this repository. The broker protocol is implemented and documented below; a reference or production worker implementation (register/consume/heartbeat/result/ack/nack) is still pending.

Workers are external processes that register with quewe, advertise **capabilities**, and process executions via the broker protocol. Routing is capability-based (not worker-ID-based) by default.

### Worker lifecycle

```text
register → heartbeat (loop) → consume (long poll) → execute → result/ack|nack → consume...
                                      │
                                      └── renew lease / heartbeat during long tasks
```

### 1. Register the worker

Register with a unique identity (or let the server derive from the authenticated principal), capabilities, and concurrency.

```bash
curl -X POST http://localhost:8081/api/v1/workers/register \
  -H "X-Debug-Principal-Id: worker-1" \
  -H "Content-Type: application/json" \
  -d '{
    "hostname": "worker-node-01",
    "version": "1.0.0",
    "concurrency": 2,
    "capabilities": ["system.maintenance", "research.web"]
  }'
```

### 2. Consume messages

Poll for work matching the worker's capabilities. Use long polling (`wait_seconds`) to reduce empty responses.

```bash
curl -X POST http://localhost:8081/api/v1/workers/consume \
  -H "X-Debug-Principal-Id: worker-1" \
  -H "Content-Type: application/json" \
  -d '{
    "capabilities": ["system.maintenance", "research.web"],
    "max": 1,
    "wait_seconds": 20
  }'
```

The response includes `execution_id`, `lease_generation`, `payload`, `timeout_seconds`, and attempt metadata. **Handlers must be idempotent** (at-least-once delivery).

### 3. Execute and report result

On success:

```bash
curl -X POST http://localhost:8081/api/v1/executions/{execution_id}/result \
  -H "X-Debug-Principal-Id: worker-1" \
  -H "Content-Type: application/json" \
  -d '{"lease_generation": 1, "success": true}'
```

On failure (retryable or terminal):

```bash
curl -X POST http://localhost:8081/api/v1/executions/{execution_id}/nack \
  -H "X-Debug-Principal-Id: worker-1" \
  -H "Content-Type: application/json" \
  -d '{"lease_generation": 1, "error_code": "exec_failed", "error_message": "timeout", "retryable": true}'
```

### 4. Heartbeat and lease renewal

Send periodic heartbeats and renew leases for long-running executions:

```bash
curl -X POST http://localhost:8081/api/v1/workers/heartbeat \
  -H "X-Debug-Principal-Id: worker-1" \
  -H "Content-Type: application/json" \
  -d '{"metrics": {"active_jobs": 1}}'
```

### 5. Drain (graceful shutdown)

Before stopping a worker, mark it as draining so it stops receiving new work:

```bash
curl -X POST http://localhost:8081/api/v1/workers/{worker_id}/drain \
  -H "X-Debug-Principal-Id: owner-1"
```

### Implementation notes

- **Idempotency:** every handler must tolerate redelivery (same execution delivered again after lease expiry/retry).
- **Lease fencing:** always send the `lease_generation` received from `consume` when calling `ack/nack/result/heartbeat` (execution lease). Stale generations are rejected.
- **Capabilities:** use explicit, least-privilege capabilities (e.g., `system.maintenance`, `research.web`). Never grant admin/shell capabilities to untrusted workers.
- **Auth:** in production, replace the dev stub (`X-Debug-Principal-Id`, config-gated) with real per-worker authentication/authorization (ADR-001).
- **Reference:** see `spec/task-service/12-contratos-de-worker.md`, `spec/decisions/001-autenticacao-e-autorizacao.md`, and `spec/decisions/004-lease-heartbeat-ack-nack.md`.

## Commands

```bash
make run             # starts on :8080
make test            # unit tests
make test-race       # tests with race detector
make lint            # go vet + gofmt
make arch-check      # validates dependency rules
make build           # binary in bin/quewe
make build-cli       # CLI binary in bin/quewe-cli
```

## API Documentation

**Contract (source of truth):** [`contracts/task/openapi.yaml`](../../contracts/task/openapi.yaml) (OpenAPI 3.1, per ADR-002).

Best practices applied:
- **Contract-first/contract-aligned:** the OpenAPI spec is maintained in `contracts/` as the integration contract; handlers/DTOs should stay in sync and changes reviewed like code.
- **OpenAPI 3.1 + JSON Schema:** explicit schemas, tags, security schemes, and the stable error envelope `{ error: { code, message, details? } }`.
- **Single source of truth:** avoid duplicating contract details across ad-hoc docs; reference the spec and keep it versioned.
- **Discoverability:** handler `Register` methods include godoc pointing to the contract/tag. For interactive browsing, load `contracts/task/openapi.yaml` in Swagger UI, Redoc, or Stoplight Elements.

> Note: some response schemas are intentionally permissive (`additionalProperties`) where the implementation evolves; tighten them as DTOs stabilize.

## Endpoints

### Health & Observability (outside `/api/v1`, no authentication)

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/healthz` | Liveness — process is alive |
| `GET` | `/readyz` | Readiness — dependencies ready (DB ping) |
| `GET` | `/metrics` | Prometheus metrics (Go runtime + custom) |

### Tasks (`/api/v1/tasks`)

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/v1/tasks` | Creates a task (cron or once). Accepts `Idempotency-Key`. |
| `GET` | `/api/v1/tasks` | Lists tasks (filter by owner, cursor pagination) |
| `GET` | `/api/v1/tasks/{id}` | Task detail |

### Executions (`/api/v1/executions`)

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/v1/executions` | Lists executions (filter by owner, task, status, period) |
| `GET` | `/api/v1/executions/{id}` | Execution detail |

### Broker (consume, leasing, ACK/NACK) (`/api/v1/executions/{id}`)

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/v1/executions/{id}/heartbeat` | Renews lease (`RenewLease`) |
| `POST` | `/api/v1/executions/{id}/result` | Reports result (success/failure) and finalizes execution |
| `POST` | `/api/v1/executions/{id}/ack` | ACK completion (updates status to `completed`) |
| `POST` | `/api/v1/executions/{id}/nack` | NACK failure (retry or dead-letter) |
| `POST` | `/api/v1/executions/{id}/cancel` | Cancels execution (pending/queued → cancelled; running → cancel_requested) |

### Workers (`/api/v1/workers`)

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/v1/workers/register` | Registers worker with capabilities and metadata |
| `POST` | `/api/v1/workers/heartbeat` | Worker heartbeat (updates last_heartbeat_at + metrics) |
| `POST` | `/api/v1/workers/consume` | Consumes queue messages (long polling) |
| `GET` | `/api/v1/workers` | Lists workers with metadata and capabilities |
| `GET` | `/api/v1/workers/{id}` | Worker detail |
| `POST` | `/api/v1/workers/{id}/drain` | Puts worker in draining mode |

### Task Groups (`/api/v1/groups`)

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/api/v1/groups` | Creates execution group (all_completed, all_settled, minimum_success, first_success) |
| `GET` | `/api/v1/groups/{id}` | Group status and members |
| `POST` | `/api/v1/groups/{id}/evaluate` | Re-evaluates group status |

### Authentication

All `/api/v1/*` endpoints require authentication via `X-Debug-Principal-Id`
(dev stub, config-gated by `APP_ENV=dev` + `DEBUG_AUTH_STUB=true`) or a real
resolver in production. In production, authentication is fail-closed.

### Errors

Standardized error response:

```json
{ "error": { "code": "...", "message": "...", "details": { ... } } }
```

Stable codes: `validation_error`, `not_found`, `unauthorized`, `lease_expired`,
`invalid_transition`, `internal`.

## Generated files

`internal/infrastructure/app/features_gen.go`, `<feature>_gen.go`,
migrations, and most of `docs/` are generated. Do not edit files with the
`DO NOT EDIT` header: regenerate from `.scaffold/`.

## Add a feature

```bash
../scripts/new-feature.sh --service . --spec feature.json
```

The descriptor (see `scripts/feature.schema.json`) defines the aggregate, fields,
use cases, adapters, and tests. The generator creates the layers, wiring,
migration, and feature documentation.
