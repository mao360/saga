package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/payment/internal/domain"
)

const (
	ChargeStatusCharged   = "charged"
	ChargeStatusRejected  = "rejected"
	ChargeStatusProcessed = "processed"

	RefundStatusRefunded  = "refunded"
	RefundStatusRejected  = "rejected"
	RefundStatusProcessed = "processed"

	BalanceStatusUpdated   = "updated"
	BalanceStatusProcessed = "processed"
)

type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Charge(ctx context.Context, cmd domain.Command) (string, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	processed, err := r.isProcessedTx(ctx, tx, cmd.CommandID)
	if err != nil {
		return "", err
	}
	if processed {
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return ChargeStatusProcessed, nil
	}

	var balance int64
	err = tx.QueryRow(ctx,
		"select balance from payment_account_balance where account_id = $1 for update",
		cmd.AccountID,
	).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
				return "", err
			}
			if err := tx.Commit(ctx); err != nil {
				return "", err
			}
			return ChargeStatusRejected, nil
		}
		return "", err
	}

	if balance < cmd.Amount {
		if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
			return "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return ChargeStatusRejected, nil
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		update payment_account_balance
		set balance = balance - $1, updated_at = $2
		where account_id = $3
	`, cmd.Amount, now, cmd.AccountID)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(ctx, `
		insert into payment_transaction (id, saga_id, order_id, account_id, amount, kind, status, created_at, updated_at)
		values ($1, $2, $3, $4, $5, 'charge', 'charged', $6, $7)
		on conflict (saga_id, order_id, kind) do nothing
	`, cmd.CommandID, cmd.SagaID, cmd.OrderID, cmd.AccountID, cmd.Amount, now, now)
	if err != nil {
		return "", err
	}

	if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return ChargeStatusCharged, nil
}

func (r *PaymentRepository) Refund(ctx context.Context, cmd domain.Command) (string, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	processed, err := r.isProcessedTx(ctx, tx, cmd.CommandID)
	if err != nil {
		return "", err
	}
	if processed {
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return RefundStatusProcessed, nil
	}

	var charged bool
	err = tx.QueryRow(ctx, `
		select exists(
			select 1
			from payment_transaction
			where saga_id = $1 and order_id = $2 and kind = 'charge' and status = 'charged'
		)`,
		cmd.SagaID, cmd.OrderID,
	).Scan(&charged)
	if err != nil {
		return "", err
	}
	if !charged {
		if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
			return "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return RefundStatusRejected, nil
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		insert into payment_transaction (id, saga_id, order_id, account_id, amount, kind, status, created_at, updated_at)
		values ($1, $2, $3, $4, $5, 'refund', 'refunded', $6, $7)
		on conflict (saga_id, order_id, kind) do nothing
	`, cmd.CommandID, cmd.SagaID, cmd.OrderID, cmd.AccountID, cmd.Amount, now, now)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(ctx, `
		insert into payment_account_balance (account_id, balance, updated_at)
		values ($1, $2, $3)
		on conflict (account_id) do update
		set balance = payment_account_balance.balance + excluded.balance,
		    updated_at = excluded.updated_at
	`, cmd.AccountID, cmd.Amount, now)
	if err != nil {
		return "", err
	}

	if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return RefundStatusRefunded, nil
}

func (r *PaymentRepository) SetBalance(ctx context.Context, cmd domain.Command) (string, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	processed, err := r.isProcessedTx(ctx, tx, cmd.CommandID)
	if err != nil {
		return "", err
	}
	if processed {
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return BalanceStatusProcessed, nil
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		insert into payment_account_balance (account_id, balance, updated_at)
		values ($1, $2, $3)
		on conflict (account_id) do update
		set balance = excluded.balance,
		    updated_at = excluded.updated_at
	`, cmd.AccountID, cmd.Amount, now)
	if err != nil {
		return "", err
	}

	if err := r.markProcessedTx(ctx, tx, cmd.CommandID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return BalanceStatusUpdated, nil
}

func (r *PaymentRepository) isProcessedTx(ctx context.Context, tx pgx.Tx, commandID string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx,
		"select exists(select 1 from processed_commands where command_id = $1)",
		commandID,
	).Scan(&exists)
	return exists, err
}

func (r *PaymentRepository) markProcessedTx(ctx context.Context, tx pgx.Tx, commandID string) error {
	_, err := tx.Exec(ctx,
		"insert into processed_commands (command_id, processed_at) values ($1, $2)",
		commandID, time.Now().UTC(),
	)
	return err
}
