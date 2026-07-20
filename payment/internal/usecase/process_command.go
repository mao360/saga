package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mao360/saga/payment/internal/domain"
	"github.com/mao360/saga/payment/internal/platform/postgres"
	"github.com/mao360/saga/payment/internal/repository"
)

// PaymentRepository выполняет изменения состояния в рамках переданной
// транзакции q — это позволяет записать исходящее событие в outbox атомарно.
type PaymentRepository interface {
	Charge(ctx context.Context, q postgres.DBTX, cmd domain.Command) (string, error)
	Refund(ctx context.Context, q postgres.DBTX, cmd domain.Command) (string, error)
	SetBalance(ctx context.Context, q postgres.DBTX, cmd domain.Command) (string, error)
}

// OutboxEnqueuer кладёт исходящее событие в таблицу outbox_messages в той же
// транзакции, что и бизнес-изменение; relay позже опубликует его в Kafka.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, q postgres.DBTX, msg domain.OutboxMessage) error
}

type TxRunner interface {
	Do(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type PaymentUseCase struct {
	repo   PaymentRepository
	outbox OutboxEnqueuer
	tx     TxRunner
	topic  string
}

func NewPaymentUseCase(repo PaymentRepository, outbox OutboxEnqueuer, tx TxRunner, topic string) *PaymentUseCase {
	return &PaymentUseCase{
		repo:   repo,
		outbox: outbox,
		tx:     tx,
		topic:  topic,
	}
}

func (u *PaymentUseCase) ProcessCommand(ctx context.Context, cmd domain.Command) error {
	if strings.TrimSpace(cmd.CommandID) == "" || strings.TrimSpace(cmd.AccountID) == "" || cmd.Amount <= 0 {
		return domain.ErrInvalidCommand
	}

	return u.tx.Do(ctx, func(tx pgx.Tx) error {
		switch cmd.Type {
		case "charge_payment":
			status, err := u.repo.Charge(ctx, tx, cmd)
			if err != nil {
				return err
			}
			if status == repository.ChargeStatusProcessed {
				return nil
			}
			event := newEvent(cmd)
			if status == repository.ChargeStatusCharged {
				event.Type = "payment_charged"
			} else {
				event.Type = "payment_rejected"
				event.Reason = "insufficient_funds"
			}
			return u.enqueue(ctx, tx, event)

		case "refund_payment":
			status, err := u.repo.Refund(ctx, tx, cmd)
			if err != nil {
				return err
			}
			if status == repository.RefundStatusProcessed {
				return nil
			}
			event := newEvent(cmd)
			if status == repository.RefundStatusRefunded {
				event.Type = "payment_refunded"
			} else {
				event.Type = "payment_refund_rejected"
				event.Reason = "charge_not_found"
			}
			return u.enqueue(ctx, tx, event)

		case "set_balance":
			status, err := u.repo.SetBalance(ctx, tx, cmd)
			if err != nil {
				return err
			}
			if status == repository.BalanceStatusProcessed {
				return nil
			}
			event := newEvent(cmd)
			event.Type = "payment_balance_set"
			return u.enqueue(ctx, tx, event)

		default:
			return nil
		}
	})
}

func newEvent(cmd domain.Command) domain.Event {
	return domain.Event{
		CommandID:  cmd.CommandID,
		SagaID:     cmd.SagaID,
		OrderID:    cmd.OrderID,
		AccountID:  cmd.AccountID,
		Amount:     cmd.Amount,
		OccurredAt: time.Now().UTC(),
	}
}

func (u *PaymentUseCase) enqueue(ctx context.Context, q postgres.DBTX, event domain.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	key := strings.TrimSpace(event.OrderID)
	if key == "" {
		key = event.AccountID
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	return u.outbox.Enqueue(ctx, q, domain.OutboxMessage{
		ID:        id,
		Topic:     u.topic,
		Key:       key,
		Payload:   payload,
		Headers:   []byte("{}"),
		CreatedAt: time.Now().UTC(),
	})
}
