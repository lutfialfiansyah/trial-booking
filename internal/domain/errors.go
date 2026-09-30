package domain

import "errors"

// Sentinel domain errors. These are pure domain errors with no knowledge of
// HTTP, transport, or infrastructure concerns.
var (
	ErrStudentNotFound           = errors.New("student not found")
	ErrClassNotFound             = errors.New("trial class not found")
	ErrBookingNotFound           = errors.New("booking not found")
	ErrParentNotFound            = errors.New("parent not found")
	ErrClassFull                 = errors.New("trial class is full")
	ErrSeatNoLongerAvailable     = errors.New("seat is no longer available")
	ErrDuplicateConfirmedBooking = errors.New("student already has a confirmed booking for this class")
	ErrDuplicatePendingBooking   = errors.New("student already has a pending booking for this class")
	ErrPaymentFailed             = errors.New("payment failed")
	ErrBookingNotPayable         = errors.New("booking is not in a payable state")
	ErrStudentNotOwnedByParent   = errors.New("student does not belong to this parent")
)
