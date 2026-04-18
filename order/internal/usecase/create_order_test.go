package usecase

import (
	"context"
	"testing"

	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/usecase/mocks"
)

func TestOrderUseCase_CreateOrder_InvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateOrderInput
		wantErr error
	}{
		{
			name:    "empty customer and zero amount",
			input:   CreateOrderInput{Customer: "", Amount: 0},
			wantErr: domain.ErrInvalidOrder,
		},
		{
			name:    "missing sku",
			input:   CreateOrderInput{Customer: "alice", Amount: 100, Qty: 1, AccountID: "acc-1"},
			wantErr: domain.ErrInvalidOrder,
		},
		{
			name:    "zero qty",
			input:   CreateOrderInput{Customer: "alice", Amount: 100, SKU: "sku-1", Qty: 0, AccountID: "acc-1"},
			wantErr: domain.ErrInvalidOrder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := NewOrderUseCase(
				mocks.NewOrderRepository(t),
				mocks.NewSagaCreator(t),
				mocks.NewOutboxEnqueuer(t),
				mocks.NewTxRunner(t),
				"test-topic", "test-commands",
			)

			_, err := u.CreateOrder(context.Background(), tt.input)
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
			want:    domain.Order{ID: "order-1", Customer: "alice", Amount: 100},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewOrderRepository(t)
			tt.setup(repo)

			u := NewOrderUseCase(
				repo,
				mocks.NewSagaCreator(t),
				mocks.NewOutboxEnqueuer(t),
				mocks.NewTxRunner(t),
				"test-topic", "test-commands",
			)
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