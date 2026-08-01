-- Restore each account's exact pre-migration role from the backup recorded by the up
-- migration. Rows the up migration never touched have no backup row and stay as they
-- are, so this is also a no-op where the accounts do not exist.

UPDATE users u
SET role = b.previous_role, updated_at = NOW()
FROM users_role_backup_20260801000000 b
WHERE u.id = b.user_id;

DROP TABLE IF EXISTS users_role_backup_20260801000000;
