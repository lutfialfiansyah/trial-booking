package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/repository/postgres"
)

const defaultDatabaseURL = "postgres://postgres:password@localhost:5432/trial-booking?sslmode=disable"

// seedStatements is the ordered list of deterministic seed operations. They all
// run inside a single transaction so a failure leaves the database untouched.
var seedStatements = []string{
	// a. Reset all data. Seed is a local/demo utility and may be destructive.
	`TRUNCATE TABLE
		payment_attempts,
		bookings,
		trial_classes,
		students,
		parents
	RESTART IDENTITY CASCADE`,

	// b. Parents.
	`INSERT INTO parents (id, name, email) VALUES
		(1, 'Sarah Mitchell', 'sarah.mitchell@gmail.com'),
		(2, 'James Okafor',   'james.okafor@gmail.com')`,

	// c. Students.
	`INSERT INTO students (id, parent_id, name) VALUES
		(1, 1, 'Emma Mitchell'),
		(2, 1, 'Noah Mitchell'),
		(3, 2, 'Zara Okafor'),
		(4, 2, 'Kai Okafor'),
		(5, 2, 'Liam Okafor'),
		(6, 2, 'Ava Okafor')`,

	// d. Trial classes.
	`INSERT INTO trial_classes (id, title, subject, starts_at, capacity) VALUES
		(1, 'Creative Drawing for Beginners', 'Art',      NOW() + INTERVAL '3 days', 4),
		(2, 'Junior Robotics Workshop',       'Robotics', NOW() + INTERVAL '5 days', 4),
		(3, 'Storytelling & Drama Basics',    'Drama',    NOW() + INTERVAL '7 days', 4),
		(4, 'Introduction to Piano',          'Music',    NOW() + INTERVAL '9 days', 4)`,

	// e. Bookings.
	`INSERT INTO bookings (id, student_id, trial_class_id, status) VALUES
		(1,  1, 1, 'confirmed'),
		(2,  2, 2, 'confirmed'),
		(3,  3, 2, 'confirmed'),
		(4,  4, 2, 'confirmed'),
		(5,  5, 2, 'pending_payment'),
		(6,  6, 2, 'pending_payment'),
		(7,  1, 3, 'confirmed'),
		(8,  2, 3, 'confirmed'),
		(9,  3, 3, 'confirmed'),
		(10, 4, 3, 'confirmed'),
		(11, 1, 4, 'payment_failed'),
		(12, 2, 1, 'pending_payment')`,

	// f. Payment attempts. One per confirmed booking plus the failure case.
	//    Bookings 5, 6 and 12 intentionally have none (paid during the demo).
	`INSERT INTO payment_attempts (id, booking_id, status, provider_reference, failure_reason) VALUES
		(1, 1,  'succeeded', 'mock_pay_1',  NULL),
		(2, 2,  'succeeded', 'mock_pay_2',  NULL),
		(3, 3,  'succeeded', 'mock_pay_3',  NULL),
		(4, 4,  'succeeded', 'mock_pay_4',  NULL),
		(5, 7,  'succeeded', 'mock_pay_7',  NULL),
		(6, 8,  'succeeded', 'mock_pay_8',  NULL),
		(7, 9,  'succeeded', 'mock_pay_9',  NULL),
		(8, 10, 'succeeded', 'mock_pay_10', NULL),
		(9, 11, 'failed',    'mock_pay_11', 'payment_declined')`,

	// g. Advance identity sequences past the explicit IDs.
	`SELECT setval(pg_get_serial_sequence('parents', 'id'), COALESCE((SELECT MAX(id) FROM parents), 1))`,
	`SELECT setval(pg_get_serial_sequence('students', 'id'), COALESCE((SELECT MAX(id) FROM students), 1))`,
	`SELECT setval(pg_get_serial_sequence('trial_classes', 'id'), COALESCE((SELECT MAX(id) FROM trial_classes), 1))`,
	`SELECT setval(pg_get_serial_sequence('bookings', 'id'), COALESCE((SELECT MAX(id) FROM bookings), 1))`,
	`SELECT setval(pg_get_serial_sequence('payment_attempts', 'id'), COALESCE((SELECT MAX(id) FROM payment_attempts), 1))`,
}

