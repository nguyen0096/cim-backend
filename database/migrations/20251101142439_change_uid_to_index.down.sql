-- Revert UID back to uniqueIndex
-- Drop regular index on UID
DROP INDEX IF EXISTS idx_users_uid;

-- Recreate unique index on UID
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uid ON users(uid);
