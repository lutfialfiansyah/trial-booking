package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apihttp "trial-booking/internal/handler/http"
	"trial-booking/internal/repository/postgres"
	"trial-booking/internal/usecase"
)

const defaultDatabaseURL = "postgres://postgres:password@localhost:5432/trial-booking?sslmode=disable"

func main() {
	// NOTE: configuration is read inline for now; a dedicated config package
	// will be introduced in a later phase.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL
	}

	ctx := context.Background()

	// 1. Database connection pool.
	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	// 2. Repositories.
	txManager := postgres.NewTxManager(pool)
	classRepo := postgres.NewClassRepository(pool)
	bookingRepo := postgres.NewBookingRepository(pool)
	paymentRepo := postgres.NewPaymentRepository(pool)
	studentRepo := postgres.NewStudentRepository(pool)
	parentRepo := postgres.NewParentRepository(pool)

	// 3. Usecase.
	uc := usecase.NewBookingUsecase(txManager, classRepo, bookingRepo, paymentRepo, studentRepo, parentRepo)

	// 4. HTTP layer.
	handler := apihttp.NewRouter(uc)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 5. Graceful shutdown on SIGINT/SIGTERM.
	shutdownCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("trial-booking API listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-shutdownCtx.Done()

	shutdownTimeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownTimeoutCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	log.Println("trial-booking API stopped")
}
