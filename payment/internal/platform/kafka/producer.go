package kafka

import (
	"context"
	"time"

	"github.com/mao360/saga/payment/internal/observability"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Producer struct {
	client     *kgo.Client
	telemetry  *observability.Telemetry
	clientName string
}

func NewProducer(brokers []string, clientID string, telemetry *observability.Telemetry) (*Producer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID),
	)
	if err != nil {
		return nil, err
	}
	return &Producer{
		client:     cl,
		telemetry:  telemetry,
		clientName: clientID,
	}, nil
}

func (p *Producer) Publish(ctx context.Context, topic string, key, value []byte) error {
	start := time.Now()
	rec := &kgo.Record{
		Topic: topic,
		Key:   key,
		Value: value,
	}

	if p.telemetry != nil {
		ctx, span := p.telemetry.Tracer.Start(ctx, "payment.kafka.publish",
			trace.WithAttributes(
				attribute.String("kafka.topic", topic),
				attribute.String("kafka.client_id", p.clientName),
			),
		)
		defer span.End()
		observability.InjectKafkaHeaders(ctx, p.telemetry.Propagator, &rec.Headers)
		err := p.client.ProduceSync(ctx, rec).FirstErr()
		p.telemetry.Metrics.ObserveKafkaPublish(topic, err, time.Since(start))
		return err
	}

	return p.client.ProduceSync(ctx, rec).FirstErr()
}

func (p *Producer) Close() {
	p.client.Close()
}
