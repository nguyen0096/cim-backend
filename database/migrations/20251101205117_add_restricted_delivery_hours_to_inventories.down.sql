-- Remove restricted_delivery_hours column from inventories table
ALTER TABLE inventories
DROP COLUMN IF EXISTS restricted_delivery_hours;

