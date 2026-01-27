package events

import (
	"context"
	"log"

	"github.com/segmentio/kafka-go"
)

type Comsumer struct {
	Reader *kafka.Reader
}

func NewConsumer(brokers []string, topic string, groupID string) *Comsumer {
	
	newKafkaReadConfig := kafka.ReaderConfig{
		Brokers: brokers,
		GroupID: groupID,
		Topic: topic,
	}

	newKafkaReader := kafka.NewReader(newKafkaReadConfig)

	return &Comsumer{
		Reader: newKafkaReader,
	}
}

func (c *Comsumer) Listen(ctx context.Context)  {
	for {
		msg, err := c.Reader.ReadMessage(ctx)
		if err != nil {
			log.Printf("Could not read message: %v", err)
			continue // if there is an error we move to the next iteration
		}

		//Print what was recvied 
		// msg.Value is []byte, so we cast it to string to read it
		log.Printf("Recevied message: %s", string(msg.Value))
	}
}