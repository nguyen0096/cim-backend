# Firefighter Guidelines

## Database and Backups

DB credentials are populated into `db-backup` container env. In order to inspect or modify database, run:

```
PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$PGHOST" -p 5432 -U "$POSTGRES_USER" -d "$POSTGRES_DB"
```

Backup tools: are located in `/usr/local/bin`

- `backup-db.sh`
- `recover-db.sh`

## User management

Create Bot Form account

```sql
-- Insert bot_form user
INSERT INTO users (uid, email, name, role, type, status, created_at, updated_at)
VALUES
    ('demoAutomationBotUid00000000', 'form@example.com', 'Automation Bot', 'bot_form', 'user', 'active', NOW(), NOW());
```

Create staff account

```sql
-- Insert staff user
INSERT INTO users (uid, email, name, role, type, status, created_at, updated_at)
VALUES
    ('demoStaffAccountUid000000000', 'staff@example.com', 'Staff User', 'staff', 'user', 'active', NOW(), NOW());
```
