package http

import (
	"net/http"
	"strconv"

	"trial-booking/internal/domain"
	"trial-booking/internal/handler/http/response"
	"trial-booking/internal/usecase"
)

// ClassHandler serves trial class endpoints.
type ClassHandler struct {
	uc usecase.BookingUsecase
}

// NewClassHandler constructs a ClassHandler.
func NewClassHandler(uc usecase.BookingUsecase) *ClassHandler {
	return &ClassHandler{uc: uc}
}

// List handles GET /api/classes.
func (h *ClassHandler) List(w http.ResponseWriter, r *http.Request) {
	classes, err := h.uc.ListClasses(r.Context())
	if err != nil {
		response.WriteError(w, err)
		return
	}

	dtos := make([]ClassDTO, 0, len(classes))
	for _, c := range classes {
		dtos = append(dtos, classToDTO(c))
	}

	response.OK(w, dtos)
}

// Get handles GET /api/classes/{id}.
func (h *ClassHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		response.WriteValidationError(w, map[string]string{"id": "must be a number"})
		return
	}

	classes, err := h.uc.ListClasses(r.Context())
	if err != nil {
		response.WriteError(w, err)
		return
	}

	for _, c := range classes {
		if c.ID == id {
			response.OK(w, classToDTO(c))
			return
		}
	}

	response.WriteError(w, domain.ErrClassNotFound)
}
