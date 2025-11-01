-- Remove uid column from users table
-- First drop the unique index on uid if it exists
DROP INDEX IF EXISTS idx_users_uid;

-- Drop the uid column
ALTER TABLE users DROP COLUMN IF EXISTS uid;
