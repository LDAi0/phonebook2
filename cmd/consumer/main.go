package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/LDAi0/phonebook/internal/config"
	"github.com/LDAi0/phonebook/internal/events"
	"github.com/LDAi0/phonebook/internal/repository"
	"github.com/LDAi0/phonebook/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}
	log := cfg.NewLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect to database failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	audit := service.NewAuditService(repository.NewEventRepo(pool), log)
	consumer := events.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroupID, audit.Record, log)
	defer consumer.Close()

	log.Info("consumer started", "topic", cfg.KafkaTopic, "group", cfg.KafkaGroupID)
	if err := consumer.Run(ctx); err != nil {
		log.Error("consumer stopped with error", "error", err)
		os.Exit(1)
	}
	log.Info("consumer stopped")
}
