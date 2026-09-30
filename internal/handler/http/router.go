package http

import (
	"encoding/json"
	"net/http"

	"trial-booking/internal/usecase"
)

// apiV1 is the versioned base path for all business API routes. The operational
// /healthz endpoint is intentionally left unversioned.
const apiV1 = "/api/v1"

// NewRouter builds the HTTP handler with all routes registered and wraps it
// with the recovery, logging, and CORS middleware.
func NewRouter(uc usecase.BookingUsecase) http.Handler {
	classHandler := NewClassHandler(uc)
	parentHandler := NewParentHandler(uc)
	bookingHandler := NewBookingHandler(uc)
	paymentHandler := NewPaymentHandler(uc)
	rosterHandler := NewRosterHandler(uc)

	mux := http.NewServeMux()

	// Operational endpoint (unversioned).
	mux.HandleFunc("GET /healthz", healthz)

	// Versioned business API.
	mux.HandleFunc("GET "+apiV1+"/classes", classHandler.List)
	mux.HandleFunc("GET "+apiV1+"/classes/{id}", classHandler.Get)
	mux.HandleFunc("GET "+apiV1+"/parents/{id}", parentHandler.Get)
	mux.HandleFunc("POST "+apiV1+"/bookings", bookingHandler.Create)
	mux.HandleFunc("GET "+apiV1+"/bookings/{id}", bookingHandler.Get)
	mux.HandleFunc("POST "+apiV1+"/bookings/{id}/payment", paymentHandler.ProcessPayment)
	mux.HandleFunc("GET "+apiV1+"/admin/classes/{id}/roster", rosterHandler.Get)

	// Static frontend. The specific /api/v1/... routes above take precedence;
	// "/" is a catch-all for everything else.
	mux.Handle("/", webHandler())

	return Chain(mux, RecoveryMiddleware, LoggingMiddleware, CORSMiddleware)
}

// healthz is a simple liveness endpoint. It intentionally returns a bare
// {"status":"ok"} (not the data envelope) so it is trivially consumable by
// infrastructure health checks.
func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
