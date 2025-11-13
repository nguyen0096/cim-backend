-- Convert quantity fields from integer to decimal(10,2)

-- Update inventory_items table
ALTER TABLE inventory_items
  ALTER COLUMN quantity TYPE DECIMAL(10,2);

-- Update purchase_order_items table
ALTER TABLE purchase_order_items
  ALTER COLUMN quantity TYPE DECIMAL(10,2),
  ALTER COLUMN received_quantity TYPE DECIMAL(10,2);

-- Update inventory_transactions table
ALTER TABLE inventory_transactions
  ALTER COLUMN quantity TYPE DECIMAL(10,2),
  ALTER COLUMN consumed_quantity TYPE DECIMAL(10,2);
