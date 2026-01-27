package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/segmentio/kafka-go"
)

type Producer struct {
	Writer *kafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	return &Producer{
		Writer: writer,
	}
}

func (p *Producer) SendBrandSearch(ctx context.Context, event domain.BrandMoniterEvent) error {
	eventInByte, err := json.Marshal(event)
	if err != nil {
		log.Printf("Could not convert event to json: %v", err)
		return err
	}

	return p.Writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.ProjectID),
		Value: eventInByte,
	})
}
