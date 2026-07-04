-- Harden purchase_order_receipts idempotency:
-- scope the key per purchase order, align the FK to the INTEGER convention,
-- and cascade on purchase order delete.

-- Scope uniqueness to (purchase_order_id, idempotency_key). The old global
-- unique index on idempotency_key alone made reusing a key across two orders
-- silently drop the second order's receive.
DROP INDEX IF EXISTS idx_purchase_order_receipts_idempotency_key;

-- Recreate the FK with ON DELETE CASCADE (else it blocks hard-delete of a PO)
-- and narrow the type to INTEGER. purchase_order_id holds purchase_orders.id
-- (SERIAL/integer), so the BIGINT -> INTEGER change is safe for existing rows.
ALTER TABLE purchase_order_receipts
    DROP CONSTRAINT IF EXISTS purchase_order_receipts_purchase_order_id_fkey;

ALTER TABLE purchase_order_receipts
    ALTER COLUMN purchase_order_id TYPE INTEGER;

ALTER TABLE purchase_order_receipts
    ADD CONSTRAINT purchase_order_receipts_purchase_order_id_fkey
    FOREIGN KEY (purchase_order_id) REFERENCES purchase_orders(id) ON DELETE CASCADE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_purchase_order_receipts_po_key
    ON purchase_order_receipts(purchase_order_id, idempotency_key);
