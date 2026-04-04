package domain

import "errors"

var ErrInvalidOrder = errors.New("invalid order")
var ErrOrderNotFound = errors.New("order not found")
var ErrSagaNotFound = errors.New("saga not found")
