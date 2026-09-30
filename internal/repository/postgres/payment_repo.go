package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/domain"
)

// PaymentRepository is the pgx-backed implementation of domain.PaymentRepository.
type PaymentRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentRepository constructs a PaymentRepository backed by the given pool.
func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

// CreateAttempt records a payment attempt for a booking. providerReference and
// failureReason may be empty strings.
func (r *PaymentRepository) CreateAttempt(ctx context.Context, tx domain.Tx, bookingID int64, status domain.PaymentStatus, providerReference string, failureReason string) (*domain.PaymentAttempt, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		INSERT INTO payment_attempts (booking_id, status, provider_reference, failure_reason)
		VALUES ($1, $2, $3, $4)
		RETURNING id, booking_id, status, provider_reference, failure_reason, created_at`

	var p domain.PaymentAttempt
	err := db.QueryRow(ctx, query, bookingID, status, providerReference, failureReason).Scan(
		&p.ID, &p.BookingID, &p.Status, &p.ProviderReference, &p.FailureReason, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create payment attempt: %w", err)
	}

	return &p, nil
}
