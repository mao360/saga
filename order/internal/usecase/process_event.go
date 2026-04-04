package usecase

import (
	"context"
	"encoding/json"

	"github.com/mao360/saga/order/internal/domain"
)

// SagaOrderRepository — часть OrderRepository, нужная для обработки событий.
type SagaOrderRepository interface {
	GetByID(ctx context.Context, id string) (domain.Order, error)
	UpdateStatus(ctx context.Context, orderID, status string) error
}

// SagaStateRepository — работа с таблицей saga_state.
type SagaStateRepository interface {
	ApplyInventoryStatus(ctx context.Context, sagaID, status string) (domain.SagaState, error)
	ApplyPaymentStatus(ctx context.Context, sagaID, status string) (domain.SagaState, error)
}

type ProcessEventUseCase struct {
	orderRepo     SagaOrderRepository
	sagaRepo      SagaStateRepository
	publisher     EventPublisher
	commandsTopic string
}

func NewProcessEventUseCase(
	orderRepo SagaOrderRepository,
	sagaRepo SagaStateRepository,
	publisher EventPublisher,
	commandsTopic string,
) *ProcessEventUseCase {
	return &ProcessEventUseCase{
		orderRepo:     orderRepo,
		sagaRepo:      sagaRepo,
		publisher:     publisher,
		commandsTopic: commandsTopic,
	}
}

// ProcessEvent реагирует на события от inventory- и payment-сервисов.
//
// Матрица решений:
//
//	inventory_reserved  + payment_charged  → order = completed
//	inventory_reserved  + payment_rejected → send release_inventory
//	inventory_rejected  + payment_charged  → send refund_payment
//	inventory_rejected  + payment_rejected → order = failed
//	inventory_released                     → order = failed (компенсация завершена)
//	payment_refunded                       → order = failed (компенсация завершена)
//	*_pending (другой шаг ещё не пришёл)  → ждём
func (u *ProcessEventUseCase) ProcessEvent(ctx context.Context, event domain.SagaEvent) error {
	switch event.Type {
	case "inventory_reserved":
		state, err := u.sagaRepo.ApplyInventoryStatus(ctx, event.SagaID, "reserved")
		if err != nil {
			return err
		}
		switch state.PaymentStatus {
		case "charged":
			return u.orderRepo.UpdateStatus(ctx, state.OrderID, domain.OrderStatusCompleted)
		case "rejected":
			return u.sendReleaseInventory(ctx, event, state)
		}

	case "inventory_rejected":
		state, err := u.sagaRepo.ApplyInventoryStatus(ctx, event.SagaID, "rejected")
		if err != nil {
			return err
		}
		switch state.PaymentStatus {
		case "charged":
			return u.sendRefundPayment(ctx, event, state)
		case "rejected":
			return u.orderRepo.UpdateStatus(ctx, state.OrderID, domain.OrderStatusFailed)
		}

	case "inventory_released":
		state, err := u.sagaRepo.ApplyInventoryStatus(ctx, event.SagaID, "released")
		if err != nil {
			return err
		}
		return u.orderRepo.UpdateStatus(ctx, state.OrderID, domain.OrderStatusFailed)

	case "payment_charged":
		state, err := u.sagaRepo.ApplyPaymentStatus(ctx, event.SagaID, "charged")
		if err != nil {
			return err
		}
		switch state.InventoryStatus {
		case "reserved":
			return u.orderRepo.UpdateStatus(ctx, state.OrderID, domain.OrderStatusCompleted)
		case "rejected":
			return u.sendRefundPayment(ctx, event, state)
		}

	case "payment_rejected":
		state, err := u.sagaRepo.ApplyPaymentStatus(ctx, event.SagaID, "rejected")
		if err != nil {
			return err
		}
		switch state.InventoryStatus {
		case "reserved":
			return u.sendReleaseInventory(ctx, event, state)
		case "rejected":
			return u.orderRepo.UpdateStatus(ctx, state.OrderID, domain.OrderStatusFailed)
		}

	case "payment_refunded":
		state, err := u.sagaRepo.ApplyPaymentStatus(ctx, event.SagaID, "refunded")
		if err != nil {
			return err
		}
		return u.orderRepo.UpdateStatus(ctx, state.OrderID, domain.OrderStatusFailed)
	}

	return nil
}

func (u *ProcessEventUseCase) sendReleaseInventory(ctx context.Context, event domain.SagaEvent, state domain.SagaState) error {
	order, err := u.orderRepo.GetByID(ctx, state.OrderID)
	if err != nil {
		return err
	}
	cmd := sagaCommand{
		CommandID: "cmd-release-" + order.ID,
		Type:      "release_inventory",
		SagaID:    event.SagaID,
		OrderID:   order.ID,
		SKU:       order.SKU,
		Qty:       order.Qty,
	}
	return u.publishCommand(ctx, order.ID, cmd)
}

func (u *ProcessEventUseCase) sendRefundPayment(ctx context.Context, event domain.SagaEvent, state domain.SagaState) error {
	order, err := u.orderRepo.GetByID(ctx, state.OrderID)
	if err != nil {
		return err
	}
	cmd := sagaCommand{
		CommandID: "cmd-refund-" + order.ID,
		Type:      "refund_payment",
		SagaID:    event.SagaID,
		OrderID:   order.ID,
		AccountID: order.AccountID,
		Amount:    order.Amount,
	}
	return u.publishCommand(ctx, order.ID, cmd)
}

func (u *ProcessEventUseCase) publishCommand(ctx context.Context, key string, cmd sagaCommand) error {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return u.publisher.Publish(ctx, u.commandsTopic, []byte(key), payload)
}