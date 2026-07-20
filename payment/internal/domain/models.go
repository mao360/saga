package domain

import (
	"time"

	"github.com/google/uuid"
)

// OutboxMessage — строка таблицы outbox_messages. Перед публикацией в Kafka
// сообщение сохраняется в той же транзакции, что и бизнес-изменения, что
// устраняет dual-write и даёт at-least-once доставку.
type OutboxMessage struct {
	ID        uuid.UUID
	Topic     string
	Key       string
	Payload   []byte
	Headers   []byte
	CreatedAt time.Time
	Attempts  int
}

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
