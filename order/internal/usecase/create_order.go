package usecase

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/mao360/saga/order/internal/domain"
)

type OrderRepository interface {
	Save(ctx context.Context, order domain.Order) error
	GetByID(ctx context.Context, id string) (domain.Order, error)
}

type SagaCreator interface {
	Create(ctx context.Context, sagaID, orderID string) error
}

type EventPublisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type OrderUseCase struct {
	repo          OrderRepository
	saga          SagaCreator
	publisher     EventPublisher
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

func NewOrderUseCase(repo OrderRepository, saga SagaCreator, publisher EventPublisher, orderTopic, commandsTopic string) *OrderUseCase {
	return &OrderUseCase{
		repo:          repo,
		saga:          saga,
		publisher:     publisher,
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

	if err := u.repo.Save(ctx, order); err != nil {
		return domain.Order{}, err
	}

	sagaID := "saga-" + order.ID
	if err := u.saga.Create(ctx, sagaID, order.ID); err != nil {
		return domain.Order{}, err
	}

	payload, err := json.Marshal(order)
	if err != nil {
		return domain.Order{}, err
	}

	if err := u.publisher.Publish(ctx, u.orderTopic, []byte(order.ID), payload); err != nil {
		return domain.Order{}, err
	}
	reserve := sagaCommand{
		CommandID: "cmd-reserve-" + order.ID,
		Type:      "reserve_inventory",
		SagaID:    sagaID,
		OrderID:   order.ID,
		SKU:       order.SKU,
		Qty:       order.Qty,
	}
	if err := u.publishCommand(ctx, order.ID, reserve); err != nil {
		return domain.Order{}, err
	}

	charge := sagaCommand{
		CommandID: "cmd-charge-" + order.ID,
		Type:      "charge_payment",
		SagaID:    sagaID,
		OrderID:   order.ID,
		AccountID: order.AccountID,
		Amount:    order.Amount,
	}
	if err := u.publishCommand(ctx, order.ID, charge); err != nil {
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

func (u *OrderUseCase) publishCommand(ctx context.Context, key string, cmd sagaCommand) error {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return u.publisher.Publish(ctx, u.commandsTopic, []byte(key), payload)
}
