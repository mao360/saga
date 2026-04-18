package outbox

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/platform/postgres"
)

// Publisher — минимальный контракт Kafka-продьюсера, который нужен relay-у.
type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

// Reader / Marker — контракты OutboxRepository, нужные relay-воркеру.
type Reader interface {
	FetchUnsent(ctx context.Context, q postgres.DBTX, limit int) ([]domain.OutboxMessage, error)
}

type Marker interface {
	MarkSent(ctx context.Context, q postgres.DBTX, id uuid.UUID) error
	MarkFailed(ctx context.Context, q postgres.DBTX, id uuid.UUID, cause string) error
}

type Config struct {
	PollInterval time.Duration
	BatchSize    int
}

func (c Config) normalize() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = 200 * time.Millisecond
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 50
	}
	return c
}

// Relay периодически вычитывает неотправленные сообщения и публикует их в Kafka.
// FOR UPDATE SKIP LOCKED позволяет безопасно запускать несколько реплик order-сервиса.
type Relay struct {
	pool      *pgxpool.Pool
	reader    Reader
	marker    Marker
	publisher Publisher
	log       *slog.Logger
	cfg       Config
}

func New(pool *pgxpool.Pool, reader Reader, marker Marker, publisher Publisher, log *slog.Logger, cfg Config) *Relay {
	return &Relay{
		pool:      pool,
		reader:    reader,
		marker:    marker,
		publisher: publisher,
		log:       log,
		cfg:       cfg.normalize(),
	}
}

func (r *Relay) Run(ctx context.Context) error {
	r.log.Info("outbox relay starting",
		"poll_interval", r.cfg.PollInterval,
		"batch_size", r.cfg.BatchSize,
	)

	ticker := time.NewTicker(r.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info("outbox relay stopped")
			return nil
		case <-ticker.C:
			n, err := r.tick(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				r.log.Error("outbox relay tick failed", "err", err)
				continue
			}
			// Если в этом тике был полный батч — не ждём следующего тика,
			// сразу обрабатываем ещё один, чтобы быстрее разгрести бэклог.
			if n == r.cfg.BatchSize {
				if _, err := r.tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
					r.log.Error("outbox relay drain tick failed", "err", err)
				}
			}
		}
	}
}

// tick возвращает количество обработанных (не обязательно успешно отправленных)
// сообщений — вызывающий может решить надо ли сразу крутить ещё один.
func (r *Relay) tick(ctx context.Context) (int, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	msgs, err := r.reader.FetchUnsent(ctx, tx, r.cfg.BatchSize)
	if err != nil {
		return 0, err
	}
	if len(msgs) == 0 {
		return 0, nil
	}

	for _, m := range msgs {
		if err := r.publisher.Publish(ctx, m.Topic, []byte(m.Key), m.Payload); err != nil {
			r.log.Warn("outbox publish failed",
				"id", m.ID, "topic", m.Topic, "attempts", m.Attempts, "err", err,
			)
			if mErr := r.marker.MarkFailed(ctx, tx, m.ID, err.Error()); mErr != nil {
				return len(msgs), mErr
			}
			continue
		}
		if err := r.marker.MarkSent(ctx, tx, m.ID); err != nil {
			return len(msgs), err
		}
	}

	return len(msgs), tx.Commit(ctx)
}