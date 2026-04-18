package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/platform/postgres"
)

type SagaRepository struct {
	db *pgxpool.Pool
}

func NewSagaRepository(db *pgxpool.Pool) *SagaRepository {
	return &SagaRepository{db: db}
}

func (r *SagaRepository) Create(ctx context.Context, q postgres.DBTX, sagaID, orderID string) error {
	now := time.Now().UTC()
	_, err := q.Exec(ctx, `
		insert into saga_state (saga_id, order_id, inventory_status, payment_status, created_at, updated_at)
		values ($1, $2, 'pending', 'pending', $3, $4)
		on conflict (saga_id) do nothing
	`, sagaID, orderID, now, now)
	return err
}

// LockState блокирует строку саги SELECT FOR UPDATE и возвращает её состояние.
// Требует транзакции — сериализует конкурирующие события, чтобы решение о
// следующем шаге принималось на актуальном снимке.
func (r *SagaRepository) LockState(ctx context.Context, tx pgx.Tx, sagaID string) (domain.SagaState, error) {
	var state domain.SagaState
	err := tx.QueryRow(ctx, `
		select saga_id, order_id, inventory_status, payment_status
		from saga_state where saga_id = $1 for update
	`, sagaID).Scan(&state.SagaID, &state.OrderID, &state.InventoryStatus, &state.PaymentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SagaState{}, domain.ErrSagaNotFound
		}
		return domain.SagaState{}, err
	}
	return state, nil
}

func (r *SagaRepository) UpdateInventoryStatus(ctx context.Context, q postgres.DBTX, sagaID, status string) error {
	_, err := q.Exec(ctx,
		`update saga_state set inventory_status = $1, updated_at = $2 where saga_id = $3`,
		status, time.Now().UTC(), sagaID,
	)
	return err
}

func (r *SagaRepository) UpdatePaymentStatus(ctx context.Context, q postgres.DBTX, sagaID, status string) error {
	_, err := q.Exec(ctx,
		`update saga_state set payment_status = $1, updated_at = $2 where saga_id = $3`,
		status, time.Now().UTC(), sagaID,
	)
	return err
}