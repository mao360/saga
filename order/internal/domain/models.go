package domain

import "time"

const (
	OrderStatusPending   = "pending"
	OrderStatusCompleted = "completed"
	OrderStatusFailed    = "failed"
)

type Order struct {
	ID        string
	Customer  string
	Amount    int64
	SKU       string
	Qty       int64
	AccountID string
	Status    string
	CreatedAt time.Time
}

// SagaState хранит текущие статусы шагов саги в order-сервисе.
type SagaState struct {
	SagaID          string
	OrderID         string
	InventoryStatus string // pending | reserved | rejected | released
	PaymentStatus   string // pending | charged | rejected | refunded
}

// SagaEvent — унифицированный тип события, которое order-сервис получает
// от inventory-сервиса и payment-сервиса.
type SagaEvent struct {
	Type       string    `json:"type"`
	CommandID  string    `json:"command_id"`
	SagaID     string    `json:"saga_id"`
	OrderID    string    `json:"order_id"`
	SKU        string    `json:"sku,omitempty"`
	Qty        int64     `json:"qty,omitempty"`
	AccountID  string    `json:"account_id,omitempty"`
	Amount     int64     `json:"amount,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
	Reason     string    `json:"reason,omitempty"`
}
