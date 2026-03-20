package usecase

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/mao360/saga/order/internal/domain"
)

type OrderRepository interface {
	Save(ctx context.Context, order domain.Order) error
}

type EventPublisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type OrderUseCase struct {
	repo      OrderRepository
	publisher EventPublisher
	topic     string
}

func NewOrderUseCase(repo OrderRepository, publisher EventPublisher, topic string) *OrderUseCase {
	return &OrderUseCase{
		repo:      repo,
		publisher: publisher,
		topic:     topic,
	}
}

func (u *OrderUseCase) CreateTestOrder(ctx context.Context) (domain.Order, error) {
	now := time.Now().UTC()
	order := domain.Order{
		ID:        strconv.FormatInt(now.UnixNano(), 10),
		Customer:  "test-user",
		Amount:    100,
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
