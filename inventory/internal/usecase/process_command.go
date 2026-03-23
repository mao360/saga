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
	if cmd.Type != "reserve_inventory" {
		return nil
	}

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

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return u.publisher.Publish(ctx, u.topic, []byte(cmd.OrderID), payload)
}
