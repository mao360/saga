package domain

import "time"

type Order struct {
	ID        string
	Customer  string
	Amount    int64
	SKU       string
	Qty       int64
	AccountID string
	CreatedAt time.Time
}
