-- Remove root admin users
DELETE FROM users WHERE email IN ('admin@example.com', 'admin2@example.com');
