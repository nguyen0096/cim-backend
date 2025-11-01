-- Revert type for root admin users back to NULL or default value
UPDATE users
SET type = NULL, updated_at = NOW()
WHERE uid IN ('demoRootAdminUid000000000000', 'demoRootAdminUid200000000000');
