-- Terminalize canceled reconciles before restoring the pre-canceled CHECK (which
-- does not admit 'canceled'). approval_status='rejected' is the terminal the
-- pre-cancel code honors as not-in-flight, so the abandoned count is not
-- start-processable after rollback; reconcile_status='closed' is the admitted value.
UPDATE inventory_submissions
SET reconcile_status = 'closed',
    approval_status = 'rejected'
WHERE reconcile_status = 'canceled';

-- Restore the reconcile_status CHECK to its pre-canceled set.
ALTER TABLE inventory_submissions
    DROP CONSTRAINT IF EXISTS chk_inventory_submissions_reconcile_status;

ALTER TABLE inventory_submissions
    ADD CONSTRAINT chk_inventory_submissions_reconcile_status
        CHECK (reconcile_status IS NULL
            OR reconcile_status = ''
            OR reconcile_status IN ('open', 'closed', 'processing', 'processed'));
