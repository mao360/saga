package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/inventory/internal/domain"
)

const (
	ReserveStatusReserved  = "reserved"
	ReserveStatusRejected  = "rejected"
	ReserveStatusProcessed = "processed"
)

type InventoryRepository struct {
	db *pgxpool.Pool
}

func NewInventoryRepository(db *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{db: db}
}

func (r *InventoryRepository) Reserve(ctx context.Context, cmd domain.Command) (string, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var alreadyProcessed bool
	if err := tx.QueryRow(ctx,
		"select exists(select 1 from processed_commands where command_id = $1)",
		cmd.CommandID,
	).Scan(&alreadyProcessed); err != nil {
		return "", err
	}
	if alreadyProcessed {
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return ReserveStatusProcessed, nil
	}

	var available int64
	err = tx.QueryRow(ctx,
		"select available_qty from inventory_stock where sku = $1 for update",
		cmd.SKU,
	).Scan(&available)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
				return "", err
			}
			if err := tx.Commit(ctx); err != nil {
				return "", err
			}
			return ReserveStatusRejected, nil
		}
		return "", err
	}

	if available < cmd.Qty {
		if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
			return "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return ReserveStatusRejected, nil
	}

	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `
		insert into inventory_reservation (id, saga_id, order_id, sku, qty, status, created_at, updated_at)
		values ($1, $2, $3, $4, $5, 'reserved', $6, $7)
		on conflict (saga_id, sku) do nothing
	`, cmd.CommandID, cmd.SagaID, cmd.OrderID, cmd.SKU, cmd.Qty, now, now)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
			return "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return ReserveStatusProcessed, nil
	}

	_, err = tx.Exec(ctx, `
		update inventory_stock
		set available_qty = available_qty - $1, updated_at = $2
		where sku = $3
	`, cmd.Qty, now, cmd.SKU)
	if err != nil {
		return "", err
	}

	if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return ReserveStatusReserved, nil
}

func (r *InventoryRepository) markProcessedTx(ctx context.Context, tx pgx.Tx, commandID string) error {
	_, err := tx.Exec(ctx, `
		insert into processed_commands (command_id, processed_at)
		values ($1, $2)
	`, commandID, time.Now().UTC())
	return err
}
