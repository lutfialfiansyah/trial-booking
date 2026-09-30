package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"trial-booking/internal/handler/http/response"
	"trial-booking/internal/usecase"
)

// BookingHandler serves booking endpoints.
type BookingHandler struct {
	uc usecase.BookingUsecase
}

// NewBookingHandler constructs a BookingHandler.
func NewBookingHandler(uc usecase.BookingUsecase) *BookingHandler {
	return &BookingHandler{uc: uc}
}

// Create handles POST /api/bookings.
func (h *BookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteValidationError(w, map[string]string{"body": "invalid JSON body"})
		return
	}

	if fields := req.Validate(); len(fields) > 0 {
		response.WriteValidationError(w, fields)
		return
	}

	booking, err := h.uc.CreateBooking(r.Context(), req.ParentID, req.StudentID, req.TrialClassID)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Created(w, bookingToDTO(*booking))
}

// Get handles GET /api/bookings/{id}.
func (h *BookingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.WriteValidationError(w, map[string]string{"id": "must be a number"})
		return
	}

	booking, err := h.uc.GetBooking(r.Context(), id)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.OK(w, bookingToDTO(*booking))
}
