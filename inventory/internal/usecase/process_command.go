package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mao360/saga/inventory/internal/domain"
	"github.com/mao360/saga/inventory/internal/repository"
)

type InventoryRepository interface {
	Reserve(ctx context.Context, cmd domain.Command) (string, error)
	Release(ctx context.Context, cmd domain.Command) (string, error)
	SetStock(ctx context.Context, cmd domain.Command) (string, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type InventoryUseCase struct {
	repo      InventoryRepository
	publisher EventPublisher
	topic     string
}

func NewInventoryUseCase(repo InventoryRepository, publisher EventPublisher, topic string) *InventoryUseCase {
	return &InventoryUseCase{
		repo:      repo,
		publisher: publisher,
		topic:     topic,
	}
}

func (u *InventoryUseCase) ProcessCommand(ctx context.Context, cmd domain.Command) error {
	if strings.TrimSpace(cmd.CommandID) == "" || cmd.Qty <= 0 || strings.TrimSpace(cmd.SKU) == "" {
		return domain.ErrInvalidCommand
	}
	switch cmd.Type {
	case "reserve_inventory":
		status, err := u.repo.Reserve(ctx, cmd)
		if err != nil {
			return err
		}
		if status == repository.ReserveStatusProcessed {
			return nil
		}
		event := domain.Event{
			CommandID:  cmd.CommandID,
			SagaID:     cmd.SagaID,
			OrderID:    cmd.OrderID,
			SKU:        cmd.SKU,
			Qty:        cmd.Qty,
			OccurredAt: time.Now().UTC(),
		}
		if status == repository.ReserveStatusReserved {
			event.Type = "inventory_reserved"
		} else {
			event.Type = "inventory_rejected"
			event.Reason = "out_of_stock"
		}
		return u.publishEvent(ctx, event)
	case "release_inventory":
		status, err := u.repo.Release(ctx, cmd)
		if err != nil {
			return err
		}
		if status == repository.ReleaseStatusProcessed {
			return nil
		}
		event := domain.Event{
			CommandID:  cmd.CommandID,
			SagaID:     cmd.SagaID,
			OrderID:    cmd.OrderID,
			SKU:        cmd.SKU,
			Qty:        cmd.Qty,
			OccurredAt: time.Now().UTC(),
		}
		if status == repository.ReleaseStatusReleased {
			event.Type = "inventory_released"
		} else {
			event.Type = "inventory_release_rejected"
			event.Reason = "reservation_not_found"
		}
		return u.publishEvent(ctx, event)
	case "set_stock":
		status, err := u.repo.SetStock(ctx, cmd)
		if err != nil {
			return err
		}
		if status == repository.StockStatusProcessed {
			return nil
		}
		event := domain.Event{
			Type:       "inventory_stock_set",
			CommandID:  cmd.CommandID,
			SagaID:     cmd.SagaID,
			OrderID:    cmd.OrderID,
			SKU:        cmd.SKU,
			Qty:        cmd.Qty,
			OccurredAt: time.Now().UTC(),
		}
		return u.publishEvent(ctx, event)
	default:
		return nil
	}
}

func (u *InventoryUseCase) publishEvent(ctx context.Context, event domain.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	key := event.OrderID
	if strings.TrimSpace(key) == "" {
		key = event.SKU
	}
	return u.publisher.Publish(ctx, u.topic, []byte(key), payload)
}
