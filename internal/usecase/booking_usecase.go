package usecase

import (
	"context"

	"trial-booking/internal/domain"
)

// BookingUsecase is the application-facing interface for booking operations.
// HTTP handlers depend on this interface, never on the concrete implementation.
type BookingUsecase interface {
	CreateBooking(ctx context.Context, parentID, studentID, trialClassID int64) (*domain.Booking, error)
	ProcessPayment(ctx context.Context, bookingID int64, providerRef string, paymentSucceeded bool) (*domain.Booking, error)
	GetRoster(ctx context.Context, trialClassID int64) (*domain.Roster, error)
	ListClasses(ctx context.Context) ([]domain.ClassWithAvailability, error)
	GetBooking(ctx context.Context, bookingID int64) (*domain.Booking, error)
	GetParentWithChildren(ctx context.Context, parentID int64) (*domain.ParentWithChildren, error)
}

// bookingUsecase orchestrates the domain repositories inside transactions.
// It depends only on domain interfaces and types.
type bookingUsecase struct {
	txManager   domain.TxManager
	classRepo   domain.ClassRepository
	bookingRepo domain.BookingRepository
	paymentRepo domain.PaymentRepository
	studentRepo domain.StudentRepository
	parentRepo  domain.ParentRepository
}

// NewBookingUsecase wires the usecase with its dependencies and returns the
// interface type.
func NewBookingUsecase(
	txManager domain.TxManager,
	classRepo domain.ClassRepository,
	bookingRepo domain.BookingRepository,
	paymentRepo domain.PaymentRepository,
	studentRepo domain.StudentRepository,
	parentRepo domain.ParentRepository,
) BookingUsecase {
	return &bookingUsecase{
		txManager:   txManager,
		classRepo:   classRepo,
		bookingRepo: bookingRepo,
		paymentRepo: paymentRepo,
		studentRepo: studentRepo,
		parentRepo:  parentRepo,
	}
}

