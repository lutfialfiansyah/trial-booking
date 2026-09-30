//go:build integration

package usecase

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/domain"
	"trial-booking/internal/repository/postgres"
)

const defaultTestDatabaseURL = "postgres://postgres:password@localhost:5432/trial-booking?sslmode=disable"

var (
	testPool *pgxpool.Pool
	testUC   BookingUsecase
	baseCtx  = context.Background()
)

func TestMain(m *testing.M) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultTestDatabaseURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}
	testPool = pool
	defer pool.Close()

	testUC = NewBookingUsecase(
		postgres.NewTxManager(pool),
		postgres.NewClassRepository(pool),
		postgres.NewBookingRepository(pool),
		postgres.NewPaymentRepository(pool),
		postgres.NewStudentRepository(pool),
		postgres.NewParentRepository(pool),
	)

	os.Exit(m.Run())
}

// resetDatabase truncates all domain tables in dependency order and inserts the
// base fixtures: parent 1, students 1 and 2 under parent 1, and parent 2 (used
// by the ownership test).
func resetDatabase(t *testing.T) {
	t.Helper()

	const truncate = `TRUNCATE TABLE payment_attempts, bookings, trial_classes, students, parents RESTART IDENTITY CASCADE`
	if _, err := testPool.Exec(baseCtx, truncate); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	if _, err := testPool.Exec(baseCtx,
		`INSERT INTO parents (id, name, email) VALUES
			(1, 'Parent One', 'parent1@example.com'),
			(2, 'Parent Two', 'parent2@example.com')`,
	); err != nil {
		t.Fatalf("insert parents: %v", err)
	}

	if _, err := testPool.Exec(baseCtx,
		`INSERT INTO students (id, parent_id, name) VALUES
			(1, 1, 'Student One'),
			(2, 1, 'Student Two')`,
	); err != nil {
		t.Fatalf("insert students: %v", err)
	}

	// Keep identity sequences ahead of the explicit ids we inserted.
	if _, err := testPool.Exec(baseCtx, `SELECT setval('parents_id_seq', (SELECT MAX(id) FROM parents))`); err != nil {
		t.Fatalf("setval parents: %v", err)
	}
	if _, err := testPool.Exec(baseCtx, `SELECT setval('students_id_seq', (SELECT MAX(id) FROM students))`); err != nil {
		t.Fatalf("setval students: %v", err)
	}
}

// createClass inserts a trial class and returns its id.
func createClass(t *testing.T, title string, capacity int) int64 {
	t.Helper()

	var id int64
	err := testPool.QueryRow(baseCtx,
		`INSERT INTO trial_classes (title, subject, starts_at, capacity)
		 VALUES ($1, 'Math', NOW() + INTERVAL '1 day', $2)
		 RETURNING id`,
		title, capacity,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert class: %v", err)
	}
	return id
}

// createStudent inserts an extra student under a parent and returns its id.
func createStudent(t *testing.T, parentID int64, name string) int64 {
	t.Helper()

	var id int64
	err := testPool.QueryRow(baseCtx,
		`INSERT INTO students (parent_id, name) VALUES ($1, $2) RETURNING id`,
		parentID, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert student: %v", err)
	}
	return id
}

// insertConfirmedBooking inserts a confirmed booking directly (bypassing the
// usecase) so tests can pre-fill seats.
func insertConfirmedBooking(t *testing.T, studentID, classID int64) int64 {
	t.Helper()

	var id int64
	err := testPool.QueryRow(baseCtx,
		`INSERT INTO bookings (student_id, trial_class_id, status)
		 VALUES ($1, $2, 'confirmed')
		 RETURNING id`,
		studentID, classID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert confirmed booking: %v", err)
	}
	return id
}

func TestCreateBooking_Success(t *testing.T) {
	resetDatabase(t)
	classID := createClass(t, "Class A", 4)

	booking, err := testUC.CreateBooking(baseCtx, 1, 1, classID)
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	if booking.Status != domain.BookingStatusPendingPayment {
		t.Fatalf("expected pending_payment, got %q", booking.Status)
	}
	if booking.StudentID != 1 || booking.TrialClassID != classID {
		t.Fatalf("unexpected booking: %+v", booking)
	}
}

