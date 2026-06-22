-- reconciliation_snapshots (epic #38, Part 1 — B1, plain table, NOT partitioned).
--
-- Captures the per-item baseline quantity at the moment a reconciliation is
-- initiated ("Start reconcile process"). This snapshot is the SOLE source of
-- truth for prev_quantity during reconciliation: staff reconciliation_request_items
-- carry counted quantities only, never independent baselines. The Sell sizing at
-- apply time is computed as (snapshot.prev_quantity - counted) per the locked
-- "Reading B" decision, so received purchase orders survive reconciliation.
--
-- One snapshot row per (submission, inventory item). prev_quantity is NUMERIC to
-- match inventory_items.quantity (decimal). Soft-delete (deleted_at) and the
-- created_by/updated_by/created_at/updated_at audit columns follow the repo's
-- models.Base convention.
CREATE TABLE IF NOT EXISTS reconciliation_snapshots (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER NOT NULL REFERENCES inventory_submissions(id),
    inventory_item_id INTEGER NOT NULL REFERENCES inventory_items(id),
    prev_quantity NUMERIC NOT NULL,
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- One snapshot per (submission, inventory item). Partial on live rows so a
-- soft-deleted snapshot never blocks re-capturing a baseline for the same pair.
CREATE UNIQUE INDEX IF NOT EXISTS uq_reconciliation_snapshots_submission_item_active
    ON reconciliation_snapshots (submission_id, inventory_item_id)
    WHERE deleted_at IS NULL;

-- Lookup baselines by submission during synthesize/review/approve.
CREATE INDEX IF NOT EXISTS idx_reconciliation_snapshots_submission_id
    ON reconciliation_snapshots (submission_id);

-- created_at index per the locked schema.
CREATE INDEX IF NOT EXISTS idx_reconciliation_snapshots_created_at
    ON reconciliation_snapshots (created_at);

-- Match models.Base soft-delete index convention.
CREATE INDEX IF NOT EXISTS idx_reconciliation_snapshots_deleted_at
    ON reconciliation_snapshots (deleted_at);
