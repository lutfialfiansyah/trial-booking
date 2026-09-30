//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/domain"
)

const defaultTestDatabaseURL = "postgres://postgres:password@localhost:5432/trial-booking?sslmode=disable"

var testPool *pgxpool.Pool

// TestMain sets up the pool and seeds a deterministic set of fixtures.
func TestMain(m *testing.M) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultTestDatabaseURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}
	testPool = pool
	defer pool.Close()

	if err := setupFixtures(ctx); err != nil {
		panic("failed to set up fixtures: " + err.Error())
	}

	os.Exit(m.Run())
}

// resetTables truncates all domain tables in dependency order.
func resetTables(ctx context.Context) error {
	const query = `TRUNCATE TABLE payment_attempts, bookings, trial_classes, students, parents RESTART IDENTITY CASCADE`
	if _, err := testPool.Exec(ctx, query); err != nil {
		return err
	}
	return nil
}

// setupFixtures resets the database and inserts one parent, two students, and
// one trial class with capacity 4.
func setupFixtures(ctx context.Context) error {
	if err := resetTables(ctx); err != nil {
		return err
	}

	if _, err := testPool.Exec(ctx,
		`INSERT INTO parents (name, email) VALUES ($1, $2)`,
		"Test Parent", "parent@example.com",
	); err != nil {
		return err
	}

	if _, err := testPool.Exec(ctx,
		`INSERT INTO students (parent_id, name) VALUES
			(1, 'Student One'),
			(1, 'Student Two')`,
	); err != nil {
		return err
	}

	if _, err := testPool.Exec(ctx,
		`INSERT INTO trial_classes (title, subject, starts_at, capacity)
		 VALUES ($1, $2, NOW() + INTERVAL '1 day', 4)`,
		"Algebra Basics", "Math",
	); err != nil {
		return err
	}

	return nil
}

// freshFixtures resets the database between tests that mutate data.
func freshFixtures(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := setupFixtures(ctx); err != nil {
		t.Fatalf("reset fixtures: %v", err)
	}
}

func TestCreateAndGetBooking(t *testing.T) {
	ctx := context.Background()
	freshFixtures(t, ctx)

	repo := NewBookingRepository(testPool)

	created, err := repo.Create(ctx, nil, 1, 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Status != domain.BookingStatusPendingPayment {
		t.Fatalf("expected pending_payment status, got %q", created.Status)
	}

	got, err := repo.GetByID(ctx, nil, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("ID mismatch: want %d, got %d", created.ID, got.ID)
	}
	if got.StudentID != 1 {
		t.Errorf("StudentID mismatch: want 1, got %d", got.StudentID)
	}
	if got.TrialClassID != 1 {
		t.Errorf("TrialClassID mismatch: want 1, got %d", got.TrialClassID)
	}
	if got.Status != domain.BookingStatusPendingPayment {
		t.Errorf("Status mismatch: want %q, got %q", domain.BookingStatusPendingPayment, got.Status)
	}
	if got.IsConfirmed() {
		t.Error("new booking should not be confirmed")
	}
	if !got.IsPendingPayment() {
		t.Error("new booking should be pending payment")
	}
}

func TestCountConfirmedByClass(t *testing.T) {
	ctx := context.Background()
	freshFixtures(t, ctx)

	repo := NewBookingRepository(testPool)

	b1, err := repo.Create(ctx, nil, 1, 1)
	if err != nil {
		t.Fatalf("Create b1: %v", err)
	}
	b2, err := repo.Create(ctx, nil, 2, 1)
	if err != nil {
		t.Fatalf("Create b2: %v", err)
	}

	// Initially nothing is confirmed.
	count, err := repo.CountConfirmedByClass(ctx, nil, 1)
	if err != nil {
		t.Fatalf("CountConfirmedByClass (initial): %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 confirmed, got %d", count)
	}

	if err := repo.UpdateStatus(ctx, nil, b1.ID, domain.BookingStatusConfirmed); err != nil {
		t.Fatalf("UpdateStatus b1: %v", err)
	}
	if err := repo.UpdateStatus(ctx, nil, b2.ID, domain.BookingStatusConfirmed); err != nil {
		t.Fatalf("UpdateStatus b2: %v", err)
	}

	count, err = repo.CountConfirmedByClass(ctx, nil, 1)
	if err != nil {
		t.Fatalf("CountConfirmedByClass (after confirm): %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 confirmed, got %d", count)
	}
}

func TestHasConfirmedBooking(t *testing.T) {
	ctx := context.Background()
	freshFixtures(t, ctx)

	repo := NewBookingRepository(testPool)

	b, err := repo.Create(ctx, nil, 1, 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	has, err := repo.HasConfirmedBooking(ctx, nil, 1, 1)
	if err != nil {
		t.Fatalf("HasConfirmedBooking (before): %v", err)
	}
	if has {
		t.Fatal("expected no confirmed booking before confirmation")
	}

	if err := repo.UpdateStatus(ctx, nil, b.ID, domain.BookingStatusConfirmed); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	has, err = repo.HasConfirmedBooking(ctx, nil, 1, 1)
	if err != nil {
		t.Fatalf("HasConfirmedBooking (after): %v", err)
	}
	if !has {
		t.Fatal("expected confirmed booking to be reported")
	}
}

func TestClassList(t *testing.T) {
	ctx := context.Background()
	freshFixtures(t, ctx)

	classRepo := NewClassRepository(testPool)
	bookingRepo := NewBookingRepository(testPool)

	b, err := bookingRepo.Create(ctx, nil, 1, 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := bookingRepo.UpdateStatus(ctx, nil, b.ID, domain.BookingStatusConfirmed); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	classes, err := classRepo.List(ctx, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(classes))
	}

	c := classes[0]
	if c.ID != 1 {
		t.Errorf("ID mismatch: want 1, got %d", c.ID)
	}
	if c.Title != "Algebra Basics" {
		t.Errorf("Title mismatch: want %q, got %q", "Algebra Basics", c.Title)
	}
	if c.Capacity != 4 {
		t.Errorf("Capacity mismatch: want 4, got %d", c.Capacity)
	}
	if c.ConfirmedCount != 1 {
		t.Errorf("ConfirmedCount mismatch: want 1, got %d", c.ConfirmedCount)
	}
	if c.RemainingSeats != 3 {
		t.Errorf("RemainingSeats mismatch: want 3, got %d", c.RemainingSeats)
	}
}
