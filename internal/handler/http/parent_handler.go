package http

import (
	"net/http"
	"strconv"

	"trial-booking/internal/handler/http/response"
	"trial-booking/internal/usecase"
)

// ParentHandler serves parent endpoints.
type ParentHandler struct {
	uc usecase.BookingUsecase
}

// NewParentHandler constructs a ParentHandler.
func NewParentHandler(uc usecase.BookingUsecase) *ParentHandler {
	return &ParentHandler{uc: uc}
}

// Get handles GET /api/parents/{id} and returns the parent with their children.
func (h *ParentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.WriteValidationError(w, map[string]string{"id": "must be a number"})
		return
	}

	parent, err := h.uc.GetParentWithChildren(r.Context(), id)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.OK(w, parentWithChildrenToDTO(*parent))
}
