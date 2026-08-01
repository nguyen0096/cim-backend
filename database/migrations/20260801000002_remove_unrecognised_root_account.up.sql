-- Remove the root account seeded by 20251024032328 whose UID Firebase does not
-- recognise. Soft delete, matching gorm.DeletedAt on models.User and
-- UserRepository.Delete. Expected to match no row: the email-keyed UID repair at
-- internal/middleware/authorization.go:61-66 has since moved that email onto
-- demoRootAdminUid000000000000.

-- Rows this migration actually soft-deletes are recorded, so the down migration
-- undeletes those and nothing else; a row already soft-deleted beforehand must stay
-- deleted. Same recording pattern as users_role_backup_20260801000000.
CREATE TABLE IF NOT EXISTS users_deleted_backup_20260801000002 (
    user_id      UUID PRIMARY KEY,
    uid          VARCHAR(255) NOT NULL,
    backed_up_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

INSERT INTO users_deleted_backup_20260801000002 (user_id, uid)
SELECT u.id, u.uid
FROM users u
WHERE u.uid = 'demoRootAdminUid200000000000'
  AND u.deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING;

UPDATE users
SET deleted_at = NOW(), updated_at = NOW()
WHERE uid = 'demoRootAdminUid200000000000'
  AND deleted_at IS NULL;
