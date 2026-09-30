package response

import (
	"encoding/json"
	"net/http"
)

// ValidationError is the payload shape for request validation failures.
type ValidationError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// ValidationEnvelope is the standard validation failure response shape.
type ValidationEnvelope struct {
	Error ValidationError `json:"error"`
}

// WriteValidationError writes a 400 response with per-field validation details.
func WriteValidationError(w http.ResponseWriter, fields map[string]string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	_ = json.NewEncoder(w).Encode(ValidationEnvelope{
		Error: ValidationError{
			Code:    "validation_error",
			Message: "request validation failed",
			Fields:  fields,
		},
	})
}
