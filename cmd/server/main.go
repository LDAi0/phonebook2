package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LDAi0/phonebook/internal/config"
	"github.com/LDAi0/phonebook/internal/events"
	"github.com/LDAi0/phonebook/internal/repository"
	"github.com/LDAi0/phonebook/internal/service"
	"github.com/LDAi0/phonebook/internal/transport/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}
	log := cfg.NewLogger()

	if err := run(cfg, log); err != nil {
		log.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	publisher := events.NewKafkaPublisher(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer publisher.Close()

	// data layer -> business layer -> presentation layer
	contactRepo := repository.NewContactRepo(pool)
	contactSvc := service.NewContactService(contactRepo, publisher, log)
	router := httpapi.NewRouter(log, contactSvc, pool.Ping)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server started", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
