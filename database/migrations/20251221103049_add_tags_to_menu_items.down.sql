-- Remove tags index
DROP INDEX IF EXISTS idx_menu_items_tags;

-- Remove tags column from menu_items table
ALTER TABLE menu_items
DROP COLUMN IF EXISTS tags;

