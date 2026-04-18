package usecase

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/platform/postgres"
)

type OrderRepository interface {
	Save(ctx context.Context, q postgres.DBTX, order domain.Order) error
	GetByID(ctx context.Context, id string) (domain.Order, error)
}

type SagaCreator interface {
	Create(ctx context.Context, q postgres.DBTX, sagaID, orderID string) error
}

// OutboxEnqueuer кладёт сообщение в таблицу outbox_messages. Вызов выполняется
// в той же транзакции, что и бизнес-изменения, — relay позже прочитает строку
// и опубликует её в Kafka.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, q postgres.DBTX, msg domain.OutboxMessage) error
}

type TxRunner interface {
	Do(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type OrderUseCase struct {
	repo          OrderRepository
	saga          SagaCreator
	outbox        OutboxEnqueuer
	tx            TxRunner
	orderTopic    string
	commandsTopic string
}

type CreateOrderInput struct {
	Customer  string `json:"customer"`
	Amount    int64  `json:"amount"`
	SKU       string `json:"sku"`
	Qty       int64  `json:"qty"`
	AccountID string `json:"account_id"`
}

type sagaCommand struct {
	CommandID string `json:"command_id"`
	Type      string `json:"type"`
	SagaID    string `json:"saga_id"`
	OrderID   string `json:"order_id"`
	SKU       string `json:"sku,omitempty"`
	Qty       int64  `json:"qty,omitempty"`
	AccountID string `json:"account_id,omitempty"`
	Amount    int64  `json:"amount,omitempty"`
}

func NewOrderUseCase(
	repo OrderRepository,
	saga SagaCreator,
	outbox OutboxEnqueuer,
	tx TxRunner,
	orderTopic, commandsTopic string,
) *OrderUseCase {
	return &OrderUseCase{
		repo:          repo,
		saga:          saga,
		outbox:        outbox,
		tx:            tx,
		orderTopic:    orderTopic,
		commandsTopic: commandsTopic,
	}
}

func (u *OrderUseCase) CreateOrder(ctx context.Context, input CreateOrderInput) (domain.Order, error) {
	now := time.Now().UTC()
	order := domain.Order{
		ID:        strconv.FormatInt(now.UnixNano(), 10),
		Customer:  strings.TrimSpace(input.Customer),
		Amount:    input.Amount,
		SKU:       strings.TrimSpace(input.SKU),
		Qty:       input.Qty,
		AccountID: strings.TrimSpace(input.AccountID),
		Status:    domain.OrderStatusPending,
		CreatedAt: now,
	}

	if order.Amount <= 0 || order.Customer == "" || order.SKU == "" || order.Qty <= 0 || order.AccountID == "" {
		return domain.Order{}, domain.ErrInvalidOrder
	}

	sagaID := "saga-" + order.ID

	err := u.tx.Do(ctx, func(tx pgx.Tx) error {
		if err := u.repo.Save(ctx, tx, order); err != nil {
			return err
		}
		if err := u.saga.Create(ctx, tx, sagaID, order.ID); err != nil {
			return err
		}

		orderPayload, err := json.Marshal(order)
		if err != nil {
			return err
		}
		if err := u.enqueue(ctx, tx, u.orderTopic, order.ID, orderPayload); err != nil {
			return err
		}

		reserve := sagaCommand{
			CommandID: "cmd-reserve-" + order.ID,
			Type:      "reserve_inventory",
			SagaID:    sagaID,
			OrderID:   order.ID,
			SKU:       order.SKU,
			Qty:       order.Qty,
		}
		if err := u.enqueueCommand(ctx, tx, order.ID, reserve); err != nil {
			return err
		}

		charge := sagaCommand{
			CommandID: "cmd-charge-" + order.ID,
			Type:      "charge_payment",
			SagaID:    sagaID,
			OrderID:   order.ID,
			AccountID: order.AccountID,
			Amount:    order.Amount,
		}
		return u.enqueueCommand(ctx, tx, order.ID, charge)
	})
	if err != nil {
		return domain.Order{}, err
	}

	return order, nil
}

func (u *OrderUseCase) GetOrderByID(ctx context.Context, id string) (domain.Order, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Order{}, domain.ErrInvalidOrder
	}
	return u.repo.GetByID(ctx, id)
}

func (u *OrderUseCase) enqueueCommand(ctx context.Context, q postgres.DBTX, key string, cmd sagaCommand) error {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return u.enqueue(ctx, q, u.commandsTopic, key, payload)
}

func (u *OrderUseCase) enqueue(ctx context.Context, q postgres.DBTX, topic, key string, payload []byte) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	return u.outbox.Enqueue(ctx, q, domain.OutboxMessage{
		ID:        id,
		Topic:     topic,
		Key:       key,
		Payload:   payload,
		Headers:   []byte("{}"),
		CreatedAt: time.Now().UTC(),
	})
}