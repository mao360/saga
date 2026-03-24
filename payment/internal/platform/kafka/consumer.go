package kafka

import (
	"context"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Handler func(ctx context.Context, record *kgo.Record) error

type Consumer struct {
	client   *kgo.Client
	log      *slog.Logger
	producer *Producer
	dlqTopic string
}

func NewConsumer(
	brokers []string,
	clientID string,
	groupID string,
	topics []string,
	log *slog.Logger,
	producer *Producer,
	dlqTopic string,
) (*Consumer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID(clientID+"-consumer"),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		client:   cl,
		log:      log,
		producer: producer,
		dlqTopic: dlqTopic,
	}, nil
}

func (c *Consumer) Run(ctx context.Context, handler Handler) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		fetches := c.client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				c.log.Error("kafka fetch error", "err", fe.Err, "topic", fe.Topic, "partition", fe.Partition)
			}
			continue
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()

			if err := handler(ctx, rec); err != nil {
				c.log.Error("handler failed", "err", err, "topic", rec.Topic, "offset", rec.Offset)
				if c.producer != nil && c.dlqTopic != "" {
					if dlqErr := c.producer.Publish(ctx, c.dlqTopic, rec.Key, rec.Value); dlqErr != nil {
						c.log.Error("dlq publish failed", "err", dlqErr)
					}
				}
				if commitErr := c.client.CommitRecords(ctx, rec); commitErr != nil {
					c.log.Error("commit failed after handler error", "err", commitErr)
				}
				continue
			}

			if commitErr := c.client.CommitRecords(ctx, rec); commitErr != nil {
				c.log.Error("commit failed", "err", commitErr, "topic", rec.Topic, "offset", rec.Offset)
			}
		}
	}
}

func (c *Consumer) Close() error {
	if c.client == nil {
		return nil
	}
	c.client.Close()
	return nil
}
