-- Insert root admin users
INSERT INTO users (uid, email, name, role, status, created_at, updated_at)
VALUES
    ('demoRootAdminUid000000000000', 'admin@example.com', 'Admin User', 'admin', 'active', NOW(), NOW()),
    ('demoRootAdminUid200000000000', 'admin2@example.com', 'Admin User', 'admin', 'active', NOW(), NOW())
