# Trial Booking Reliability — API + Frontend

A backend-led trial class booking system. Parents book their children into trial
classes, pay (mock, synchronous), and staff view a confirmed roster. The core
focus is **correctness under concurrency** — proving that the last seat is never
double-booked and a class is never overbooked.

---

## 1. How to Run

### Option A: Docker

```bash
docker compose up --build
```

This single command does everything:

1. Starts PostgreSQL 16.
2. Waits for Postgres to become healthy.
3. Runs Goose migrations automatically.
4. Seeds deterministic demo data (`AUTO_SEED=true` by default).
5. Starts the API on `http://localhost:8080`.

No Go toolchain, no Goose, no manual migration step is required.

Reset everything (including the data volume):

```bash
docker compose down -v --remove-orphans
```

### Option B: Local development

Requires Go 1.26, Goose, Docker, and a running Postgres.

```bash
make db-up          # start Postgres via docker compose
make migrate-up     # apply migrations
make seed           # load demo data
make run-api        # start the API on :8080
```

Run the curl demo (seeds first, then exercises every edge case):

```bash
make demo
```

---

## 2. What I Built

A trial booking backend where:

- A parent selects one of their children and books a trial class.
- Payment is mocked and synchronous (`success` / `failure`).
- Only `confirmed` bookings consume class capacity (default 4).
- Staff can view a confirmed roster for any class.

Edge cases handled: duplicate confirmed bookings, overbooking, payment failure,
and — most importantly — the **last-seat race condition**, which is proven under
real concurrent load with the Go race detector.

Backend correctness, invariants, and tests were prioritized over frontend polish.
There is deliberately no polished UI; the API is the product surface.

---

## 3. Time Spent

Time spent: approximately 3.5–4 hours.

---

## 4. Assumptions

- Payment is mocked and synchronous.
- There is no real payment-provider webhook.
- Authentication is not implemented; `parent_id` is trusted for demo purposes.
- Only `confirmed` bookings consume capacity.
- A pending booking does not permanently reserve a seat.
- The admin roster endpoint is unprotected for demo purposes.
- All timestamps are stored as `timestamptz`.

---

## 5. Architecture

```
cmd/
  api/        entrypoint: wiring + HTTP server + graceful shutdown
  seed/       deterministic demo data (raw SQL, single transaction)
internal/
  domain/     models, sentinel errors, repository interfaces, Tx/TxManager
  usecase/    business logic (transactions, race-safe booking + payment)
  repository/postgres/  pgx/v5 implementations of the domain interfaces
  handler/http/         HTTP handlers, router, middleware, response helpers
migrations/   Goose SQL migrations
scripts/      create_migration.sh, generate_initial_migration.sh, demo.sh
docker/       entrypoint.sh (wait → migrate → seed → run)
```

Dependency direction:

```
handler → usecase → domain ← repository/postgres
```

The domain layer has **zero infrastructure dependencies** (no pgx, no `database/sql`,
no `net/http` — only `context`, `errors`, and `time`). The usecase depends only on
the domain interfaces. Repositories implement those interfaces with pgx. Handlers
depend only on the usecase interface.

---

## 6. Data Model

Tables: `parents`, `students`, `trial_classes`, `bookings`, `payment_attempts`.

Important constraints:

- `bookings.status` CHECK in (`pending_payment`, `confirmed`, `payment_failed`, `cancelled`).
- `payment_attempts.status` CHECK in (`pending`, `succeeded`, `failed`).
- `payment_attempts.provider_reference` is `UNIQUE`.
- `trial_classes.capacity` CHECK (`> 0`), default 4.
- **Partial unique index** — the core duplicate-confirmed guard:

```sql
CREATE UNIQUE INDEX uniq_confirmed_booking_per_student_class
ON bookings (student_id, trial_class_id)
WHERE status = 'confirmed';
```

- `bookings.updated_at` exists but is maintained by application code
  (`UPDATE ... SET updated_at = NOW()`), not by a database trigger.

---

## 7. API Endpoints

All business routes are versioned under `/api/v1`. `/healthz` is unversioned.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Liveness probe |
| GET | `/api/v1/classes` | List classes with availability |
| GET | `/api/v1/classes/{id}` | Get one class |
| GET | `/api/v1/parents/{id}` | Parent with their children |
| POST | `/api/v1/bookings` | Create a booking |
| GET | `/api/v1/bookings/{id}` | Get a booking |
| POST | `/api/v1/bookings/{id}/payment` | Process (mock) payment |
| GET | `/api/v1/admin/classes/{id}/roster` | Confirmed roster (admin) |

Create a booking:

```bash
curl -X POST http://localhost:8080/api/v1/bookings \
  -H 'Content-Type: application/json' \
  -d '{"parent_id":1,"student_id":2,"trial_class_id":1}'
```

Process a payment:

```bash
curl -X POST http://localhost:8080/api/v1/bookings/12/payment \
  -H 'Content-Type: application/json' \
  -d '{"outcome":"success","provider_ref":"demo_happy_path"}'
```

---

## 8. Booking Statuses

```
pending_payment ──► confirmed
pending_payment ──► payment_failed
pending_payment ──► cancelled
```

Only a `pending_payment` booking can be confirmed or failed. A booking that is
already `confirmed` (or otherwise not payable) returns `booking_not_payable`.

---

## 9. Edge Cases and Invariants

