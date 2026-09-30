package domain

import "context"

// ClassRepository abstracts persistence for trial classes.
type ClassRepository interface {
	GetByID(ctx context.Context, tx Tx, id int64) (*TrialClass, error)
	List(ctx context.Context, tx Tx) ([]ClassWithAvailability, error)
	LockByID(ctx context.Context, tx Tx, id int64) (*TrialClass, error)
}

// BookingRepository abstracts persistence for bookings.
type BookingRepository interface {
	Create(ctx context.Context, tx Tx, studentID int64, trialClassID int64) (*Booking, error)
	GetByID(ctx context.Context, tx Tx, id int64) (*Booking, error)
	HasConfirmedBooking(ctx context.Context, tx Tx, studentID int64, trialClassID int64) (bool, error)
	HasPendingBooking(ctx context.Context, tx Tx, studentID int64, trialClassID int64) (bool, error)
	CountConfirmedByClass(ctx context.Context, tx Tx, trialClassID int64) (int, error)
	UpdateStatus(ctx context.Context, tx Tx, id int64, status BookingStatus) error
	GetClassBookings(ctx context.Context, tx Tx, trialClassID int64) ([]RosterEntry, error)
}

// PaymentRepository abstracts persistence for payment attempts.
type PaymentRepository interface {
	CreateAttempt(ctx context.Context, tx Tx, bookingID int64, status PaymentStatus, providerReference string, failureReason string) (*PaymentAttempt, error)
}

// StudentRepository abstracts persistence for students.
type StudentRepository interface {
	GetByID(ctx context.Context, tx Tx, id int64) (*Student, error)
	GetByParentID(ctx context.Context, tx Tx, parentID int64) ([]Student, error)
}

// ParentRepository abstracts persistence for parents.
type ParentRepository interface {
	GetByID(ctx context.Context, tx Tx, id int64) (*Parent, error)
}
