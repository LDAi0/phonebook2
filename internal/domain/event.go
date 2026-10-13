package domain

import "time"

type EventType string

const (
	EventContactCreated EventType = "contact.created"
	EventContactUpdated EventType = "contact.updated"
	EventContactDeleted EventType = "contact.deleted"
)

// ContactEvent is the message published to Kafka after every change.
type ContactEvent struct {
	ID         string    `json:"id"`
	Type       EventType `json:"type"`
	ContactID  int64     `json:"contact_id"`
	Contact    *Contact  `json:"contact,omitempty"` // nil for deletions
	OccurredAt time.Time `json:"occurred_at"`
}