func TestCreateBooking_StudentNotOwnedByParent(t *testing.T) {
	resetDatabase(t)
	classID := createClass(t, "Class A", 4)

	// Student 1 belongs to parent 1, but parent 2 is asking.
	_, err := testUC.CreateBooking(baseCtx, 2, 1, classID)
	if !errors.Is(err, domain.ErrStudentNotOwnedByParent) {
		t.Fatalf("expected ErrStudentNotOwnedByParent, got %v", err)
	}
}

func TestCreateBooking_DuplicateConfirmedBooking(t *testing.T) {
	resetDatabase(t)
	classID := createClass(t, "Class A", 4)

	booking, err := testUC.CreateBooking(baseCtx, 1, 1, classID)
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}

	if _, err := testUC.ProcessPayment(baseCtx, booking.ID, "ref-1", true); err != nil {
		t.Fatalf("ProcessPayment: %v", err)
	}

	_, err = testUC.CreateBooking(baseCtx, 1, 1, classID)
	if !errors.Is(err, domain.ErrDuplicateConfirmedBooking) {
		t.Fatalf("expected ErrDuplicateConfirmedBooking, got %v", err)
	}
}

func TestCreateBooking_ClassFull(t *testing.T) {
	resetDatabase(t)
	classID := createClass(t, "Tiny Class", 1)

	booking, err := testUC.CreateBooking(baseCtx, 1, 1, classID)
	if err != nil {
		t.Fatalf("CreateBooking (student 1): %v", err)
	}
	if _, err := testUC.ProcessPayment(baseCtx, booking.ID, "ref-1", true); err != nil {
		t.Fatalf("ProcessPayment: %v", err)
	}

	// Student 2 tries to book the now-full class.
	_, err = testUC.CreateBooking(baseCtx, 1, 2, classID)
	if !errors.Is(err, domain.ErrClassFull) {
		t.Fatalf("expected ErrClassFull, got %v", err)
	}
}

func TestProcessPayment_Success(t *testing.T) {
	resetDatabase(t)
	classID := createClass(t, "Class A", 4)

	booking, err := testUC.CreateBooking(baseCtx, 1, 1, classID)
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}

	confirmed, err := testUC.ProcessPayment(baseCtx, booking.ID, "ref-success", true)
	if err != nil {
		t.Fatalf("ProcessPayment: %v", err)
	}
	if confirmed.Status != domain.BookingStatusConfirmed {
		t.Fatalf("expected confirmed, got %q", confirmed.Status)
	}

	repo := postgres.NewBookingRepository(testPool)
	count, err := repo.CountConfirmedByClass(baseCtx, nil, classID)
	if err != nil {
		t.Fatalf("CountConfirmedByClass: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 confirmed, got %d", count)
	}
}

func TestProcessPayment_PaymentDeclined(t *testing.T) {
	resetDatabase(t)
	classID := createClass(t, "Class A", 4)

	booking, err := testUC.CreateBooking(baseCtx, 1, 1, classID)
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}

	_, err = testUC.ProcessPayment(baseCtx, booking.ID, "ref-declined", false)
	if !errors.Is(err, domain.ErrPaymentFailed) {
		t.Fatalf("expected ErrPaymentFailed, got %v", err)
	}

	// The business outcome must have been COMMITTED despite the error.
	repo := postgres.NewBookingRepository(testPool)
	got, err := repo.GetByID(baseCtx, nil, booking.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != domain.BookingStatusPaymentFailed {
		t.Fatalf("expected payment_failed in DB, got %q", got.Status)
	}

	// A failed payment attempt must exist.
	var attempts int
	if err := testPool.QueryRow(baseCtx,
		`SELECT COUNT(*) FROM payment_attempts WHERE booking_id = $1 AND status = 'failed'`,
		booking.ID,
	).Scan(&attempts); err != nil {
		t.Fatalf("count payment attempts: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 failed payment attempt, got %d", attempts)
	}
}

