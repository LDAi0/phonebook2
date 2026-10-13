-- Audit log filled by the Kafka consumer.
CREATE TABLE contact_events (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id     TEXT        NOT NULL UNIQUE, -- makes the consumer idempotent
    event_type   TEXT        NOT NULL,
    contact_id   BIGINT      NOT NULL,
    payload      JSONB       NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX contact_events_contact_id_idx ON contact_events (contact_id);
