#!/usr/bin/env bash
#
# trial-booking demo script.
#
# Resets the seed data, verifies the API is running, then exercises every
# assessment case with curl: available seats, duplicate booking, full class,
# payment failure, and the last-seat race.

set -uo pipefail

cd "$(dirname "$0")/.."

BASE_URL="${BASE_URL:-http://localhost:8080}"

section() {
  echo
  echo "================================================================"
  echo "$1"
  echo "================================================================"
}

get() {
  echo
  echo "== $1 =="
  shift
  curl -sS -w "\nHTTP_STATUS:%{http_code}\n" "$@"
}

post_json() {
  echo
  echo "== $1 =="
  url="$2"
  data="$3"
  shift 3
  curl -sS -X POST -H "Content-Type: application/json" -d "$data" -w "\nHTTP_STATUS:%{http_code}\n" "$url"
}

# ----------------------------------------------------------------------------
# 1. Reset seed data (unless explicitly skipped).
# ----------------------------------------------------------------------------
if [ "${SKIP_SEED:-0}" != "1" ]; then
  section "Seeding demo data (make seed)"
  go run ./cmd/seed
fi

# ----------------------------------------------------------------------------
# 2. Verify the API is running.
# ----------------------------------------------------------------------------
section "Checking API health at $BASE_URL/healthz"
if ! curl -sS "$BASE_URL/healthz" >/dev/null 2>&1; then
  echo "API is not running. Start it with: make run-api"
  exit 1
fi
echo "API is up."

# ----------------------------------------------------------------------------
# 3. List classes.
# ----------------------------------------------------------------------------
get "List classes" "$BASE_URL/api/v1/classes"

# ----------------------------------------------------------------------------
# 4. Parent 1 with children (frontend child picker).
# ----------------------------------------------------------------------------
get "Parent 1 and children" "$BASE_URL/api/v1/parents/1"

# ----------------------------------------------------------------------------
# 5. Happy path: pay seeded pending booking 12.
# ----------------------------------------------------------------------------
post_json "Pay booking 12 successfully" \
  "$BASE_URL/api/v1/bookings/12/payment" \
  '{"outcome":"success","provider_ref":"demo_happy_path"}'

# ----------------------------------------------------------------------------
# 6. Duplicate booking attempt (student 1 already confirmed in class 1).
# ----------------------------------------------------------------------------
post_json "Duplicate booking attempt (expect 409 duplicate_booking)" \
  "$BASE_URL/api/v1/bookings" \
  '{"parent_id":1,"student_id":1,"trial_class_id":1}'

# ----------------------------------------------------------------------------
# 7. Full class attempt (class 3 is full).
# ----------------------------------------------------------------------------
post_json "Full class attempt (expect 409 class_full)" \
  "$BASE_URL/api/v1/bookings" \
  '{"parent_id":2,"student_id":5,"trial_class_id":3}'

# ----------------------------------------------------------------------------
# 8. Payment failure case (seeded booking 11).
# ----------------------------------------------------------------------------
get "Seeded payment failure booking 11" "$BASE_URL/api/v1/bookings/11"

# ----------------------------------------------------------------------------
# 9. Last-seat race: class 2 has 3 confirmed and two pending (5 and 6).
# ----------------------------------------------------------------------------
post_json "Race: pay booking 6 first (expect confirmed)" \
  "$BASE_URL/api/v1/bookings/6/payment" \
  '{"outcome":"success","provider_ref":"demo_race_b"}'

post_json "Race: pay booking 5 after seat is gone (expect 409 seat_unavailable)" \
  "$BASE_URL/api/v1/bookings/5/payment" \
  '{"outcome":"success","provider_ref":"demo_race_a"}'

get "Booking 5 after failed race payment" "$BASE_URL/api/v1/bookings/5"

get "Class 2 roster after race" "$BASE_URL/api/v1/admin/classes/2/roster"

echo
echo "Demo finished."
echo "Run \"make seed\" to reset the demo data."
