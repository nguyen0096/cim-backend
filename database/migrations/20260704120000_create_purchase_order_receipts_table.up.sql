-- Track applied purchase order receive submissions by idempotency key so a
-- repeated submit (double-click, refresh, retry) is applied at most once.
CREATE TABLE IF NOT EXISTS purchase_order_receipts (
    id SERIAL PRIMARY KEY,
    idempotency_key VARCHAR(255) NOT NULL,
    purchase_order_id BIGINT NOT NULL REFERENCES purchase_orders(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_purchase_order_receipts_idempotency_key
    ON purchase_order_receipts(idempotency_key);
