-- Re-apply 20260801000000 for the accounts it backed up. The backup table is kept by
-- the up migration precisely so this is reversible; 20260801000000's own down drops it.

DO $$
BEGIN
    IF to_regclass('users_role_backup_20260801000000') IS NULL THEN
        RETURN;
    END IF;

    UPDATE users
    SET role = 'developer', updated_at = NOW()
    WHERE id IN (SELECT user_id FROM users_role_backup_20260801000000)
      AND role IS DISTINCT FROM 'developer';
END $$;
