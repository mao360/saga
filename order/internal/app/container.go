package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/order/internal/observability"
	"github.com/mao360/saga/order/internal/platform/config"
	"github.com/mao360/saga/order/internal/platform/kafka"
	"github.com/mao360/saga/order/internal/platform/logger"
	"github.com/mao360/saga/order/internal/platform/postgres"
)

type Container struct {
	Cfg config.Config
	Log *slog.Logger
	Tel *observability.Telemetry

	KafkaProducer *kafka.Producer
	KafkaConsumer *kafka.Consumer

	Database *pgxpool.Pool
}

func NewContainer() (*Container, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	log := logger.New()
	tel, err := observability.New("order-service", log)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseConnectTimeout)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseDSN)
	if err != nil {
		return nil, err
	}

	producer, err := kafka.NewProducer(cfg.KafkaBrokers, cfg.KafkaClientID, tel)
	if err != nil {
		_ = tel.Shutdown(context.Background())
		pool.Close()
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
		_ = tel.Shutdown(context.Background())
		pool.Close()
		return nil, err
	}

	return &Container{
		Cfg:           cfg,
		Log:           log,
		Tel:           tel,
		KafkaProducer: producer,
		KafkaConsumer: consumer,
		Database:      pool,
	}, nil
}

func (c *Container) Close() {
	if c.KafkaConsumer != nil {
		c.KafkaConsumer.Close()
	}
	if c.KafkaProducer != nil {
		c.KafkaProducer.Close()
	}

	if c.Database != nil {
		c.Database.Close()
	}
	if c.Tel != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Tel.Shutdown(ctx)
	}
}
