-- Remove index on unit_id
DROP INDEX IF EXISTS idx_inventory_items_unit_id;

-- Remove foreign key constraint
ALTER TABLE inventory_items
DROP CONSTRAINT IF EXISTS fk_inventory_items_unit;

-- Remove unit_id column from inventory_items table
ALTER TABLE inventory_items
DROP COLUMN IF EXISTS unit_id;
