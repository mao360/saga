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
	insert into orders (id, customer, amount, created_at)
	values ($1, $2, $3, $4)`
	_, err := r.db.Exec(ctx, q, order.ID, order.Customer, order.Amount, order.CreatedAt)
	return err
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (domain.Order, error) {
	const q = `
	select id, customer, amount, created_at
	from orders
	where id = $1`

	var order domain.Order
	err := r.db.QueryRow(ctx, q, id).Scan(
		&order.ID,
		&order.Customer,
		&order.Amount,
		&order.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, domain.ErrOrderNotFound
		}
		return domain.Order{}, err
	}

	return order, nil
}

func (r *OrderRepository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}
