package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mao360/saga/inventory/internal/domain"
	"github.com/mao360/saga/inventory/internal/platform/postgres"
	"github.com/mao360/saga/inventory/internal/repository"
)

// InventoryRepository выполняет изменения состояния в рамках переданной
// транзакции q — это позволяет записать исходящее событие в outbox атомарно.
type InventoryRepository interface {
	Reserve(ctx context.Context, q postgres.DBTX, cmd domain.Command) (string, error)
	Release(ctx context.Context, q postgres.DBTX, cmd domain.Command) (string, error)
	SetStock(ctx context.Context, q postgres.DBTX, cmd domain.Command) (string, error)
}

// OutboxEnqueuer кладёт исходящее событие в таблицу outbox_messages в той же
// транзакции, что и бизнес-изменение; relay позже опубликует его в Kafka.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, q postgres.DBTX, msg domain.OutboxMessage) error
}

type TxRunner interface {
	Do(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type InventoryUseCase struct {
	repo   InventoryRepository
	outbox OutboxEnqueuer
	tx     TxRunner
	topic  string
}

func NewInventoryUseCase(repo InventoryRepository, outbox OutboxEnqueuer, tx TxRunner, topic string) *InventoryUseCase {
	return &InventoryUseCase{
		repo:   repo,
		outbox: outbox,
		tx:     tx,
		topic:  topic,
	}
}

func (u *InventoryUseCase) ProcessCommand(ctx context.Context, cmd domain.Command) error {
	if strings.TrimSpace(cmd.CommandID) == "" || cmd.Qty <= 0 || strings.TrimSpace(cmd.SKU) == "" {
		return domain.ErrInvalidCommand
	}

	return u.tx.Do(ctx, func(tx pgx.Tx) error {
		switch cmd.Type {
		case "reserve_inventory":
			status, err := u.repo.Reserve(ctx, tx, cmd)
			if err != nil {
				return err
			}
			if status == repository.ReserveStatusProcessed {
				return nil
			}
			event := newEvent(cmd)
			if status == repository.ReserveStatusReserved {
				event.Type = "inventory_reserved"
			} else {
				event.Type = "inventory_rejected"
				event.Reason = "out_of_stock"
			}
			return u.enqueue(ctx, tx, event)

		case "release_inventory":
			status, err := u.repo.Release(ctx, tx, cmd)
			if err != nil {
				return err
			}
			if status == repository.ReleaseStatusProcessed {
				return nil
			}
			event := newEvent(cmd)
			if status == repository.ReleaseStatusReleased {
				event.Type = "inventory_released"
			} else {
				event.Type = "inventory_release_rejected"
				event.Reason = "reservation_not_found"
			}
			return u.enqueue(ctx, tx, event)

		case "set_stock":
			status, err := u.repo.SetStock(ctx, tx, cmd)
			if err != nil {
				return err
			}
			if status == repository.StockStatusProcessed {
				return nil
			}
			event := newEvent(cmd)
			event.Type = "inventory_stock_set"
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
		SKU:        cmd.SKU,
		Qty:        cmd.Qty,
		OccurredAt: time.Now().UTC(),
	}
}

func (u *InventoryUseCase) enqueue(ctx context.Context, q postgres.DBTX, event domain.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	key := strings.TrimSpace(event.OrderID)
	if key == "" {
		key = event.SKU
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