-- Re-collapse the status CHECK to ('in_progress'), restoring the post-20260624000000
-- state. Any 'ready_for_review' row is relabeled to 'in_progress' first (a pure
-- relabel; counts unchanged) so the narrower constraint applies cleanly.
UPDATE reconciliation_request_items
    SET status = 'in_progress'
    WHERE status <> 'in_progress';

ALTER TABLE reconciliation_request_items
    DROP CONSTRAINT IF EXISTS chk_reconciliation_request_items_status;

ALTER TABLE reconciliation_request_items
    ADD CONSTRAINT chk_reconciliation_request_items_status
        CHECK (status IN ('in_progress'));
