package kafka

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Producer struct {
	client *kgo.Client
}

func NewProducer(brokers []string, clientID string) (*Producer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID),
	)
	if err != nil {
		return nil, err
	}
	return &Producer{client: cl}, nil
}

func (p *Producer) Publish(ctx context.Context, topic string, key, value []byte) error {
	rec := &kgo.Record{
		Topic: topic,
		Key:   key,
		Value: value,
	}
	return p.client.ProduceSync(ctx, rec).FirstErr()
}

func (p *Producer) Close() {
	p.client.Close()
}
