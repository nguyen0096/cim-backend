-- Drop the reconciliation lifecycle status + processed-at marker. Dropping the
-- columns also discards both of the up migration's backfills — the `open` backfill
-- of in-flight initiated reconciles (reconcile_status) and the processed_at backfill
-- of pre-existing completed consuming submissions — so no separate UPDATE is needed
-- to reverse either.
ALTER TABLE inventory_submissions
    DROP CONSTRAINT IF EXISTS chk_inventory_submissions_reconcile_status;

ALTER TABLE inventory_submissions
    DROP COLUMN IF EXISTS processed_at;

ALTER TABLE inventory_submissions
    DROP COLUMN IF EXISTS reconcile_status;
