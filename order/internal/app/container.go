package app

import (
	"log/slog"

	"github.com/mao360/saga/order/internal/platform/config"
	"github.com/mao360/saga/order/internal/platform/kafka"
	"github.com/mao360/saga/order/internal/platform/logger"
)

type Container struct {
	Cfg config.Config
	Log *slog.Logger

	KafkaProducer *kafka.Producer
	KafkaConsumer *kafka.Consumer
}

func NewContainer() (*Container, error) {
	cfg := config.Load()
	log := logger.New()

	producer, err := kafka.NewProducer(cfg.KafkaBrokers, cfg.KafkaClientID)
	if err != nil {
		return nil, err
	}

	consumer, err := kafka.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaClientID,
		cfg.KafkaGroupID,
		[]string{cfg.TopicPaymentEvents, cfg.TopicInventoryEvents},
		log,
		producer,
		cfg.TopicDLQ,
	)
	if err != nil {
		producer.Close()
		return nil, err
	}

	return &Container{
		Cfg:           cfg,
		Log:           log,
		KafkaProducer: producer,
		KafkaConsumer: consumer,
	}, nil
}

func (c *Container) Close() {
	if c.KafkaConsumer != nil {
		_ = c.KafkaConsumer.Close()
	}
	if c.KafkaProducer != nil {
		c.KafkaProducer.Close()
	}
}