const summaryQuery = `
	SELECT
		tc.id,
		tc.title,
		tc.capacity,
		COUNT(b.id) FILTER (WHERE b.status = 'confirmed')::int        AS confirmed_count,
		COUNT(b.id) FILTER (WHERE b.status = 'pending_payment')::int  AS pending_count,
		tc.capacity - COUNT(b.id) FILTER (WHERE b.status = 'confirmed')::int AS remaining_seats
	FROM trial_classes tc
	LEFT JOIN bookings b ON b.trial_class_id = tc.id
	GROUP BY tc.id, tc.title, tc.capacity
	ORDER BY tc.id`

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("seed: connect to database: %v", err)
	}
	defer pool.Close()

	// Run the whole seed atomically.
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("seed: begin tx: %v", err)
	}

	for i, stmt := range seedStatements {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("seed: statement %d failed: %v", i+1, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("seed: commit: %v", err)
	}

	if err := printSummary(ctx, pool); err != nil {
		log.Fatalf("seed: print summary: %v", err)
	}

	printGuidance()
}

// printSummary prints a per-class availability table.
func printSummary(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, summaryQuery)
	if err != nil {
		return fmt.Errorf("query summary: %w", err)
	}
	defer rows.Close()

	fmt.Println()
	fmt.Printf("%-3s  %-34s  %-8s  %-15s  %-13s  %-15s\n",
		"id", "title", "capacity", "confirmed_count", "pending_count", "remaining_seats")
	fmt.Println("---  ----------------------------------  --------  ---------------  -------------  ---------------")

	for rows.Next() {
		var (
			id             int64
			title          string
			capacity       int
			confirmedCount int
			pendingCount   int
			remainingSeats int
		)
		if err := rows.Scan(&id, &title, &capacity, &confirmedCount, &pendingCount, &remainingSeats); err != nil {
			return fmt.Errorf("scan summary: %w", err)
		}
		fmt.Printf("%-3d  %-34s  %-8d  %-15d  %-13d  %-15d\n",
			id, title, capacity, confirmedCount, pendingCount, remainingSeats)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate summary: %w", err)
	}

	return nil
}

// printGuidance prints the demo cases and suggested commands.
func printGuidance() {
	fmt.Println()
	fmt.Println("Seed complete.")
	fmt.Println()
	fmt.Println("Demo cases:")
	fmt.Println("- Class 1 (Creative Drawing for Beginners) has available seats.")
	fmt.Println("- Class 2 (Junior Robotics Workshop) has exactly 3 confirmed students and 2 pending race bookings: booking 5 and booking 6.")
	fmt.Println("- Class 3 (Storytelling & Drama Basics) is full.")
	fmt.Println("- Booking 11 is a payment failure case.")
	fmt.Println("- Emma Mitchell (student 1) already has a confirmed booking in class 1. Use this to demo duplicate booking rejection.")
	fmt.Println()
	fmt.Println("Suggested commands:")
	fmt.Println("  curl http://localhost:8080/api/v1/classes")
	fmt.Println("  curl http://localhost:8080/api/v1/parents/1")
	fmt.Println("  curl http://localhost:8080/api/v1/bookings/11")
	fmt.Println("  curl -X POST http://localhost:8080/api/v1/bookings/12/payment -H 'Content-Type: application/json' -d '{\"outcome\":\"success\",\"provider_ref\":\"demo_happy_path\"}'")
	fmt.Println("  curl -X POST http://localhost:8080/api/v1/bookings -H 'Content-Type: application/json' -d '{\"parent_id\":1,\"student_id\":1,\"trial_class_id\":1}'")
	fmt.Println("  curl -X POST http://localhost:8080/api/v1/bookings/6/payment -H 'Content-Type: application/json' -d '{\"outcome\":\"success\",\"provider_ref\":\"demo_race_b\"}'")
	fmt.Println("  curl -X POST http://localhost:8080/api/v1/bookings/5/payment -H 'Content-Type: application/json' -d '{\"outcome\":\"success\",\"provider_ref\":\"demo_race_a\"}'")
	fmt.Println("  curl http://localhost:8080/api/v1/admin/classes/2/roster")
}
