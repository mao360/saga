package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/payment/internal/domain"
	"github.com/mao360/saga/payment/internal/platform/postgres"
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

// Charge списывает средства в рамках переданной транзакции q. Транзакцией
// владеет вызывающий (usecase), что позволяет записать событие в outbox
// атомарно со списанием.
func (r *PaymentRepository) Charge(ctx context.Context, q postgres.DBTX, cmd domain.Command) (string, error) {
	processed, err := r.isProcessed(ctx, q, cmd.CommandID)
	if err != nil {
		return "", err
	}
	if processed {
		return ChargeStatusProcessed, nil
	}

	var balance int64
	err = q.QueryRow(ctx,
		"select balance from payment_account_balance where account_id = $1 for update",
		cmd.AccountID,
	).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := r.markProcessed(ctx, q, cmd.CommandID); err != nil {
				return "", err
			}
			return ChargeStatusRejected, nil
		}
		return "", err
	}

	if balance < cmd.Amount {
		if err := r.markProcessed(ctx, q, cmd.CommandID); err != nil {
			return "", err
		}
		return ChargeStatusRejected, nil
	}

	now := time.Now().UTC()
	_, err = q.Exec(ctx, `
		update payment_account_balance
		set balance = balance - $1, updated_at = $2
		where account_id = $3
	`, cmd.Amount, now, cmd.AccountID)
	if err != nil {
		return "", err
	}

	_, err = q.Exec(ctx, `
		insert into payment_transaction (id, saga_id, order_id, account_id, amount, kind, status, created_at, updated_at)
		values ($1, $2, $3, $4, $5, 'charge', 'charged', $6, $7)
		on conflict (saga_id, order_id, kind) do nothing
	`, cmd.CommandID, cmd.SagaID, cmd.OrderID, cmd.AccountID, cmd.Amount, now, now)
	if err != nil {
		return "", err
	}

	if err := r.markProcessed(ctx, q, cmd.CommandID); err != nil {
		return "", err
	}
	return ChargeStatusCharged, nil
}

// Refund возвращает средства (компенсация) в рамках q.
func (r *PaymentRepository) Refund(ctx context.Context, q postgres.DBTX, cmd domain.Command) (string, error) {
	processed, err := r.isProcessed(ctx, q, cmd.CommandID)
	if err != nil {
		return "", err
	}
	if processed {
		return RefundStatusProcessed, nil
	}

	var charged bool
	err = q.QueryRow(ctx, `
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
		if err := r.markProcessed(ctx, q, cmd.CommandID); err != nil {
			return "", err
		}
		return RefundStatusRejected, nil
	}

	now := time.Now().UTC()
	_, err = q.Exec(ctx, `
		insert into payment_transaction (id, saga_id, order_id, account_id, amount, kind, status, created_at, updated_at)
		values ($1, $2, $3, $4, $5, 'refund', 'refunded', $6, $7)
		on conflict (saga_id, order_id, kind) do nothing
	`, cmd.CommandID, cmd.SagaID, cmd.OrderID, cmd.AccountID, cmd.Amount, now, now)
	if err != nil {
		return "", err
	}

	_, err = q.Exec(ctx, `
		insert into payment_account_balance (account_id, balance, updated_at)
		values ($1, $2, $3)
		on conflict (account_id) do update
		set balance = payment_account_balance.balance + excluded.balance,
		    updated_at = excluded.updated_at
	`, cmd.AccountID, cmd.Amount, now)
	if err != nil {
		return "", err
	}

	if err := r.markProcessed(ctx, q, cmd.CommandID); err != nil {
		return "", err
	}
	return RefundStatusRefunded, nil
}

// SetBalance задаёт баланс счёта (служебная команда) в рамках q.
func (r *PaymentRepository) SetBalance(ctx context.Context, q postgres.DBTX, cmd domain.Command) (string, error) {
	processed, err := r.isProcessed(ctx, q, cmd.CommandID)
	if err != nil {
		return "", err
	}
	if processed {
		return BalanceStatusProcessed, nil
	}

	now := time.Now().UTC()
	_, err = q.Exec(ctx, `
		insert into payment_account_balance (account_id, balance, updated_at)
		values ($1, $2, $3)
		on conflict (account_id) do update
		set balance = excluded.balance,
		    updated_at = excluded.updated_at
	`, cmd.AccountID, cmd.Amount, now)
	if err != nil {
		return "", err
	}

	if err := r.markProcessed(ctx, q, cmd.CommandID); err != nil {
		return "", err
	}
	return BalanceStatusUpdated, nil
}

func (r *PaymentRepository) isProcessed(ctx context.Context, q postgres.DBTX, commandID string) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx,
		"select exists(select 1 from processed_commands where command_id = $1)",
		commandID,
	).Scan(&exists)
	return exists, err
}

func (r *PaymentRepository) markProcessed(ctx context.Context, q postgres.DBTX, commandID string) error {
	_, err := q.Exec(ctx,
		"insert into processed_commands (command_id, processed_at) values ($1, $2)",
		commandID, time.Now().UTC(),
	)
	return err
}