func TestProcessPayment_SeatGone_LastSeatRace_Sequential(t *testing.T) {
	resetDatabase(t)
	classID := createClass(t, "Capacity Class", 4)

	// Fill 3 of 4 seats directly.
	insertConfirmedBooking(t, 1, classID)
	insertConfirmedBooking(t, 2, classID)
	extra := createStudent(t, 1, "Student Three")
	insertConfirmedBooking(t, extra, classID)

	// Two more students create pending bookings, competing for the last seat.
	studentX := createStudent(t, 1, "Student X")
	studentY := createStudent(t, 1, "Student Y")

	bookingA, err := testUC.CreateBooking(baseCtx, 1, studentX, classID)
	if err != nil {
		t.Fatalf("CreateBooking A: %v", err)
	}
	bookingB, err := testUC.CreateBooking(baseCtx, 1, studentY, classID)
	if err != nil {
		t.Fatalf("CreateBooking B: %v", err)
	}

	// B pays first and takes the last seat.
	confirmedB, err := testUC.ProcessPayment(baseCtx, bookingB.ID, "ref-b", true)
	if err != nil {
		t.Fatalf("ProcessPayment B: %v", err)
	}
	if confirmedB.Status != domain.BookingStatusConfirmed {
		t.Fatalf("expected B confirmed, got %q", confirmedB.Status)
	}

	// A pays second; the seat is gone.
	_, err = testUC.ProcessPayment(baseCtx, bookingA.ID, "ref-a", true)
	if !errors.Is(err, domain.ErrSeatNoLongerAvailable) {
		t.Fatalf("expected ErrSeatNoLongerAvailable, got %v", err)
	}

	repo := postgres.NewBookingRepository(testPool)
	count, err := repo.CountConfirmedByClass(baseCtx, nil, classID)
	if err != nil {
		t.Fatalf("CountConfirmedByClass: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected exactly 4 confirmed (no overbooking), got %d", count)
	}
}

func TestProcessPayment_BookingNotPayable(t *testing.T) {
	resetDatabase(t)
	classID := createClass(t, "Class A", 4)

	booking, err := testUC.CreateBooking(baseCtx, 1, 1, classID)
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	if _, err := testUC.ProcessPayment(baseCtx, booking.ID, "ref-1", true); err != nil {
		t.Fatalf("ProcessPayment (first): %v", err)
	}

	// A confirmed booking cannot be paid again.
	_, err = testUC.ProcessPayment(baseCtx, booking.ID, "ref-2", true)
	if !errors.Is(err, domain.ErrBookingNotPayable) {
		t.Fatalf("expected ErrBookingNotPayable, got %v", err)
	}
}

func TestGetRoster(t *testing.T) {
	resetDatabase(t)
	classID := createClass(t, "Roster Class", 4)

	// Mixed-status roster: 2 confirmed, 1 pending, 1 payment_failed.
	b1, err := testUC.CreateBooking(baseCtx, 1, 1, classID)
	if err != nil {
		t.Fatalf("CreateBooking 1: %v", err)
	}
	b2, err := testUC.CreateBooking(baseCtx, 1, 2, classID)
	if err != nil {
		t.Fatalf("CreateBooking 2: %v", err)
	}

	student3 := createStudent(t, 1, "Student Three")
	student4 := createStudent(t, 1, "Student Four")

	b3, err := testUC.CreateBooking(baseCtx, 1, student3, classID)
	if err != nil {
		t.Fatalf("CreateBooking 3: %v", err)
	}
	b4, err := testUC.CreateBooking(baseCtx, 1, student4, classID)
	if err != nil {
		t.Fatalf("CreateBooking 4: %v", err)
	}

	// Confirm two, leave one pending (b3), and fail one payment (b4).
	if _, err := testUC.ProcessPayment(baseCtx, b1.ID, "ref-1", true); err != nil {
		t.Fatalf("ProcessPayment 1: %v", err)
	}
	if _, err := testUC.ProcessPayment(baseCtx, b2.ID, "ref-2", true); err != nil {
		t.Fatalf("ProcessPayment 2: %v", err)
	}
	if _, err := testUC.ProcessPayment(baseCtx, b4.ID, "ref-4", false); err != nil && !errors.Is(err, domain.ErrPaymentFailed) {
		t.Fatalf("ProcessPayment 4: %v", err)
	}
	// b3 is left untouched (pending_payment).
	if b3.Status != domain.BookingStatusPendingPayment {
		t.Fatalf("expected booking 3 to remain pending_payment, got %q", b3.Status)
	}

	roster, err := testUC.GetRoster(baseCtx, classID)
	if err != nil {
		t.Fatalf("GetRoster: %v", err)
	}

	// All bookings are returned, not just confirmed.
	if len(roster.Entries) != 4 {
		t.Fatalf("expected 4 entries (all statuses), got %d", len(roster.Entries))
	}
	if roster.TotalBookings != 4 {
		t.Fatalf("expected TotalBookings 4, got %d", roster.TotalBookings)
	}

	// Only confirmed bookings count toward capacity.
	if roster.ConfirmedCount != 2 {
		t.Fatalf("expected ConfirmedCount 2, got %d", roster.ConfirmedCount)
	}
	if roster.RemainingSeats != 2 {
		t.Fatalf("expected RemainingSeats 2, got %d", roster.RemainingSeats)
	}

	// Statuses must include non-confirmed bookings.
	statusCounts := map[domain.BookingStatus]int{}
	names := map[string]bool{}
	for _, e := range roster.Entries {
		statusCounts[e.Status]++
		names[e.StudentName] = true
		if e.ParentName != "Parent One" {
			t.Errorf("expected ParentName %q, got %q", "Parent One", e.ParentName)
		}
		if e.ParentEmail != "parent1@example.com" {
			t.Errorf("expected ParentEmail %q, got %q", "parent1@example.com", e.ParentEmail)
		}
	}

	if statusCounts[domain.BookingStatusConfirmed] != 2 {
		t.Errorf("expected 2 confirmed entries, got %d", statusCounts[domain.BookingStatusConfirmed])
	}
	if statusCounts[domain.BookingStatusPendingPayment] != 1 {
		t.Errorf("expected 1 pending_payment entry, got %d", statusCounts[domain.BookingStatusPendingPayment])
	}
	if statusCounts[domain.BookingStatusPaymentFailed] != 1 {
		t.Errorf("expected 1 payment_failed entry, got %d", statusCounts[domain.BookingStatusPaymentFailed])
	}

	// Confirmed entries must be ordered before the others.
	seenNonConfirmed := false
	for _, e := range roster.Entries {
		if e.Status != domain.BookingStatusConfirmed {
			seenNonConfirmed = true
			continue
		}
		if seenNonConfirmed {
			t.Errorf("confirmed entry %q appeared after a non-confirmed entry", e.StudentName)
		}
	}

	if !names["Student One"] || !names["Student Two"] {
		t.Fatalf("roster entries missing students: %+v", roster.Entries)
	}
}

