-- Remove index on unit_id
DROP INDEX IF EXISTS idx_purchase_order_items_unit_id;

-- Remove foreign key constraint
ALTER TABLE purchase_order_items
DROP CONSTRAINT IF EXISTS fk_purchase_order_items_unit;

-- Remove unit_id column from purchase_order_items table
ALTER TABLE purchase_order_items
DROP COLUMN IF EXISTS unit_id;

