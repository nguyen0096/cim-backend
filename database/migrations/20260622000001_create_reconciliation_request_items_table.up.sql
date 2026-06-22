-- reconciliation_request_items (epic #38, Part 1).
--
-- One row per staff/batch contribution to an in-flight reconciliation. Each row
-- carries a JSONB payload in the legacy reconcile shape
-- {"items":[{"inventory_item_id":<id>,"quantity":<counted>}]} holding COUNTED
-- quantities only (never an independent baseline — the baseline lives in
-- reconciliation_snapshots). At apply time these active rows are summed by
-- inventory_item_id into the finalized synthesized ReconcileInventoryRequest
-- payload stored on inventory_submissions.
--
-- Per-row state machine (status):
--   in_progress -> ready -> approved -> applied
-- with an escape hatch: approved -> in_progress on a staff edit. Staff may
-- soft-delete only their own in_progress/ready rows. Once applied the row is
-- immutable. Enforcement of these transitions lives in service logic (later
-- parts); this migration only persists the column + a sane default + a CHECK on
-- the allowed value set.
--
-- created_by (the contributing staff) is provided by the repo-wide models.Base
-- convention (user email, VARCHAR(255)); no separate user-id column is added, to
-- stay consistent with every other table in this codebase. See PR self-check for
-- this assumption.
CREATE TABLE IF NOT EXISTS reconciliation_request_items (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER NOT NULL REFERENCES inventory_submissions(id),
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'in_progress',
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT chk_reconciliation_request_items_status
        CHECK (status IN ('in_progress', 'ready', 'approved', 'applied'))
);

-- Synthesize/list/review all scope to a submission's active child rows.
CREATE INDEX IF NOT EXISTS idx_reconciliation_request_items_submission_id
    ON reconciliation_request_items (submission_id);

-- Match models.Base soft-delete index convention.
CREATE INDEX IF NOT EXISTS idx_reconciliation_request_items_deleted_at
    ON reconciliation_request_items (deleted_at);
