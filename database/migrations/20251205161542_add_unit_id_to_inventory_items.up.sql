-- Add unit_id column to inventory_items table (nullable first to handle existing data)
ALTER TABLE inventory_items
ADD COLUMN IF NOT EXISTS unit_id BIGINT;

-- Update existing rows: set unit_id to the product's unit_id
-- This assumes products have a unit_id and we want to use the product's unit as default
UPDATE inventory_items ii
SET unit_id = p.unit_id
FROM products p
WHERE ii.product_id = p.id AND ii.unit_id IS NULL;

-- Now make the column NOT NULL (only if it's not already NOT NULL)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'inventory_items'
        AND column_name = 'unit_id'
        AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE inventory_items
        ALTER COLUMN unit_id SET NOT NULL;
    END IF;
END $$;

-- Add foreign key constraint to units table (only if it doesn't exist)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_inventory_items_unit'
    ) THEN
        ALTER TABLE inventory_items
        ADD CONSTRAINT fk_inventory_items_unit
        FOREIGN KEY (unit_id) REFERENCES units(id);
    END IF;
END $$;

-- Add index on unit_id for better query performance
CREATE INDEX IF NOT EXISTS idx_inventory_items_unit_id ON inventory_items(unit_id);
