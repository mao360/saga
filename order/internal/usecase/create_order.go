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

type EventPublisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type OrderUseCase struct {
	repo      OrderRepository
	publisher EventPublisher
	topic     string
}

type CreateOrderInput struct {
	Customer string `json:"customer"`
	Amount   int64  `json:"amount"`
}

func NewOrderUseCase(repo OrderRepository, publisher EventPublisher, topic string) *OrderUseCase {
	return &OrderUseCase{
		repo:      repo,
		publisher: publisher,
		topic:     topic,
	}
}

func (u *OrderUseCase) CreateTestOrder(ctx context.Context) (domain.Order, error) {
	return u.CreateOrder(ctx, CreateOrderInput{
		Customer: "test-user",
		Amount:   100,
	})
}

func (u *OrderUseCase) CreateOrder(ctx context.Context, input CreateOrderInput) (domain.Order, error) {
	now := time.Now().UTC()
	order := domain.Order{
		ID:        strconv.FormatInt(now.UnixNano(), 10),
		Customer:  strings.TrimSpace(input.Customer),
		Amount:    input.Amount,
		CreatedAt: now,
	}

	if order.Amount <= 0 || order.Customer == "" {
		return domain.Order{}, domain.ErrInvalidOrder
	}

	if err := u.repo.Save(ctx, order); err != nil {
		return domain.Order{}, err
	}

	payload, err := json.Marshal(order)
	if err != nil {
		return domain.Order{}, err
	}

	if err := u.publisher.Publish(ctx, u.topic, []byte(order.ID), payload); err != nil {
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