func TestListClasses(t *testing.T) {
	resetDatabase(t)
	classA := createClass(t, "Class A", 4)
	classB := createClass(t, "Class B", 3)

	// One confirmed booking in A, two in B.
	ba, err := testUC.CreateBooking(baseCtx, 1, 1, classA)
	if err != nil {
		t.Fatalf("CreateBooking A: %v", err)
	}
	if _, err := testUC.ProcessPayment(baseCtx, ba.ID, "ref-a", true); err != nil {
		t.Fatalf("ProcessPayment A: %v", err)
	}

	bb1, err := testUC.CreateBooking(baseCtx, 1, 1, classB)
	if err != nil {
		t.Fatalf("CreateBooking B1: %v", err)
	}
	bb2, err := testUC.CreateBooking(baseCtx, 1, 2, classB)
	if err != nil {
		t.Fatalf("CreateBooking B2: %v", err)
	}
	if _, err := testUC.ProcessPayment(baseCtx, bb1.ID, "ref-b1", true); err != nil {
		t.Fatalf("ProcessPayment B1: %v", err)
	}
	if _, err := testUC.ProcessPayment(baseCtx, bb2.ID, "ref-b2", true); err != nil {
		t.Fatalf("ProcessPayment B2: %v", err)
	}

	classes, err := testUC.ListClasses(baseCtx)
	if err != nil {
		t.Fatalf("ListClasses: %v", err)
	}
	if len(classes) != 2 {
		t.Fatalf("expected 2 classes, got %d", len(classes))
	}

	byID := map[int64]domain.ClassWithAvailability{}
	for _, c := range classes {
		byID[c.ID] = c
	}

	if got := byID[classA]; got.ConfirmedCount != 1 || got.RemainingSeats != 3 {
		t.Errorf("Class A: want confirmed=1 remaining=3, got confirmed=%d remaining=%d", got.ConfirmedCount, got.RemainingSeats)
	}
	if got := byID[classB]; got.ConfirmedCount != 2 || got.RemainingSeats != 1 {
		t.Errorf("Class B: want confirmed=2 remaining=1, got confirmed=%d remaining=%d", got.ConfirmedCount, got.RemainingSeats)
	}
}
