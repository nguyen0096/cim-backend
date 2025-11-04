-- Restore location column to payment_receipt_forms table
ALTER TABLE payment_receipt_forms
ADD COLUMN IF NOT EXISTS location VARCHAR(255);

