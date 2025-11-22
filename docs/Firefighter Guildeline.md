# Firefighter Guidelines

## Database and Backups

DB credentials are populated into `db-backup` container env. In order to inspect or modify database, run:

```
PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$PGHOST" -p 5432 -U "$POSTGRES_USER" -d "$POSTGRES_DB"
```

Backup tools: are located in `/usr/local/bin`

- `backup-db.sh`
- `recover-db.sh`
