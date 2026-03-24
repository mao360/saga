package domain

import "time"

type Command struct {
	CommandID string `json:"command_id"`
	Type      string `json:"type"`
	SagaID    string `json:"saga_id"`
	OrderID   string `json:"order_id"`
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
}

type Event struct {
	Type       string    `json:"type"`
	CommandID  string    `json:"command_id"`
	SagaID     string    `json:"saga_id"`
	OrderID    string    `json:"order_id"`
	AccountID  string    `json:"account_id"`
	Amount     int64     `json:"amount"`
	OccurredAt time.Time `json:"occurred_at"`
	Reason     string    `json:"reason,omitempty"`
}
