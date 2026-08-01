-- Validate the widened chk_inventory_transactions_type added NOT VALID by the prior
-- migration. VALIDATE takes only SHARE UPDATE EXCLUSIVE (non-blocking for reads and
-- writes) and must run in its own transaction, so it holds ONE statement.
ALTER TABLE inventory_transactions
    VALIDATE CONSTRAINT chk_inventory_transactions_type;
