package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/platform/postgres"
)

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

// Save вставляет заказ. q может быть *pgxpool.Pool или pgx.Tx; последнее
// позволяет включить вставку заказа в общую транзакцию с outbox/saga_state.
func (r *OrderRepository) Save(ctx context.Context, q postgres.DBTX, order domain.Order) error {
	const query = `
		insert into orders (id, customer, amount, sku, qty, account_id, status, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := q.Exec(ctx, query,
		order.ID, order.Customer, order.Amount,
		order.SKU, order.Qty, order.AccountID,
		order.Status, order.CreatedAt,
	)
	return err
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (domain.Order, error) {
	return r.getByID(ctx, r.db, id)
}

// GetByIDTx — версия GetByID, использующая переданный querier (tx). Нужна
// в ProcessEventUseCase, где чтение заказа и запись компенсирующей команды
// в outbox идут в одной транзакции.
func (r *OrderRepository) GetByIDTx(ctx context.Context, q postgres.DBTX, id string) (domain.Order, error) {
	return r.getByID(ctx, q, id)
}

func (r *OrderRepository) getByID(ctx context.Context, q postgres.DBTX, id string) (domain.Order, error) {
	const query = `
		select id, customer, amount, sku, qty, account_id, status, created_at
		from orders
		where id = $1`

	var o domain.Order
	err := q.QueryRow(ctx, query, id).Scan(
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

func (r *OrderRepository) UpdateStatus(ctx context.Context, q postgres.DBTX, orderID, status string) error {
	const query = `update orders set status = $1 where id = $2`
	_, err := q.Exec(ctx, query, status, orderID)
	return err
}

func (r *OrderRepository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}