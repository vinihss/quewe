#!/bin/sh
set -e
echo "Applying quewe migrations..."
for f in /migrations/*.up.sql; do
    [ -e "$f" ] || continue
    echo "  -> $f"
    psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -f "$f"
done

# Seed dev principals (debug auth stub uses string IDs as UUIDs in dev)
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'EOF'
INSERT INTO principals (id, kind, name, active) VALUES
  ('00000000-0000-0000-0000-000000000001', 'user', 'owner-1', true),
  ('00000000-0000-0000-0000-000000000002', 'worker', 'worker-1', true)
ON CONFLICT (id) DO NOTHING;
EOF
echo "Dev principals seeded."
echo "Migrations applied."