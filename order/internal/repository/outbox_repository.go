package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/platform/postgres"
)

type OutboxRepository struct {
	db *pgxpool.Pool
}

func NewOutboxRepository(db *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// Enqueue вставляет сообщение. q может быть *pgxpool.Pool или pgx.Tx —
// для использования в той же транзакции, что и бизнес-изменения.
func (r *OutboxRepository) Enqueue(ctx context.Context, q postgres.DBTX, msg domain.OutboxMessage) error {
	const query = `
		insert into outbox_messages (id, topic, key, payload, headers, created_at)
		values ($1, $2, $3, $4, $5, $6)`
	_, err := q.Exec(ctx, query,
		msg.ID, msg.Topic, msg.Key, msg.Payload, msg.Headers, msg.CreatedAt,
	)
	return err
}

// FetchUnsent выбирает до limit неотправленных сообщений и блокирует их
// (SKIP LOCKED), чтобы несколько relay-воркеров не дрались за одни и те же
// строки. Должен вызываться внутри транзакции.
func (r *OutboxRepository) FetchUnsent(ctx context.Context, q postgres.DBTX, limit int) ([]domain.OutboxMessage, error) {
	const query = `
		select id, topic, key, payload, headers, created_at, attempts
		from outbox_messages
		where sent_at is null
		order by created_at
		limit $1
		for update skip locked`

	rows, err := q.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.OutboxMessage
	for rows.Next() {
		var m domain.OutboxMessage
		if err := rows.Scan(&m.ID, &m.Topic, &m.Key, &m.Payload, &m.Headers, &m.CreatedAt, &m.Attempts); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *OutboxRepository) MarkSent(ctx context.Context, q postgres.DBTX, id uuid.UUID) error {
	const query = `
		update outbox_messages
		set sent_at = $1
		where id = $2`
	_, err := q.Exec(ctx, query, time.Now().UTC(), id)
	return err
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, q postgres.DBTX, id uuid.UUID, cause string) error {
	const query = `
		update outbox_messages
		set attempts = attempts + 1, last_error = $1
		where id = $2`
	_, err := q.Exec(ctx, query, cause, id)
	return err
}

// DeleteSentOlderThan удаляет успешно отправленные сообщения старше cutoff.
// Используется фоновым cleaner-ом.
func (r *OutboxRepository) DeleteSentOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	const query = `
		delete from outbox_messages
		where sent_at is not null and sent_at < $1`
	tag, err := r.db.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Pool возвращает внутренний пул — нужен relay-воркеру для BeginTx.
func (r *OutboxRepository) Pool() *pgxpool.Pool {
	return r.db
}