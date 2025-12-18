-- Add product_image column to products table
ALTER TABLE products
ADD COLUMN IF NOT EXISTS product_image BYTEA;
