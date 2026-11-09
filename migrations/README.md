# Migrations (quewe)

Phase 1 placeholder (forward-only). **No real database in this phase.**

## Policy (ADR-010)

- Migrations in a **separate job**, forward-only, versioned (e.g., `NNNN_description.{up,down}.sql`).
- The application **does not** migrate automatically on boot.
- On boot (when DB is integrated): the application validates the schema version and **fails readiness** if behind the required minimum (or refuses to start if ahead of what is supported), per `spec/decisions/010-aplicacao-de-migrations.md`.
- `--auto-migrate` only with `APP_ENV=dev` (future). Lock via PostgreSQL advisory lock in the migration tool.

## Current state

Migrations `0001_init`, `0002_broker`, `0003_groups` exist and are applied via Docker init script. Migration tool (`migrate up|status`) and `schema_migrations` table integration are pending per ADR-010 decisions.