-- Update type for root admin users to 'developer'
UPDATE users
SET type = 'developer', updated_at = NOW()
WHERE uid IN ('demoRootAdminUid000000000000', 'demoRootAdminUid200000000000');