### Duplicate confirmed booking
Checked in the usecase **and** enforced by the partial unique index
`uniq_confirmed_booking_per_student_class`. A student cannot hold two confirmed
seats in the same class.

### Overbooking
The confirmed count is checked **inside the payment transaction** before
confirming. A class can never exceed `capacity`.

### Payment failure
A failed payment is recorded in `payment_attempts` (status `failed`), the booking
is set to `payment_failed`, and the child never appears on the confirmed roster.

### Last-seat race condition
Payment confirmation runs inside a transaction that:

1. Locks the class row with `SELECT ... FOR UPDATE`.
2. Re-reads the booking status under that lock.
3. Re-checks the confirmed count against capacity.
4. Only then confirms the seat.

Because all competing confirmations for a class serialize on the row lock, **at
most one** pending booking can win the last seat.

**Why pessimistic over optimistic locking:** The last-seat scenario is a
*guaranteed* conflict, not a rare one — optimistic locking's "try then retry"
model performs worst at exactly the moment that matters most. Optimistic also
relies on retries, which is unsafe when the transaction has side effects (a
mocked payment here, a real charge in production). Pessimistic locking is the
right tool because (1) class capacity is tiny (4 seats), so lock hold time is
negligible, and (2) the domain demands strong consistency, not eventual
consistency.

**Tradeoff accepted:** a pending booking does not reserve a seat — a racer can
lose to a faster concurrent payer. For the timebox, this is the simplest correct
solution; a TTL-based seat hold would improve UX but add significant complexity.

---

## 10. Where Checks Live

| Check | UI | HTTP Handler | Usecase | Database | Background Job |
|-------|----|--------------|---------|----------|----------------|
| Request validation (positive IDs, outcome) | — | ✅ | — | — | — |
| Student owned by parent | — | — | ✅ | — | — |
| Duplicate confirmed booking | — | — | ✅ | ✅ partial unique index | — |
| Duplicate pending booking | — | — | ✅ | — | — |
| Class full | — | — | ✅ | — | — |
| Overbooking / last seat | — | — | ✅ (row lock) | ✅ `FOR UPDATE` | — |
| Payment failure recorded | — | — | ✅ | ✅ | — |
| Booking payable (status) | — | — | ✅ | — | — |

---

## 11. Tests and Verification

```bash
go build ./...        # compiles
go vet ./...          # static analysis
make test-usecase     # integration tests (DB up)
make test-concurrency # concurrency tests with -race (DB up)
```

The concurrency tests launch real goroutines racing for the last seat and run
under the Go race detector (`-race`). They prove that **exactly one** racer wins
the last seat and that a class is **never** overbooked, deterministically, across
repeated runs.

---

## 12. Seed Data

```bash
make seed    # truncate + load deterministic demo data
make demo    # seed, then curl every edge case end to end
```

Seeded demo cases:

- Class 1: available seats (1 confirmed, 3 remaining).
- Class 2: exactly 3 confirmed + 2 pending "race" bookings (bookings 5 and 6).
- Class 3: full (4 confirmed).
- Class 4: a payment-failure case (booking 11 is `payment_failed`).
- Student 1 already has a confirmed booking in class 1 → duplicate-booking demo.

---

## 13. What I Deliberately Cut

- Real payment provider / webhooks.
- Authentication and authorization (parent_id is trusted).
- Frontend polish (backend-led; no UI).
- Admin CRUD for classes/parents/students.
- Notifications / email.
- Seat reservation TTL / expiry.
- Refund workflow.

These were cut to stay inside the timebox and focus on the correctness-critical
transaction and race-condition handling.

---

## 14. What I Would Monitor After Release

Business metrics:

- Bookings created vs. confirmed.
- Payment success / failure rate.
- Seat-lost count (`seat_unavailable` occurrences) — a signal of demand vs. capacity.

Technical metrics:

- API error rate and latency (p99).
- Database transaction latency.
- Lock wait time / lock contention on `trial_classes`.

Integrity alert queries:

```sql
-- No class should ever exceed its capacity.
SELECT trial_class_id, COUNT(*)
FROM bookings
WHERE status = 'confirmed'
GROUP BY trial_class_id
HAVING COUNT(*) > 4;

-- No student should hold two confirmed seats in one class.
SELECT student_id, trial_class_id, COUNT(*)
FROM bookings
WHERE status = 'confirmed'
GROUP BY student_id, trial_class_id
HAVING COUNT(*) > 1;
```

---

## 15. What I Would Do Next

- Short-lived seat hold (TTL) so pending bookings reserve a seat for a bounded time.
- Real payment provider with webhooks and **idempotency keys** for safe retries.
- **Message broker/queue — conditionally.** Only if observability shows user traffic has grown significantly AND database lock contention (lock wait time) has become a measurable throughput bottleneck. Even then, the broker would handle only asynchronous side-effects (notifications, analytics), NOT the core seat-confirmation path, which must remain strongly consistent.
- Background jobs for reconciliation and pending-booking expiry.
- Authentication / RBAC (especially for the admin roster).

---

## 16. Tradeoffs Summary

I chose backend correctness and pessimistic locking (`SELECT ... FOR UPDATE`)
over feature breadth and optimistic UI-level reservations. The result is a smaller
feature set, but one whose hardest invariant — "a class is never overbooked" — is
provably enforced under concurrency rather than assumed.
```

---
