//go:build integration

package usecase

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"trial-booking/internal/domain"
)

// The package already defines TestMain, testPool, testUC, baseCtx, resetDatabase,
// createClass, createStudent and insertConfirmedBooking in
// booking_usecase_integration_test.go. We REUSE those and only add the small
// fixture helpers this file needs. Do NOT define a second TestMain here.

// raceResult captures the outcome of one concurrent ProcessPayment call.
type raceResult struct {
	bookingID int64
	err       error
}

// concurrencyCtx returns a context with a generous timeout so heavy concurrent
// DB work does not time out prematurely.
func concurrencyCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// resetForConcurrency truncates all domain tables and seeds a single parent plus
// numStudents students (ids 1..numStudents) under that parent.
func resetForConcurrency(t *testing.T, numStudents int) {
	t.Helper()

	const truncate = `TRUNCATE TABLE payment_attempts, bookings, trial_classes, students, parents RESTART IDENTITY CASCADE`
	if _, err := testPool.Exec(baseCtx, truncate); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	if _, err := testPool.Exec(baseCtx,
		`INSERT INTO parents (id, name, email) VALUES (1, 'Race Parent', 'race@example.com')`,
	); err != nil {
		t.Fatalf("insert parent: %v", err)
	}

	for i := 1; i <= numStudents; i++ {
		if _, err := testPool.Exec(baseCtx,
			`INSERT INTO students (id, parent_id, name) VALUES ($1, 1, $2)`,
			i, "Race Student "+strconv.Itoa(i),
		); err != nil {
			t.Fatalf("insert student %d: %v", i, err)
		}
	}

	// Keep the identity sequence ahead of the explicit ids we inserted.
	if _, err := testPool.Exec(baseCtx, `SELECT setval('parents_id_seq', (SELECT MAX(id) FROM parents))`); err != nil {
		t.Fatalf("setval parents: %v", err)
	}
	if _, err := testPool.Exec(baseCtx, `SELECT setval('students_id_seq', (SELECT MAX(id) FROM students))`); err != nil {
		t.Fatalf("setval students: %v", err)
	}
}

// confirmedCountForClass returns the number of confirmed bookings for a class.
func confirmedCountForClass(t *testing.T, classID int64) int {
	t.Helper()
	var count int
	if err := testPool.QueryRow(baseCtx,
		`SELECT COUNT(*) FROM bookings WHERE trial_class_id = $1 AND status = 'confirmed'`,
		classID,
	).Scan(&count); err != nil {
		t.Fatalf("count confirmed: %v", err)
	}
	return count
}

// bookingStatus returns the status of a booking.
func bookingStatus(t *testing.T, bookingID int64) domain.BookingStatus {
	t.Helper()
	var status domain.BookingStatus
	if err := testPool.QueryRow(baseCtx,
		`SELECT status FROM bookings WHERE id = $1`,
		bookingID,
	).Scan(&status); err != nil {
		t.Fatalf("get booking status %d: %v", bookingID, err)
	}
	return status
}

