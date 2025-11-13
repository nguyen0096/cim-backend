-- Rollback: Convert quantity fields from decimal(10,2) back to integer

-- Update inventory_items table
ALTER TABLE inventory_items
  ALTER COLUMN quantity TYPE INTEGER;

-- Update purchase_order_items table
ALTER TABLE purchase_order_items
  ALTER COLUMN quantity TYPE INTEGER,
  ALTER COLUMN received_quantity TYPE INTEGER;

-- Update inventory_transactions table
ALTER TABLE inventory_transactions
  ALTER COLUMN quantity TYPE INTEGER,
  ALTER COLUMN consumed_quantity TYPE INTEGER;
