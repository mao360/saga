-- Частичный индекс под опрос незавершённых саг (order_saga_pending_orders и
-- order_saga_oldest_pending_age_seconds). Индексируется только 'pending', так
-- что размер держится на числе саг в полёте, а не на всей таблице заказов.
CREATE INDEX IF NOT EXISTS idx_orders_pending_created_at
    ON orders (created_at)
    WHERE status = 'pending';