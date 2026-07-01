-- Add `canceled` to the reconcile_status CHECK so the cancel terminal state is
-- accepted. Drop + re-ADD the constraint (Postgres cannot alter a CHECK in place).
-- The shipped 20260624000001 constraint is not edited.
ALTER TABLE inventory_submissions
    DROP CONSTRAINT IF EXISTS chk_inventory_submissions_reconcile_status;

ALTER TABLE inventory_submissions
    ADD CONSTRAINT chk_inventory_submissions_reconcile_status
        CHECK (reconcile_status IS NULL
            OR reconcile_status = ''
            OR reconcile_status IN ('open', 'closed', 'processing', 'processed', 'canceled'));
