package domain

import "time"

// BookingStatus is the lifecycle state of a booking.
type BookingStatus string

const (
	BookingStatusPendingPayment BookingStatus = "pending_payment"
	BookingStatusConfirmed      BookingStatus = "confirmed"
	BookingStatusPaymentFailed  BookingStatus = "payment_failed"
	BookingStatusCancelled      BookingStatus = "cancelled"
)

// Booking represents a student's seat in a trial class.
type Booking struct {
	ID           int64
	StudentID    int64
	TrialClassID int64
	Status       BookingStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsConfirmed reports whether the booking holds a confirmed seat.
func (b *Booking) IsConfirmed() bool {
	return b.Status == BookingStatusConfirmed
}

// IsPendingPayment reports whether the booking is awaiting payment.
func (b *Booking) IsPendingPayment() bool {
	return b.Status == BookingStatusPendingPayment
}
