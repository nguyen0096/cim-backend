-- Delete users with empty or NULL type
DELETE FROM users
WHERE type IS NULL OR type = '';
