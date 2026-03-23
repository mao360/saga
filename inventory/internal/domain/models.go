package domain

import "time"

type Command struct {
	CommandID string `json:"command_id"`
	Type      string `json:"type"`
	SagaID    string `json:"saga_id"`
	OrderID   string `json:"order_id"`
	SKU       string `json:"sku"`
	Qty       int64  `json:"qty"`
}

type Event struct {
	Type       string    `json:"type"`
	CommandID  string    `json:"command_id"`
	SagaID     string    `json:"saga_id"`
	OrderID    string    `json:"order_id"`
	SKU        string    `json:"sku"`
	Qty        int64     `json:"qty"`
	OccurredAt time.Time `json:"occurred_at"`
	Reason     string    `json:"reason,omitempty"`
}
