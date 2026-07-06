-- Add `reconcile_stock_up` to the transaction_type CHECK so reconciliation surplus
-- stock-up txns are accepted. Drop + re-ADD the constraint (Postgres cannot alter a
-- CHECK in place). The shipped 20251023143213 constraint is not edited.
--
-- ADD ... NOT VALID skips the validating full scan (this only widens the allowed
-- set, so all existing rows already satisfy it), keeping the ACCESS EXCLUSIVE lock
-- brief. The VALIDATE runs in the next migration so its SHARE UPDATE EXCLUSIVE scan
-- is not folded into this file's implicit transaction (which holds ACCESS EXCLUSIVE
-- until commit).
ALTER TABLE inventory_transactions
    DROP CONSTRAINT IF EXISTS chk_inventory_transactions_type;

ALTER TABLE inventory_transactions
    ADD CONSTRAINT chk_inventory_transactions_type
        CHECK (transaction_type IN ('purchase', 'disposal', 'sell', 'transfer_out', 'transfer_in', 'reconcile_stock_up'))
        NOT VALID;
