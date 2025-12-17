-- Remove product_image column from products table
ALTER TABLE products
DROP COLUMN IF EXISTS product_image;
