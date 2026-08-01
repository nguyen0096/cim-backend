-- Track applied initial-stock loads so a repeated submit under the same key is
-- replayed rather than re-applied. result_summary holds the full response payload,
-- so a replay satisfies the same response contract as the original run.
--
-- Uniqueness is scoped per inventory (not global on idempotency_key): a global
-- index would make the same key against a second inventory look like an applied
-- load and return the first run's result for work never done.
CREATE TABLE IF NOT EXISTS initial_stock_imports (
    id SERIAL PRIMARY KEY,
    idempotency_key VARCHAR(255) NOT NULL,
    inventory_id INTEGER NOT NULL REFERENCES inventories(id) ON DELETE CASCADE,
    sheet_name TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_sha256 CHAR(64) NOT NULL,
    row_count INT NOT NULL,
    result_summary JSONB NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_initial_stock_imports_inventory_key
    ON initial_stock_imports(inventory_id, idempotency_key);