// TestProcessPayment_LastSeatRace_Concurrent proves that when two users truly
// race for the single remaining seat, the SELECT ... FOR UPDATE row lock
// guarantees EXACTLY ONE winner and no overbooking.
func TestProcessPayment_LastSeatRace_Concurrent(t *testing.T) {
	ctx, cancel := concurrencyCtx(t)
	defer cancel()

	// 1 parent, 5 students.
	resetForConcurrency(t, 5)

	// 1 class with capacity 4.
	classID := createClass(t, "Race Class", 4)

	// Pre-fill 3 confirmed seats (students 1, 2, 3).
	insertConfirmedBooking(t, 1, classID)
	insertConfirmedBooking(t, 2, classID)
	insertConfirmedBooking(t, 3, classID)

	if got := confirmedCountForClass(t, classID); got != 3 {
		t.Fatalf("precondition: expected 3 confirmed, got %d", got)
	}

	// Two racers create PENDING bookings for the last remaining seat.
	bookingA, err := testUC.CreateBooking(ctx, 1, 4, classID)
	if err != nil {
		t.Fatalf("create racer A booking: %v", err)
	}
	bookingB, err := testUC.CreateBooking(ctx, 1, 5, classID)
	if err != nil {
		t.Fatalf("create racer B booking: %v", err)
	}

	racers := []struct {
		bookingID int64
		ref       string
	}{
		{bookingA.ID, "race_a"},
		{bookingB.ID, "race_b"},
	}

	// Launch the race.
	results := make(chan raceResult, len(racers))
	var wg sync.WaitGroup
	start := make(chan struct{})

	for _, r := range racers {
		wg.Add(1)
		go func(bookingID int64, ref string) {
			defer wg.Done()
			<-start // release all goroutines at once
			_, err := testUC.ProcessPayment(ctx, bookingID, ref, true)
			results <- raceResult{bookingID: bookingID, err: err}
		}(r.bookingID, r.ref)
	}

	close(start) // fire
	wg.Wait()
	close(results)

	// Collect + classify outcomes.
	var winners, seatGone int
	var winnerBookingID int64
	for res := range results {
		switch {
		case res.err == nil:
			winners++
			winnerBookingID = res.bookingID
		case errors.Is(res.err, domain.ErrSeatNoLongerAvailable):
			seatGone++
		default:
			t.Fatalf("unexpected error from racer booking %d: %v", res.bookingID, res.err)
		}
	}

	if winners != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", winners)
	}
	if seatGone != 1 {
		t.Fatalf("expected exactly 1 ErrSeatNoLongerAvailable, got %d", seatGone)
	}

	// No overbooking: exactly capacity confirmed.
	if got := confirmedCountForClass(t, classID); got != 4 {
		t.Fatalf("expected exactly 4 confirmed (no overbooking), got %d", got)
	}

	// Exactly one racer booking confirmed, the other payment_failed.
	loserBookingID := bookingA.ID
	if winnerBookingID == bookingA.ID {
		loserBookingID = bookingB.ID
	}
	if got := bookingStatus(t, winnerBookingID); got != domain.BookingStatusConfirmed {
		t.Fatalf("winner booking %d: expected confirmed, got %q", winnerBookingID, got)
	}
	if got := bookingStatus(t, loserBookingID); got != domain.BookingStatusPaymentFailed {
		t.Fatalf("loser booking %d: expected payment_failed, got %q", loserBookingID, got)
	}
}

// TestProcessPayment_Overbooking_Concurrent proves a class can NEVER exceed its
// capacity even when many racers compete from an empty class. With 8 racers and
// capacity 4, exactly 4 must win.
func TestProcessPayment_Overbooking_Concurrent(t *testing.T) {
	ctx, cancel := concurrencyCtx(t)
	defer cancel()

	const (
		numRacers = 8
		capacity  = 4
	)

	// 1 parent, 8 students.
	resetForConcurrency(t, numRacers)

	// 1 class with capacity 4.
	classID := createClass(t, "Stress Class", capacity)

	// Create 8 PENDING bookings (one per student). All succeed because confirmed
	// count (0) < capacity; pending bookings do not consume seats.
	bookings := make([]int64, 0, numRacers)
	for studentID := 1; studentID <= numRacers; studentID++ {
		b, err := testUC.CreateBooking(ctx, 1, int64(studentID), classID)
		if err != nil {
			t.Fatalf("create booking for student %d: %v", studentID, err)
		}
		bookings = append(bookings, b.ID)
	}

	// Launch all racers.
	results := make(chan raceResult, len(bookings))
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i, bookingID := range bookings {
		wg.Add(1)
		go func(n int, id int64) {
			defer wg.Done()
			<-start
			_, err := testUC.ProcessPayment(ctx, id, "stress_"+strconv.Itoa(n), true)
			results <- raceResult{bookingID: id, err: err}
		}(i, bookingID)
	}

	close(start)
	wg.Wait()
	close(results)

	// Classify outcomes.
	var winners, seatGone int
	for res := range results {
		switch {
		case res.err == nil:
			winners++
		case errors.Is(res.err, domain.ErrSeatNoLongerAvailable):
			seatGone++
		default:
			t.Fatalf("unexpected error from booking %d: %v", res.bookingID, res.err)
		}
	}

	if winners != capacity {
		t.Fatalf("expected exactly %d winners, got %d", capacity, winners)
	}
	if seatGone != numRacers-capacity {
		t.Fatalf("expected exactly %d ErrSeatNoLongerAvailable, got %d", numRacers-capacity, seatGone)
	}

	// No overbooking.
	if got := confirmedCountForClass(t, classID); got != capacity {
		t.Fatalf("expected exactly %d confirmed (no overbooking), got %d", capacity, got)
	}
}
