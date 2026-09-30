#!/bin/sh
set -e

echo "[entrypoint] Waiting for database at ${DB_HOST}:${DB_PORT}..."
until pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$POSTGRES_USER" >/dev/null 2>&1; do
  echo "[entrypoint] database not ready yet, retrying in 1s..."
  sleep 1
done
echo "[entrypoint] database is ready."

echo "[entrypoint] Running migrations..."
goose -dir /app/migrations postgres "$DATABASE_URL" up

if [ "$AUTO_SEED" = "true" ]; then
  echo "[entrypoint] Seeding demo data..."
  /app/seed
else
  echo "[entrypoint] AUTO_SEED is not true, skipping seed."
fi

echo "[entrypoint] Starting API..."
exec /app/api
