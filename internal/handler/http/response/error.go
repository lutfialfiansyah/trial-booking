package response

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"trial-booking/internal/domain"
)

// Error is the error payload shape. Codes are lower_snake_case and
// i18n-ready by design.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorEnvelope is the standard error response shape.
type ErrorEnvelope struct {
	Error Error `json:"error"`
}

// AppError couples a domain error with its transport representation.
type AppError struct {
	Status  int
	Code    string
	Message string
}

// Error implements the error interface.
func (e *AppError) Error() string { return e.Message }

// WriteError maps a domain error to an HTTP response and writes it.
func WriteError(w http.ResponseWriter, err error) {
	appErr := mapDomainError(err)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(appErr.Status)

	_ = json.NewEncoder(w).Encode(ErrorEnvelope{
		Error: Error{
			Code:    appErr.Code,
			Message: appErr.Message,
		},
	})
}

// mapDomainError translates domain sentinel errors into transport errors.
// All codes are lower_snake_case.
func mapDomainError(err error) *AppError {
	switch {
	case errors.Is(err, domain.ErrStudentNotFound),
		errors.Is(err, domain.ErrClassNotFound),
		errors.Is(err, domain.ErrBookingNotFound),
		errors.Is(err, domain.ErrParentNotFound):
		return &AppError{
			Status:  http.StatusNotFound,
			Code:    "not_found",
			Message: err.Error(),
		}

	case errors.Is(err, domain.ErrClassFull):
		return &AppError{
			Status:  http.StatusConflict,
			Code:    "class_full",
			Message: err.Error(),
		}

	case errors.Is(err, domain.ErrSeatNoLongerAvailable):
		return &AppError{
			Status:  http.StatusConflict,
			Code:    "seat_unavailable",
			Message: err.Error(),
		}

	case errors.Is(err, domain.ErrDuplicateConfirmedBooking),
		errors.Is(err, domain.ErrDuplicatePendingBooking):
		return &AppError{
			Status:  http.StatusConflict,
			Code:    "duplicate_booking",
			Message: err.Error(),
		}

	case errors.Is(err, domain.ErrBookingNotPayable):
		return &AppError{
			Status:  http.StatusConflict,
			Code:    "booking_not_payable",
			Message: err.Error(),
		}

	case errors.Is(err, domain.ErrPaymentFailed):
		return &AppError{
			Status:  http.StatusPaymentRequired,
			Code:    "payment_failed",
			Message: err.Error(),
		}

	case errors.Is(err, domain.ErrStudentNotOwnedByParent):
		return &AppError{
			Status:  http.StatusForbidden,
			Code:    "forbidden",
			Message: err.Error(),
		}

	default:
		// Never expose internal error details to the client; log them instead.
		log.Printf("internal error: %v", err)
		return &AppError{
			Status:  http.StatusInternalServerError,
			Code:    "internal_error",
			Message: "something went wrong",
		}
	}
}
