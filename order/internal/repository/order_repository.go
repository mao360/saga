package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/order/internal/domain"
)

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Init(ctx context.Context) error {
	const q = `
	create table if not exists orders (
		id text primary key,
		customer text not null,
		amount bigint not null,
		created_at timestamptz not null
	)`
	_, err := r.db.Exec(ctx, q)
	return err
}

func (r *OrderRepository) Save(ctx context.Context, order domain.Order) error {
	const q = `
	insert into orders (id, customer, amount, created_at)
	values ($1, $2, $3, $4)`
	_, err := r.db.Exec(ctx, q, order.ID, order.Customer, order.Amount, order.CreatedAt)
	return err
}

func (r *OrderRepository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}
