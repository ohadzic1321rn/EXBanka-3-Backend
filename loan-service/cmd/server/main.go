package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/config"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/database"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/handler"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/middleware"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/repository"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		slog.Error("DB connection failed", "error", err)
		os.Exit(1)
	}
	if err := database.Migrate(db); err != nil {
		slog.Error("DB migration failed", "error", err)
		os.Exit(1)
	}

	loanRepo := repository.NewLoanRepository(db)
	installmentRepo := repository.NewInstallmentRepository(db)
	loanSvc := service.NewLoanService(loanRepo, installmentRepo)
	loanH := handler.NewLoanHandler(loanSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthCheck)
	mux.Handle("/api/v1/loans/", middleware.CORS(loanH))

	httpServer := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: middleware.CORS(mux),
	}

	go func() {
		slog.Info("Loan-service HTTP server listening", "port", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down loan-service gracefully")
	if err := httpServer.Close(); err != nil {
		slog.Error("HTTP shutdown error", "error", err)
	}
	slog.Info("loan-service stopped")
}

func healthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"ok","service":"loan-service"}`)
}
