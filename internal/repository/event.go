package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/LDAi0/phonebook/internal/domain"
)

type EventRepo struct {
	pool *pgxpool.Pool
}

func NewEventRepo(pool *pgxpool.Pool) *EventRepo {
	return &EventRepo{pool: pool}
}

// Save stores the event; duplicates (same event_id) are ignored, so
// Kafka's at-least-once delivery cannot create duplicate audit rows.
func (r *EventRepo) Save(ctx context.Context, evt domain.ContactEvent) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO contact_events (event_id, event_type, contact_id, payload, occurred_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (event_id) DO NOTHING`,
		evt.ID, string(evt.Type), evt.ContactID, payload, evt.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert contact event: %w", err)
	}
	return nil
}
