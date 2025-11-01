-- Add restricted_delivery_hours column to inventories table
ALTER TABLE inventories
ADD COLUMN IF NOT EXISTS restricted_delivery_hours VARCHAR(255);

