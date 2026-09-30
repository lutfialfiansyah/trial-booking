package domain

import "time"

// PaymentStatus is the lifecycle state of a payment attempt.
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
)

// PaymentAttempt records one attempt to pay for a booking.
type PaymentAttempt struct {
	ID                int64
	BookingID         int64
	Status            PaymentStatus
	ProviderReference string
	FailureReason     string
	CreatedAt         time.Time
}
