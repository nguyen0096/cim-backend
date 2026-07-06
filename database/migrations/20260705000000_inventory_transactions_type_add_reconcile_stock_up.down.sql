-- Irreversible if any reconcile_stock_up rows exist: the narrowed CHECK below
-- rejects that value, so this down-migration fails until such rows are removed or
-- retyped. There is no safe automatic retype (the surplus stock they back has no
-- other transaction kind), so no data rewrite is attempted here.
ALTER TABLE inventory_transactions
    DROP CONSTRAINT IF EXISTS chk_inventory_transactions_type;

ALTER TABLE inventory_transactions
    ADD CONSTRAINT chk_inventory_transactions_type
        CHECK (transaction_type IN ('purchase', 'disposal', 'sell', 'transfer_out', 'transfer_in'));
