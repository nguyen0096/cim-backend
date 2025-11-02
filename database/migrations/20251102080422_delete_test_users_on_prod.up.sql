-- Delete test users from production (if they exist)
DELETE FROM users WHERE uid IN ('demoRemovedTestUidA000000000', 'demoRemovedTestUidB000000000');
