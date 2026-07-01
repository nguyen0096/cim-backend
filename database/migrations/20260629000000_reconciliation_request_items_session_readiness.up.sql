-- Widen the reconciliation_request_items status CHECK to admit per-session
-- readiness, restoring 'ready_for_review' on the column collapsed by migration
-- 20260624000000. No backfill (column already defaults to 'in_progress').
ALTER TABLE reconciliation_request_items
    DROP CONSTRAINT IF EXISTS chk_reconciliation_request_items_status;

ALTER TABLE reconciliation_request_items
    ADD CONSTRAINT chk_reconciliation_request_items_status
        CHECK (status IN ('in_progress', 'ready_for_review'));
