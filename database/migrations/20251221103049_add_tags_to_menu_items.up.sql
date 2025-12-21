-- Add tags column to menu_items table
-- Tags is a PostgreSQL array type (text[]) for storing multiple tag values
ALTER TABLE menu_items
ADD COLUMN IF NOT EXISTS tags TEXT[];

-- Create GIN index on tags column for efficient array queries
-- GIN index supports array operators like && (overlap), @> (contains), etc.
CREATE INDEX IF NOT EXISTS idx_menu_items_tags ON menu_items USING GIN (tags);

