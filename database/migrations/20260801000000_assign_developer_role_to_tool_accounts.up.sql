-- Assign the 'developer' role (shipped in #146) to the two maintenance accounts so
-- the Developer Tools screen and its endpoints become reachable. users.role holds a
-- single role, so while assigned these accounts are developer-only.
--
-- Both statements are naturally no-ops when a UID has no users row, so this cannot
-- fail a deploy in an environment where the account does not exist.

-- Prior roles are recorded per users.id so the down migration restores each account's
-- exact previous value instead of guessing a default. Keyed by id, not uid, because
-- idx_users_uid is not unique.
CREATE TABLE IF NOT EXISTS users_role_backup_20260801000000 (
    user_id       UUID PRIMARY KEY,
    uid           VARCHAR(255) NOT NULL,
    previous_role VARCHAR(50),
    backed_up_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- ON CONFLICT DO NOTHING keeps the first-recorded value, so a re-run cannot overwrite
-- the real prior role with 'developer'.
INSERT INTO users_role_backup_20260801000000 (user_id, uid, previous_role)
SELECT u.id, u.uid, u.role
FROM users u
WHERE u.uid IN ('demoRootAdminUid000000000000', 'demoMaintAdminUid00000000000')
  AND u.deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING;

UPDATE users
SET role = 'developer', updated_at = NOW()
WHERE uid IN ('demoRootAdminUid000000000000', 'demoMaintAdminUid00000000000')
  AND deleted_at IS NULL
  AND role IS DISTINCT FROM 'developer';
