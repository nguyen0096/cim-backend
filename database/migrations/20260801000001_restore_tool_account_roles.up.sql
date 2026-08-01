-- Reverse 20260801000000. The two maintenance accounts now reach admin and developer
-- through their g rows in rbac_policy.csv, so users.role no longer has to carry
-- 'developer'. No-op when the backup table is absent or empty.

DO $$
BEGIN
    IF to_regclass('users_role_backup_20260801000000') IS NULL THEN
        RETURN;
    END IF;

    UPDATE users u
    SET role = b.previous_role, updated_at = NOW()
    FROM users_role_backup_20260801000000 b
    WHERE u.id = b.user_id
      AND u.role IS DISTINCT FROM b.previous_role;
END $$;
