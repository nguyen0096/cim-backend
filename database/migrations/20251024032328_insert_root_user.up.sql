-- Insert root admin users
INSERT INTO users (email, name, role, status, created_at, updated_at)
VALUES
    ('admin@example.com', 'Admin User', 'admin', 'active', NOW(), NOW()),
    ('admin2@example.com', 'Admin User', 'admin', 'active', NOW(), NOW())
ON CONFLICT (email) DO NOTHING;
