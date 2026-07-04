-- Revert to a global unique index on idempotency_key and the original
-- BIGINT FK without ON DELETE CASCADE.

-- Preflight: the forward migration scopes uniqueness per purchase order, so the
-- same idempotency_key may legitimately exist across different orders once the
-- new code has been live. Recreating a GLOBAL unique index on idempotency_key
-- would fail on those rows. Abort with a clear, actionable message instead of a
-- cryptic duplicate-key error; never delete data automatically.
DO $$
DECLARE
    dup_keys integer;
BEGIN
    SELECT count(*) INTO dup_keys FROM (
        SELECT idempotency_key
        FROM purchase_order_receipts
        GROUP BY idempotency_key
        HAVING count(*) > 1
    ) d;
    IF dup_keys > 0 THEN
        RAISE EXCEPTION 'Cannot roll back: % idempotency_key value(s) are shared across multiple purchase orders; a global unique index on idempotency_key cannot be recreated. Dedupe purchase_order_receipts (keep one row per idempotency_key) before rolling back this migration. No rows were modified.', dup_keys;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_purchase_order_receipts_po_key;

ALTER TABLE purchase_order_receipts
    DROP CONSTRAINT IF EXISTS purchase_order_receipts_purchase_order_id_fkey;

ALTER TABLE purchase_order_receipts
    ALTER COLUMN purchase_order_id TYPE BIGINT;

ALTER TABLE purchase_order_receipts
    ADD CONSTRAINT purchase_order_receipts_purchase_order_id_fkey
    FOREIGN KEY (purchase_order_id) REFERENCES purchase_orders(id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_purchase_order_receipts_idempotency_key
    ON purchase_order_receipts(idempotency_key);
