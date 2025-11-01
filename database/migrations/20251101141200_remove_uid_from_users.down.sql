-- Re-add uid column to users table
ALTER TABLE users ADD COLUMN uid VARCHAR;

-- Re-add unique index on uid
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uid ON users(uid);
