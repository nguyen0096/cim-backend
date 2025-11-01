-- Change UID from uniqueIndex to regular index
-- Drop existing unique index/constraint on UID
DROP INDEX IF EXISTS idx_users_uid;
DROP INDEX IF EXISTS users_uid_key;

-- Create regular index on UID
CREATE INDEX IF NOT EXISTS idx_users_uid ON users(uid);
