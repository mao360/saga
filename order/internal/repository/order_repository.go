package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/order/internal/domain"
)

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Save(ctx context.Context, order domain.Order) error {
	const q = `
	insert into orders (id, customer, amount, sku, qty, account_id, status, created_at)
	values ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.Exec(ctx, q,
		order.ID, order.Customer, order.Amount,
		order.SKU, order.Qty, order.AccountID,
		order.Status, order.CreatedAt,
	)
	return err
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (domain.Order, error) {
	const q = `
	select id, customer, amount, sku, qty, account_id, status, created_at
	from orders
	where id = $1`

	var o domain.Order
	err := r.db.QueryRow(ctx, q, id).Scan(
		&o.ID, &o.Customer, &o.Amount,
		&o.SKU, &o.Qty, &o.AccountID,
		&o.Status, &o.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, domain.ErrOrderNotFound
		}
		return domain.Order{}, err
	}
	return o, nil
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, orderID, status string) error {
	const q = `update orders set status = $1 where id = $2`
	_, err := r.db.Exec(ctx, q, status, orderID)
	return err
}

func (r *OrderRepository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}