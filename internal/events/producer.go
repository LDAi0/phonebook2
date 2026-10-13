// Package events contains the Kafka adapters (producer and consumer).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/LDAi0/phonebook/internal/domain"
)

type KafkaPublisher struct {
	writer *kafka.Writer
}

func NewKafkaPublisher(brokers []string, topic string) *KafkaPublisher {
	return &KafkaPublisher{writer: &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{}, // same contact -> same partition -> ordered events
		RequiredAcks:           kafka.RequireAll,
		BatchTimeout:           10 * time.Millisecond,
		WriteTimeout:           5 * time.Second,
		AllowAutoTopicCreation: true,
	}}
}

func (p *KafkaPublisher) Publish(ctx context.Context, evt domain.ContactEvent) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:     []byte(strconv.FormatInt(evt.ContactID, 10)),
		Value:   payload,
		Headers: []kafka.Header{{Key: "event_type", Value: []byte(evt.Type)}},
	})
	if err != nil {
		return fmt.Errorf("write kafka message: %w", err)
	}
	return nil
}

func (p *KafkaPublisher) Close() error { return p.writer.Close() }
