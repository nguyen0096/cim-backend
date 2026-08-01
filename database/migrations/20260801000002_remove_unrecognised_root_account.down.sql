-- Undelete only the rows the up migration soft-deleted. A row that was already
-- soft-deleted beforehand has no backup row and stays deleted.

UPDATE users u
SET deleted_at = NULL, updated_at = NOW()
FROM users_deleted_backup_20260801000002 b
WHERE u.id = b.user_id;

DROP TABLE IF EXISTS users_deleted_backup_20260801000002;
