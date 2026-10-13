package events

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/LDAi0/phonebook/internal/domain"
)

type HandlerFunc func(ctx context.Context, evt domain.ContactEvent) error

type Consumer struct {
	reader *kafka.Reader
	handle HandlerFunc
	log    *slog.Logger
}

func NewConsumer(brokers []string, topic, groupID string, h HandlerFunc, log *slog.Logger) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     groupID,
			MinBytes:    1,
			MaxBytes:    10 << 20,
			MaxWait:     time.Second,
			StartOffset: kafka.FirstOffset, // used only when the group has no committed offset
		}),
		handle: h,
		log:    log,
	}
}

// Run blocks until ctx is cancelled. Offsets are committed only after a
// message was handled successfully (at-least-once delivery).
func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				return nil
			}
			c.log.Error("fetch message failed", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
			continue
		}

		if err := c.process(ctx, msg); err != nil {
			return nil // shutting down: leave the message uncommitted
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.log.Error("commit failed", "error", err)
		}
	}
}

// process returns an error only when ctx was cancelled before the message was handled.
func (c *Consumer) process(ctx context.Context, msg kafka.Message) error {
	var evt domain.ContactEvent
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		// Poison message: retrying cannot help, so skip it.
		c.log.Error("skip malformed message", "partition", msg.Partition, "offset", msg.Offset, "error", err)
		return nil
	}

	backoff := 500 * time.Millisecond
	for {
		err := c.handle(ctx, evt)
		if err == nil {
			return nil
		}
		c.log.Error("handle event failed, will retry", "event_id", evt.ID, "error", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (c *Consumer) Close() error { return c.reader.Close() }
