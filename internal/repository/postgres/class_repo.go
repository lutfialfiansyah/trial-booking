package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/domain"
)

// ClassRepository is the pgx-backed implementation of domain.ClassRepository.
type ClassRepository struct {
	pool *pgxpool.Pool
}

// NewClassRepository constructs a ClassRepository backed by the given pool.
func NewClassRepository(pool *pgxpool.Pool) *ClassRepository {
	return &ClassRepository{pool: pool}
}

// GetByID returns a single trial class by id.
func (r *ClassRepository) GetByID(ctx context.Context, tx domain.Tx, id int64) (*domain.TrialClass, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT id, title, subject, starts_at, capacity, created_at
		FROM trial_classes
		WHERE id = $1`

	var c domain.TrialClass
	err := db.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.Title, &c.Subject, &c.StartsAt, &c.Capacity, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrClassNotFound
		}
		return nil, fmt.Errorf("get trial class by id: %w", err)
	}

	return &c, nil
}

// List returns all trial classes with their confirmed counts and remaining seats.
func (r *ClassRepository) List(ctx context.Context, tx domain.Tx) ([]domain.ClassWithAvailability, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT
			tc.id, tc.title, tc.subject, tc.starts_at, tc.capacity, tc.created_at,
			COUNT(b.id) FILTER (WHERE b.status = 'confirmed')::int AS confirmed_count
		FROM trial_classes tc
		LEFT JOIN bookings b ON b.trial_class_id = tc.id
		GROUP BY tc.id
		ORDER BY tc.starts_at ASC`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list trial classes: %w", err)
	}
	defer rows.Close()

	classes := make([]domain.ClassWithAvailability, 0)
	for rows.Next() {
		var c domain.ClassWithAvailability
		if err := rows.Scan(
			&c.ID, &c.Title, &c.Subject, &c.StartsAt, &c.Capacity, &c.CreatedAt,
			&c.ConfirmedCount,
		); err != nil {
			return nil, fmt.Errorf("scan trial class: %w", err)
		}

		c.RemainingSeats = c.Capacity - c.ConfirmedCount
		if c.RemainingSeats < 0 {
			c.RemainingSeats = 0
		}

		classes = append(classes, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trial classes: %w", err)
	}

	return classes, nil
}

// LockByID returns a trial class and acquires a row-level lock on it.
//
// It uses SELECT ... FOR UPDATE so that concurrent seat-claiming operations
// (e.g. payment confirmations) serialize on the class row, preventing the
// last-seat race condition.
//
// NOTE: FOR UPDATE only takes effect inside a transaction. This method must be
// called with a real pgx.Tx (i.e. within TxManager.WithTx). resolveDBTX will
// pass the pgx.Tx through when one is supplied; if it falls back to the pool,
// the statement runs in its own implicit transaction and the lock is released
// immediately, which provides no protection.
func (r *ClassRepository) LockByID(ctx context.Context, tx domain.Tx, id int64) (*domain.TrialClass, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT id, title, subject, starts_at, capacity, created_at
		FROM trial_classes
		WHERE id = $1
		FOR UPDATE`

	var c domain.TrialClass
	err := db.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.Title, &c.Subject, &c.StartsAt, &c.Capacity, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrClassNotFound
		}
		return nil, fmt.Errorf("lock trial class by id: %w", err)
	}

	return &c, nil
}
