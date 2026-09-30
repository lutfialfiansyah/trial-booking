# AI Usage

## Tools Used

- **Chat-based AI coding assistant** — used throughout the project for architecture discussion, prompt-driven implementation, code review, and documentation drafting.
- **IDE code completion** — used for inline completions and small manual edits during review passes.

## Division of Responsibility

All architectural decisions and correctness invariants were mine. AI was used as an implementation accelerator under my direction.

**My decisions (human-driven):**
- Data model and schema design (5 tables, partial unique index, booking statuses)
- Chose pessimistic locking (`SELECT ... FOR UPDATE`) for the last-seat race
- Layer boundaries (domain → usecase → repository → handler) and dependency inversion
- The `businessErr` capture pattern for transaction-safe business failures
- Scope control: rejected i18n and frontend polish, kept the solution backend-led
- Chose machine-readable `lower_snake_case` error codes

**AI-assisted:**
- Generated boilerplate and scaffolding from my specifications
- Drafted test scaffolding and seed data
- Helped structure per-phase implementation prompts
- Formatted README and documentation drafts

## What I Used AI For

- Scaffolding project structure and boilerplate (DTOs, handlers, Makefile targets, Dockerfile).
- Structuring implementation prompts for each build phase.
- Drafting README and documentation structure.
- Reviewing edge-case coverage and suggesting test scenarios.

## Where AI Helped Me Move Faster

AI accelerated the creation of repetitive backend scaffolding, especially DTOs, handler boilerplate, Makefile targets, and test scaffolding. This let me spend more time on the parts that matter most for this assessment: the core transaction design and race-condition handling.

## Where I Disagreed With, Corrected, or Rejected AI Output

**Primary example — transaction rollback bug:**
AI initially suggested handling business failures (payment declined, seat lost) by returning an error directly from inside the transaction closure. That would have caused the transaction to roll back and lose the failed payment-attempt record. I corrected this by introducing a `businessErr` capture pattern: business failures that require committed database changes are captured in a variable, the closure returns `nil` to force a COMMIT, and the business error is returned to the caller after the transaction completes.

**Additional examples of steering AI output:**

- **Rejected message broker for booking confirmations.** AI proposed introducing a message broker (Kafka/RabbitMQ) to handle booking confirmations asynchronously for scalability. I rejected this because the last-seat race requires *strong consistency*, which a broker's eventual-consistency model would undermine. I would only introduce a message broker/queue when observability data shows that user traffic has grown significantly AND database lock contention (lock wait time) has become a measurable bottleneck degrading throughput. Even then, the broker would handle only asynchronous side-effects (notifications, analytics), not the core seat-confirmation path, which must remain strongly consistent.

- **Corrected error code casing.** AI initially generated `UPPER_CASE` error codes (e.g., `CLASS_FULL`, `SEAT_UNAVAILABLE`). I corrected these to `lower_snake_case` (e.g., `class_full`, `seat_unavailable`) for consistency with the booking status values (`pending_payment`, `confirmed`) and the `snake_case` JSON field naming already used throughout the API response contract.

- **Rejected i18n error messages.** AI suggested adding multi-language error messages. I rejected this given the 4-hour timebox and instead chose machine-readable `lower_snake_case` error codes, which are i18n-ready without adding localization complexity.

- **Rejected a database trigger for `updated_at`.** AI suggested maintaining `updated_at` via a database trigger. I chose application-level updates instead for explicit control and testability.

- **Replaced Docker init scripts with versioned migrations.** AI initially used PostgreSQL Docker `initdb` scripts (`db/init/`) for schema setup. I directed a switch to proper versioned migrations using Goose with Makefile targets (`make migrate-up`, `make migrate-down`, `make migrate-status`), because migrations are idempotent, version-controlled, reversible, and represent how schema changes would actually be managed in production.

## What I Would Change About My AI Workflow

- Keep a prompt log from the beginning.
- Ask AI to produce an edge-case matrix before implementation, not after.
- Keep AI_USAGE notes updated during development instead of writing them at the very end.

## How I Verified the Final Implementation

```bash
go build ./...
go vet ./...
make test-usecase
make test-concurrency
make seed
make demo
docker compose up --build