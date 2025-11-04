-- Remove location column from payment_receipt_forms table
ALTER TABLE payment_receipt_forms
DROP COLUMN IF EXISTS location;
