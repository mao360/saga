package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	OrderStatusPending   = "pending"
	OrderStatusCompleted = "completed"
	OrderStatusFailed    = "failed"
)

// Order сериализуется прямо в HTTP-ответ, поэтому теги обязательны: без них
// наружу уезжают имена Go-полей (ID, SKU, AccountID), и любой клиент,
// ожидающий snake_case, молча получает undefined.
type Order struct {
	ID        string    `json:"id"`
	Customer  string    `json:"customer"`
	Amount    int64     `json:"amount"`
	SKU       string    `json:"sku"`
	Qty       int64     `json:"qty"`
	AccountID string    `json:"account_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// SagaState хранит текущие статусы шагов саги в order-сервисе.
type SagaState struct {
	SagaID          string
	OrderID         string
	InventoryStatus string // pending | reserved | rejected | released
	PaymentStatus   string // pending | charged | rejected | refunded
}

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
