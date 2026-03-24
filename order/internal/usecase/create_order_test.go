package usecase

import (
	"context"
	"testing"

	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/usecase/mocks"
)

func TestOrderUseCase_CreateOrder(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateOrderInput
		setup   func(repo *mocks.OrderRepository, publisher *mocks.EventPublisher)
		wantErr error
	}{
		{
			name: "invalid order input",
			input: CreateOrderInput{
				Customer: "",
				Amount:   0,
			},
			setup:   func(_ *mocks.OrderRepository, _ *mocks.EventPublisher) {},
			wantErr: domain.ErrInvalidOrder,
		},
		// TODO: add happy-path and error-path cases:
		// - repo save failed
		// - publisher failed
		// - success path with repo + publisher expectations
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewOrderRepository(t)
			publisher := mocks.NewEventPublisher(t)
			tt.setup(repo, publisher)

			u := NewOrderUseCase(repo, publisher, "test-topic")

			_, err := u.CreateOrder(context.Background(), tt.input)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("CreateOrder() unexpected error: %v", err)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("CreateOrder() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestOrderUseCase_GetOrderByID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		setup   func(repo *mocks.OrderRepository)
		want    domain.Order
		wantErr bool
	}{
		{
			name:    "empty id",
			id:      "",
			setup:   func(_ *mocks.OrderRepository) {},
			wantErr: true,
		},
		{
			name: "found by id",
			id:   "order-1",
			setup: func(repo *mocks.OrderRepository) {
				repo.EXPECT().
					GetByID(context.Background(), "order-1").
					Return(domain.Order{ID: "order-1", Customer: "alice", Amount: 100}, nil)
			},
			want: domain.Order{
				ID:       "order-1",
				Customer: "alice",
				Amount:   100,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewOrderRepository(t)
			publisher := mocks.NewEventPublisher(t)
			tt.setup(repo)

			u := NewOrderUseCase(repo, publisher, "test-topic")
			got, err := u.GetOrderByID(context.Background(), tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetOrderByID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("GetOrderByID() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}