// CreateBooking creates a pending-payment booking for a student in a class.
//
// The whole operation runs in a transaction. It locks the class row
// (LockByID -> SELECT ... FOR UPDATE) so that concurrent booking creations for
// the same class serialize, which prevents overbooking and duplicate pending
// bookings from being created at the same time.
func (u *bookingUsecase) CreateBooking(ctx context.Context, parentID, studentID, trialClassID int64) (*domain.Booking, error) {
	var booking *domain.Booking

	err := u.txManager.WithTx(ctx, func(tx domain.Tx) error {
		student, err := u.studentRepo.GetByID(ctx, tx, studentID)
		if err != nil {
			return err
		}

		if student.ParentID != parentID {
			return domain.ErrStudentNotOwnedByParent
		}

		// Lock the class row so concurrent bookings for the same class
		// serialize; this prevents overbooking and duplicate pending bookings
		// being created at the same time.
		class, err := u.classRepo.LockByID(ctx, tx, trialClassID)
		if err != nil {
			return err
		}

		hasConfirmed, err := u.bookingRepo.HasConfirmedBooking(ctx, tx, studentID, trialClassID)
		if err != nil {
			return err
		}
		if hasConfirmed {
			return domain.ErrDuplicateConfirmedBooking
		}

		hasPending, err := u.bookingRepo.HasPendingBooking(ctx, tx, studentID, trialClassID)
		if err != nil {
			return err
		}
		if hasPending {
			return domain.ErrDuplicatePendingBooking
		}

		// Only confirmed bookings consume capacity, so a pending booking is
		// always allowed to be created as long as confirmed < capacity.
		confirmedCount, err := u.bookingRepo.CountConfirmedByClass(ctx, tx, trialClassID)
		if err != nil {
			return err
		}
		if confirmedCount >= class.Capacity {
			return domain.ErrClassFull
		}

		booking, err = u.bookingRepo.Create(ctx, tx, studentID, trialClassID)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return booking, nil
}

// ProcessPayment records the outcome of a payment attempt and, on success,
// confirms the seat.
//
// RACE SAFETY (last-seat problem):
// Payments for a class may arrive concurrently. If two payments each read
// "confirmed=3, capacity=4" they would both think a seat is free and both
// confirm, producing an overbooking. We prevent this by (a) taking a row-level
// lock on the class via LockByID (SELECT ... FOR UPDATE) so all competing
// confirmations for the same class serialize, and (b) re-reading the booking
// status *after* acquiring the lock so we observe an authoritative value.
//
// BUSINESS-ERROR PATTERN:
// When a payment fails for a business reason (declined, seat gone, duplicate),
// we have deliberately written rows inside the transaction (a failed
// payment_attempt and a payment_failed booking status) that we want to KEEP.
// Returning an error from the closure would cause WithTx to ROLLBACK and lose
// them. Instead we capture the outcome in businessErr and return nil so the
// transaction COMMITS; the captured error is returned to the caller afterwards.
// Genuine failures (DB errors, not-found, not-payable) make no writes and are
// returned from the closure so the (no-op) rollback loses nothing.
func (u *bookingUsecase) ProcessPayment(ctx context.Context, bookingID int64, providerRef string, paymentSucceeded bool) (*domain.Booking, error) {
	var booking *domain.Booking
	var businessErr error

	txErr := u.txManager.WithTx(ctx, func(tx domain.Tx) error {
		// Initial read only to discover the class id.
		b, err := u.bookingRepo.GetByID(ctx, tx, bookingID)
		if err != nil {
			return err
		}

		class, err := u.classRepo.LockByID(ctx, tx, b.TrialClassID)
		if err != nil {
			return err
		}

		// Re-read under the lock for an authoritative status.
		b, err = u.bookingRepo.GetByID(ctx, tx, bookingID)
		if err != nil {
			return err
		}

		// No writes so far, so returning here is safe (rollback is a no-op).
		if b.Status != domain.BookingStatusPendingPayment {
			return domain.ErrBookingNotPayable
		}

		if !paymentSucceeded {
			if _, err := u.paymentRepo.CreateAttempt(ctx, tx, bookingID, domain.PaymentStatusFailed, providerRef, "payment_declined"); err != nil {
				return err
			}
			if err := u.bookingRepo.UpdateStatus(ctx, tx, bookingID, domain.BookingStatusPaymentFailed); err != nil {
				return err
			}
			businessErr = domain.ErrPaymentFailed
			return nil
		}

		// Re-check availability while still holding the class lock.
		hasConfirmed, err := u.bookingRepo.HasConfirmedBooking(ctx, tx, b.StudentID, class.ID)
		if err != nil {
			return err
		}
		if hasConfirmed {
			if _, err := u.paymentRepo.CreateAttempt(ctx, tx, bookingID, domain.PaymentStatusFailed, providerRef, "duplicate_confirmed_booking"); err != nil {
				return err
			}
			if err := u.bookingRepo.UpdateStatus(ctx, tx, bookingID, domain.BookingStatusPaymentFailed); err != nil {
				return err
			}
			businessErr = domain.ErrDuplicateConfirmedBooking
			return nil
		}

		confirmedCount, err := u.bookingRepo.CountConfirmedByClass(ctx, tx, class.ID)
		if err != nil {
			return err
		}
		if confirmedCount >= class.Capacity {
			if _, err := u.paymentRepo.CreateAttempt(ctx, tx, bookingID, domain.PaymentStatusFailed, providerRef, "seat_no_longer_available"); err != nil {
				return err
			}
			if err := u.bookingRepo.UpdateStatus(ctx, tx, bookingID, domain.BookingStatusPaymentFailed); err != nil {
				return err
			}
			businessErr = domain.ErrSeatNoLongerAvailable
			return nil
		}

		if _, err := u.paymentRepo.CreateAttempt(ctx, tx, bookingID, domain.PaymentStatusSucceeded, providerRef, ""); err != nil {
			return err
		}
		if err := u.bookingRepo.UpdateStatus(ctx, tx, bookingID, domain.BookingStatusConfirmed); err != nil {
			return err
		}

		booking = b
		booking.Status = domain.BookingStatusConfirmed
		return nil
	})

	if txErr != nil {
		// Rolled back; nothing to keep.
		return nil, txErr
	}
	if businessErr != nil {
		// Committed; surface the business outcome.
		return nil, businessErr
	}

	return booking, nil
}

// GetRoster returns a class with all of its bookings. Read-only; it runs in a
// transaction for a consistent snapshot but takes no locks.
//
// All bookings are returned for admin visibility, but only confirmed bookings
// count toward capacity (ConfirmedCount and RemainingSeats).
func (u *bookingUsecase) GetRoster(ctx context.Context, trialClassID int64) (*domain.Roster, error) {
	var roster *domain.Roster

	err := u.txManager.WithTx(ctx, func(tx domain.Tx) error {
		class, err := u.classRepo.GetByID(ctx, tx, trialClassID)
		if err != nil {
			return err
		}

		entries, err := u.bookingRepo.GetClassBookings(ctx, tx, trialClassID)
		if err != nil {
			return err
		}

		// Only confirmed bookings consume capacity.
		confirmedCount := 0
		for _, e := range entries {
			if e.Status == domain.BookingStatusConfirmed {
				confirmedCount++
			}
		}

		remainingSeats := class.Capacity - confirmedCount
		if remainingSeats < 0 {
			remainingSeats = 0
		}

		roster = &domain.Roster{
			Class:          *class,
			ConfirmedCount: confirmedCount,
			RemainingSeats: remainingSeats,
			TotalBookings:  len(entries),
			Entries:        entries,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return roster, nil
}

// ListClasses returns all trial classes with availability. Read-only.
func (u *bookingUsecase) ListClasses(ctx context.Context) ([]domain.ClassWithAvailability, error) {
	var classes []domain.ClassWithAvailability

	err := u.txManager.WithTx(ctx, func(tx domain.Tx) error {
		result, err := u.classRepo.List(ctx, tx)
		if err != nil {
			return err
		}
		classes = result
		return nil
	})
	if err != nil {
		return nil, err
	}

	if classes == nil {
		classes = make([]domain.ClassWithAvailability, 0)
	}

	return classes, nil
}

// GetBooking returns a single booking by id. Read-only.
func (u *bookingUsecase) GetBooking(ctx context.Context, bookingID int64) (*domain.Booking, error) {
	var booking *domain.Booking

	err := u.txManager.WithTx(ctx, func(tx domain.Tx) error {
		b, err := u.bookingRepo.GetByID(ctx, tx, bookingID)
		if err != nil {
			return err
		}
		booking = b
		return nil
	})
	if err != nil {
		return nil, err
	}

	return booking, nil
}

// GetParentWithChildren returns a parent and their students. Read-only; it runs
// in a transaction for a consistent snapshot but takes no locks.
func (u *bookingUsecase) GetParentWithChildren(ctx context.Context, parentID int64) (*domain.ParentWithChildren, error) {
	var result *domain.ParentWithChildren

	err := u.txManager.WithTx(ctx, func(tx domain.Tx) error {
		parent, err := u.parentRepo.GetByID(ctx, tx, parentID)
		if err != nil {
			return err
		}

		children, err := u.studentRepo.GetByParentID(ctx, tx, parentID)
		if err != nil {
			return err
		}

		result = &domain.ParentWithChildren{
			Parent:   *parent,
			Children: children,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}
