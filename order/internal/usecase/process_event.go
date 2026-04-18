package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/platform/postgres"
)

// SagaOrderRepository — часть OrderRepository, нужная для обработки событий.
type SagaOrderRepository interface {
	GetByIDTx(ctx context.Context, q postgres.DBTX, id string) (domain.Order, error)
	UpdateStatus(ctx context.Context, q postgres.DBTX, orderID, status string) error
}

// SagaStateRepository — работа с таблицей saga_state.
type SagaStateRepository interface {
	LockState(ctx context.Context, tx pgx.Tx, sagaID string) (domain.SagaState, error)
	UpdateInventoryStatus(ctx context.Context, q postgres.DBTX, sagaID, status string) error
	UpdatePaymentStatus(ctx context.Context, q postgres.DBTX, sagaID, status string) error
}

type ProcessEventUseCase struct {
	orderRepo     SagaOrderRepository
	sagaRepo      SagaStateRepository
	outbox        OutboxEnqueuer
	tx            TxRunner
	commandsTopic string
}

func NewProcessEventUseCase(
	orderRepo SagaOrderRepository,
	sagaRepo SagaStateRepository,
	outbox OutboxEnqueuer,
	tx TxRunner,
	commandsTopic string,
) *ProcessEventUseCase {
	return &ProcessEventUseCase{
		orderRepo:     orderRepo,
		sagaRepo:      sagaRepo,
		outbox:        outbox,
		tx:            tx,
		commandsTopic: commandsTopic,
	}
}

// ProcessEvent реагирует на события от inventory- и payment-сервисов.
// Вся обработка идёт в одной транзакции: SELECT FOR UPDATE на saga_state
// + UPDATE статуса + возможный UPDATE заказа / Enqueue компенсирующей команды
// в outbox. Это устраняет dual-write и делает шаг саги атомарным.
//
// Матрица решений:
//
//	inventory_reserved  + payment_charged  → order = completed
//	inventory_reserved  + payment_rejected → enqueue release_inventory
//	inventory_rejected  + payment_charged  → enqueue refund_payment
//	inventory_rejected  + payment_rejected → order = failed
//	inventory_released                     → order = failed (компенсация завершена)
//	payment_refunded                       → order = failed (компенсация завершена)
//	*_pending (другой шаг ещё не пришёл)  → ждём
func (u *ProcessEventUseCase) ProcessEvent(ctx context.Context, event domain.SagaEvent) error {
	return u.tx.Do(ctx, func(tx pgx.Tx) error {
		state, err := u.sagaRepo.LockState(ctx, tx, event.SagaID)
		if err != nil {
			return err
		}

		switch event.Type {
		case "inventory_reserved":
			if err := u.sagaRepo.UpdateInventoryStatus(ctx, tx, event.SagaID, "reserved"); err != nil {
				return err
			}
			state.InventoryStatus = "reserved"
			switch state.PaymentStatus {
			case "charged":
				return u.orderRepo.UpdateStatus(ctx, tx, state.OrderID, domain.OrderStatusCompleted)
			case "rejected":
				return u.enqueueReleaseInventory(ctx, tx, event, state)
			}

		case "inventory_rejected":
			if err := u.sagaRepo.UpdateInventoryStatus(ctx, tx, event.SagaID, "rejected"); err != nil {
				return err
			}
			state.InventoryStatus = "rejected"
			switch state.PaymentStatus {
			case "charged":
				return u.enqueueRefundPayment(ctx, tx, event, state)
			case "rejected":
				return u.orderRepo.UpdateStatus(ctx, tx, state.OrderID, domain.OrderStatusFailed)
			}

		case "inventory_released":
			if err := u.sagaRepo.UpdateInventoryStatus(ctx, tx, event.SagaID, "released"); err != nil {
				return err
			}
			return u.orderRepo.UpdateStatus(ctx, tx, state.OrderID, domain.OrderStatusFailed)

		case "payment_charged":
			if err := u.sagaRepo.UpdatePaymentStatus(ctx, tx, event.SagaID, "charged"); err != nil {
				return err
			}
			state.PaymentStatus = "charged"
			switch state.InventoryStatus {
			case "reserved":
				return u.orderRepo.UpdateStatus(ctx, tx, state.OrderID, domain.OrderStatusCompleted)
			case "rejected":
				return u.enqueueRefundPayment(ctx, tx, event, state)
			}

		case "payment_rejected":
			if err := u.sagaRepo.UpdatePaymentStatus(ctx, tx, event.SagaID, "rejected"); err != nil {
				return err
			}
			state.PaymentStatus = "rejected"
			switch state.InventoryStatus {
			case "reserved":
				return u.enqueueReleaseInventory(ctx, tx, event, state)
			case "rejected":
				return u.orderRepo.UpdateStatus(ctx, tx, state.OrderID, domain.OrderStatusFailed)
			}

		case "payment_refunded":
			if err := u.sagaRepo.UpdatePaymentStatus(ctx, tx, event.SagaID, "refunded"); err != nil {
				return err
			}
			return u.orderRepo.UpdateStatus(ctx, tx, state.OrderID, domain.OrderStatusFailed)
		}

		return nil
	})
}

func (u *ProcessEventUseCase) enqueueReleaseInventory(ctx context.Context, tx pgx.Tx, event domain.SagaEvent, state domain.SagaState) error {
	order, err := u.orderRepo.GetByIDTx(ctx, tx, state.OrderID)
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
	return u.enqueueCommand(ctx, tx, order.ID, cmd)
}

func (u *ProcessEventUseCase) enqueueRefundPayment(ctx context.Context, tx pgx.Tx, event domain.SagaEvent, state domain.SagaState) error {
	order, err := u.orderRepo.GetByIDTx(ctx, tx, state.OrderID)
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
	return u.enqueueCommand(ctx, tx, order.ID, cmd)
}

func (u *ProcessEventUseCase) enqueueCommand(ctx context.Context, q postgres.DBTX, key string, cmd sagaCommand) error {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	return u.outbox.Enqueue(ctx, q, domain.OutboxMessage{
		ID:        id,
		Topic:     u.commandsTopic,
		Key:       key,
		Payload:   payload,
		Headers:   []byte("{}"),
		CreatedAt: time.Now().UTC(),
	})
}