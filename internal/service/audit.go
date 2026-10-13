package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/LDAi0/phonebook/internal/domain"
)

// EventRepository is implemented by the data layer.
type EventRepository interface {
	Save(ctx context.Context, evt domain.ContactEvent) error
}

// AuditService records every contact event consumed from Kafka.
type AuditService struct {
	repo EventRepository
	log  *slog.Logger
}

func NewAuditService(repo EventRepository, log *slog.Logger) *AuditService {
	return &AuditService{repo: repo, log: log}
}

func (s *AuditService) Record(ctx context.Context, evt domain.ContactEvent) error {
	if evt.ID == "" || evt.Type == "" {
		return errors.New("event without id or type")
	}
	if err := s.repo.Save(ctx, evt); err != nil {
		return err
	}
	s.log.Info("event recorded", "event_id", evt.ID, "type", evt.Type, "contact_id", evt.ContactID)
	return nil
}
