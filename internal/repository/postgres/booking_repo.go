package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/domain"
)

// BookingRepository is the pgx-backed implementation of domain.BookingRepository.
type BookingRepository struct {
	pool *pgxpool.Pool
}

// NewBookingRepository constructs a BookingRepository backed by the given pool.
func NewBookingRepository(pool *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{pool: pool}
}

// Create inserts a new booking in the pending_payment state.
func (r *BookingRepository) Create(ctx context.Context, tx domain.Tx, studentID int64, trialClassID int64) (*domain.Booking, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		INSERT INTO bookings (student_id, trial_class_id, status)
		VALUES ($1, $2, 'pending_payment')
		RETURNING id, student_id, trial_class_id, status, created_at, updated_at`

	var b domain.Booking
	err := db.QueryRow(ctx, query, studentID, trialClassID).Scan(
		&b.ID, &b.StudentID, &b.TrialClassID, &b.Status, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create booking: %w", err)
	}

	b.Status = domain.BookingStatusPendingPayment

	return &b, nil
}

// GetByID returns a single booking by id.
func (r *BookingRepository) GetByID(ctx context.Context, tx domain.Tx, id int64) (*domain.Booking, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT id, student_id, trial_class_id, status, created_at, updated_at
		FROM bookings
		WHERE id = $1`

	var b domain.Booking
	err := db.QueryRow(ctx, query, id).Scan(
		&b.ID, &b.StudentID, &b.TrialClassID, &b.Status, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("get booking by id: %w", err)
	}

	return &b, nil
}

// HasConfirmedBooking reports whether the student already has a confirmed
// booking for the given class.
func (r *BookingRepository) HasConfirmedBooking(ctx context.Context, tx domain.Tx, studentID int64, trialClassID int64) (bool, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT EXISTS (
			SELECT 1 FROM bookings
			WHERE student_id = $1 AND trial_class_id = $2 AND status = 'confirmed'
		)`

	var exists bool
	if err := db.QueryRow(ctx, query, studentID, trialClassID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check confirmed booking: %w", err)
	}

	return exists, nil
}

// HasPendingBooking reports whether the student already has a pending-payment
// booking for the given class.
func (r *BookingRepository) HasPendingBooking(ctx context.Context, tx domain.Tx, studentID int64, trialClassID int64) (bool, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT EXISTS (
			SELECT 1 FROM bookings
			WHERE student_id = $1 AND trial_class_id = $2 AND status = 'pending_payment'
		)`

	var exists bool
	if err := db.QueryRow(ctx, query, studentID, trialClassID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check pending booking: %w", err)
	}

	return exists, nil
}

// CountConfirmedByClass returns the number of confirmed bookings for a class.
func (r *BookingRepository) CountConfirmedByClass(ctx context.Context, tx domain.Tx, trialClassID int64) (int, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT COUNT(*) FROM bookings
		WHERE trial_class_id = $1 AND status = 'confirmed'`

	var count int
	if err := db.QueryRow(ctx, query, trialClassID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count confirmed bookings: %w", err)
	}

	return count, nil
}

// UpdateStatus changes a booking's status and bumps updated_at.
func (r *BookingRepository) UpdateStatus(ctx context.Context, tx domain.Tx, id int64, status domain.BookingStatus) error {
	db := resolveDBTX(r.pool, tx)

	const query = `
		UPDATE bookings SET status = $2, updated_at = NOW()
		WHERE id = $1`

	tag, err := db.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("update booking status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrBookingNotFound
	}

	return nil
}

// GetClassBookings returns all bookings for a class enriched with student and
// parent details. Confirmed bookings are ordered first (they are the real
// roster), followed by pending, failed, and cancelled.
func (r *BookingRepository) GetClassBookings(ctx context.Context, tx domain.Tx, trialClassID int64) ([]domain.RosterEntry, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT
			b.id          AS booking_id,
			s.id          AS student_id,
			s.name        AS student_name,
			p.name        AS parent_name,
			p.email       AS parent_email,
			b.status,
			b.created_at
		FROM bookings b
		JOIN students s ON s.id = b.student_id
		JOIN parents  p ON p.id = s.parent_id
		WHERE b.trial_class_id = $1
		ORDER BY
			CASE b.status
				WHEN 'confirmed'       THEN 0
				WHEN 'pending_payment' THEN 1
				WHEN 'payment_failed'  THEN 2
				WHEN 'cancelled'       THEN 3
			END,
			b.created_at ASC`

	rows, err := db.Query(ctx, query, trialClassID)
	if err != nil {
		return nil, fmt.Errorf("list class bookings: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.RosterEntry, 0)
	for rows.Next() {
		var e domain.RosterEntry
		if err := rows.Scan(
			&e.BookingID, &e.StudentID, &e.StudentName,
			&e.ParentName, &e.ParentEmail, &e.Status, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan roster entry: %w", err)
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate class bookings: %w", err)
	}

	return entries, nil
}
