package http

import (
	"net/http"
	"strconv"

	"trial-booking/internal/handler/http/response"
	"trial-booking/internal/usecase"
)

// RosterHandler serves the admin roster endpoint.
type RosterHandler struct {
	uc usecase.BookingUsecase
}

// NewRosterHandler constructs a RosterHandler.
func NewRosterHandler(uc usecase.BookingUsecase) *RosterHandler {
	return &RosterHandler{uc: uc}
}

// Get handles GET /api/admin/classes/{id}/roster.
func (h *RosterHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.WriteValidationError(w, map[string]string{"id": "must be a number"})
		return
	}

	roster, err := h.uc.GetRoster(r.Context(), id)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.OK(w, rosterToDTO(*roster))
}
