package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"trial-booking/internal/handler/http/response"
	"trial-booking/internal/usecase"
)

// PaymentHandler serves payment endpoints.
type PaymentHandler struct {
	uc usecase.BookingUsecase
}

// NewPaymentHandler constructs a PaymentHandler.
func NewPaymentHandler(uc usecase.BookingUsecase) *PaymentHandler {
	return &PaymentHandler{uc: uc}
}

// ProcessPayment handles POST /api/bookings/{id}/payment.
//
// Business failures (payment declined, seat gone, duplicate, not payable) are
// returned by the usecase as domain errors; WriteError maps them to the correct
// status codes.
func (h *PaymentHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.WriteValidationError(w, map[string]string{"id": "must be a number"})
		return
	}

	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteValidationError(w, map[string]string{"body": "invalid JSON body"})
		return
	}

	if req.ProviderRef == "" {
		req.ProviderRef = fmt.Sprintf("mock_%d", time.Now().UnixNano())
	}

	if fields := req.Validate(); len(fields) > 0 {
		response.WriteValidationError(w, fields)
		return
	}

	paymentSucceeded := req.Outcome == "success"

	booking, err := h.uc.ProcessPayment(r.Context(), id, req.ProviderRef, paymentSucceeded)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.OK(w, bookingToDTO(*booking))
}
