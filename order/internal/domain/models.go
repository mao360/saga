package domain

import "time"

type Order struct {
	ID        string
	Customer  string
	Amount    int64
	CreatedAt time.Time
}
