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
	UpdateStatus(ctx context.Context, q postgres.DBTX, orderID, status string) (time.Time, error)
}

// SagaMetrics — то, что use case сообщает наружу о завершившихся сагах.
// Может быть nil, если телеметрия выключена.
type SagaMetrics interface {
	ObserveSagaFinished(status string, d time.Duration)
}

// sagaOutcome фиксирует терминальный переход, случившийся внутри транзакции.
// Метрика пишется уже после коммита: иначе откат или ретрай транзакции дал бы
// наблюдение о саге, которая на самом деле не завершилась.
type sagaOutcome struct {
	finished  bool
	status    string
	createdAt time.Time
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
	metrics       SagaMetrics
}

func NewProcessEventUseCase(
	orderRepo SagaOrderRepository,
	sagaRepo SagaStateRepository,
	outbox OutboxEnqueuer,
	tx TxRunner,
	commandsTopic string,
	metrics SagaMetrics,
) *ProcessEventUseCase {
	return &ProcessEventUseCase{
		orderRepo:     orderRepo,
		sagaRepo:      sagaRepo,
		outbox:        outbox,
		tx:            tx,
		commandsTopic: commandsTopic,
		metrics:       metrics,
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
	var outcome sagaOutcome

	err := u.tx.Do(ctx, func(tx pgx.Tx) error {
		// Сброс на случай, если TxRunner повторит замыкание после отката.
		outcome = sagaOutcome{}

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
				return u.finishOrder(ctx, tx, state.OrderID, domain.OrderStatusCompleted, &outcome)
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
				return u.finishOrder(ctx, tx, state.OrderID, domain.OrderStatusFailed, &outcome)
			}

		case "inventory_released":
			if err := u.sagaRepo.UpdateInventoryStatus(ctx, tx, event.SagaID, "released"); err != nil {
				return err
			}
			return u.finishOrder(ctx, tx, state.OrderID, domain.OrderStatusFailed, &outcome)

		case "payment_charged":
			if err := u.sagaRepo.UpdatePaymentStatus(ctx, tx, event.SagaID, "charged"); err != nil {
				return err
			}
			state.PaymentStatus = "charged"
			switch state.InventoryStatus {
			case "reserved":
				return u.finishOrder(ctx, tx, state.OrderID, domain.OrderStatusCompleted, &outcome)
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
				return u.finishOrder(ctx, tx, state.OrderID, domain.OrderStatusFailed, &outcome)
			}

		case "payment_refunded":
			if err := u.sagaRepo.UpdatePaymentStatus(ctx, tx, event.SagaID, "refunded"); err != nil {
				return err
			}
			return u.finishOrder(ctx, tx, state.OrderID, domain.OrderStatusFailed, &outcome)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if outcome.finished && u.metrics != nil {
		u.metrics.ObserveSagaFinished(outcome.status, time.Since(outcome.createdAt))
	}
	return nil
}

// finishOrder переводит заказ в терминальный статус и запоминает исход в out,
// чтобы вызывающий код записал длительность саги после коммита.
func (u *ProcessEventUseCase) finishOrder(ctx context.Context, tx pgx.Tx, orderID, status string, out *sagaOutcome) error {
	createdAt, err := u.orderRepo.UpdateStatus(ctx, tx, orderID, status)
	if err != nil {
		return err
	}
	*out = sagaOutcome{finished: true, status: status, createdAt: createdAt}
	return nil
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
