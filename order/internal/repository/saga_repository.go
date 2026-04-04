package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/order/internal/domain"
)

type SagaRepository struct {
	db *pgxpool.Pool
}

func NewSagaRepository(db *pgxpool.Pool) *SagaRepository {
	return &SagaRepository{db: db}
}

func (r *SagaRepository) Create(ctx context.Context, sagaID, orderID string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		insert into saga_state (saga_id, order_id, inventory_status, payment_status, created_at, updated_at)
		values ($1, $2, 'pending', 'pending', $3, $4)
		on conflict (saga_id) do nothing
	`, sagaID, orderID, now, now)
	return err
}

// ApplyInventoryStatus атомарно обновляет inventory_status и возвращает
// актуальное состояние саги. SELECT FOR UPDATE сериализует два конкурирующих
// события (inventory + payment), чтобы решение о финализации принималось
// единожды на основе полного снимка.
func (r *SagaRepository) ApplyInventoryStatus(ctx context.Context, sagaID, status string) (domain.SagaState, error) {
	return r.apply(ctx, sagaID, func(tx pgx.Tx, state *domain.SagaState) error {
		state.InventoryStatus = status
		_, err := tx.Exec(ctx,
			`update saga_state set inventory_status = $1, updated_at = $2 where saga_id = $3`,
			status, time.Now().UTC(), sagaID,
		)
		return err
	})
}

func (r *SagaRepository) ApplyPaymentStatus(ctx context.Context, sagaID, status string) (domain.SagaState, error) {
	return r.apply(ctx, sagaID, func(tx pgx.Tx, state *domain.SagaState) error {
		state.PaymentStatus = status
		_, err := tx.Exec(ctx,
			`update saga_state set payment_status = $1, updated_at = $2 where saga_id = $3`,
			status, time.Now().UTC(), sagaID,
		)
		return err
	})
}

func (r *SagaRepository) apply(ctx context.Context, sagaID string, fn func(pgx.Tx, *domain.SagaState) error) (domain.SagaState, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.SagaState{}, err
	}
	defer tx.Rollback(ctx)

	var state domain.SagaState
	err = tx.QueryRow(ctx, `
		select saga_id, order_id, inventory_status, payment_status
		from saga_state where saga_id = $1 for update
	`, sagaID).Scan(&state.SagaID, &state.OrderID, &state.InventoryStatus, &state.PaymentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SagaState{}, domain.ErrSagaNotFound
		}
		return domain.SagaState{}, err
	}

	if err = fn(tx, &state); err != nil {
		return domain.SagaState{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.SagaState{}, err
	}
	return state, nil
}