package outbox

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// Deleter — контракт для удаления успешно отправленных старых сообщений.
type Deleter interface {
	DeleteSentOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type CleanerConfig struct {
	Interval  time.Duration // как часто запускать GC
	Retention time.Duration // насколько долго держим sent-сообщения
}

func (c CleanerConfig) normalize() CleanerConfig {
	if c.Interval <= 0 {
		c.Interval = 10 * time.Minute
	}
	if c.Retention <= 0 {
		c.Retention = 7 * 24 * time.Hour
	}
	return c
}

type Cleaner struct {
	deleter Deleter
	log     *slog.Logger
	cfg     CleanerConfig
}

func NewCleaner(deleter Deleter, log *slog.Logger, cfg CleanerConfig) *Cleaner {
	return &Cleaner{deleter: deleter, log: log, cfg: cfg.normalize()}
}

func (c *Cleaner) Run(ctx context.Context) error {
	c.log.Info("outbox cleaner starting",
		"interval", c.cfg.Interval,
		"retention", c.cfg.Retention,
	)

	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.log.Info("outbox cleaner stopped")
			return nil
		case <-ticker.C:
			cutoff := time.Now().UTC().Add(-c.cfg.Retention)
			deleted, err := c.deleter.DeleteSentOlderThan(ctx, cutoff)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				c.log.Error("outbox cleaner failed", "err", err)
				continue
			}
			if deleted > 0 {
				c.log.Info("outbox cleaner deleted", "rows", deleted)
			}
		}
	}
}