package app

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/payment/internal/platform/config"
	"github.com/mao360/saga/payment/internal/platform/kafka"
	"github.com/mao360/saga/payment/internal/platform/logger"
	"github.com/mao360/saga/payment/internal/platform/postgres"
)

type Container struct {
	Cfg config.Config
	Log *slog.Logger

	KafkaProducer *kafka.Producer
	KafkaConsumer *kafka.Consumer
	Database      *pgxpool.Pool
}

func NewContainer() (*Container, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	log := logger.New()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseConnectTimeout)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseDSN)
	if err != nil {
		return nil, err
	}

	producer, err := kafka.NewProducer(cfg.KafkaBrokers, cfg.KafkaClientID)
	if err != nil {
		pool.Close()
		return nil, err
	}

	consumer, err := kafka.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaClientID,
		cfg.KafkaGroupID,
		[]string{cfg.TopicCommands},
		log,
		producer,
		cfg.TopicDLQ,
	)
	if err != nil {
		producer.Close()
		pool.Close()
		return nil, err
	}

	return &Container{
		Cfg:           *cfg,
		Log:           log,
		KafkaProducer: producer,
		KafkaConsumer: consumer,
		Database:      pool,
	}, nil
}

func (c *Container) Close() error {
	if c.KafkaConsumer != nil {
		_ = c.KafkaConsumer.Close()
	}
	if c.KafkaProducer != nil {
		c.KafkaProducer.Close()
	}
	if c.Database != nil {
		c.Database.Close()
	}
	return nil
}
