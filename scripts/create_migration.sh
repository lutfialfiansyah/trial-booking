#!/usr/bin/env bash
set -euo pipefail

# Create a new Goose migration file in migrations/.
# Usage: scripts/create_migration.sh <migration_name>

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
MIGRATIONS_DIR="${PROJECT_ROOT}/migrations"

if [ "$#" -lt 1 ] || [ -z "${1:-}" ]; then
  echo "Error: migration name is required." >&2
  echo "Usage: scripts/create_migration.sh <migration_name>" >&2
  exit 1
fi

MIGRATION_NAME="$1"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"
FILENAME="${TIMESTAMP}_${MIGRATION_NAME}.sql"
FILEPATH="${MIGRATIONS_DIR}/${FILENAME}"

mkdir -p "${MIGRATIONS_DIR}"

if [ -e "${FILEPATH}" ]; then
  echo "Error: migration file already exists: ${FILEPATH}" >&2
  exit 1
fi

cat > "${FILEPATH}" <<'EOF'
-- +goose Up
-- TODO: write up migration

-- +goose Down
-- TODO: write down migration
EOF

echo "${FILEPATH}"
