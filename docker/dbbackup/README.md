# Database Backup & Recovery

Automated PostgreSQL database backup system with Google Drive sync using rclone.

## Features

- **Automated backups** every 3 minutes via cron
- **Google Drive sync** using rclone service account
- **Automatic restore** on container startup
- **Manual recovery** from latest backup
- **Docker logs integration** for monitoring

## Prerequisites

1. Google Drive service account with access to target folder
2. rclone configuration file (`test/rclone.conf`)
3. Service account JSON key (`test/rclone-service-account.json`)

> **⚠️ Important Note on Google Drive Service Accounts:**
>
> Service accounts have **zero storage quota** on personal Google Drive accounts (@gmail.com). You will encounter `403: storageQuotaExceeded` errors when trying to upload files to a personal drive folder.
>
> **Solutions:**
>
> - **Use Google Workspace Shared Drive** (Team Drive) - Recommended for production
> - **Use OAuth delegation** instead of service account authentication
>
> See: [Google Drive API Limits](https://developers.google.com/drive/api/guides/limits) | [StackOverflow Discussion](https://stackoverflow.com/questions/79700077)

## Configuration

Update `test/rclone.conf`:

```ini
[cim-backup]
type = drive
service_account_file = /root/.config/rclone/rclone-service-account.json
root_folder_id = YOUR_GOOGLE_DRIVE_FOLDER_ID
```

Update `docker-compose.yml` environment variables:

```yaml
environment:
  - POSTGRES_DB=your_database
  - POSTGRES_USER=your_user
  - POSTGRES_PASSWORD=your_password
  - PGHOST=host.docker.internal
  - RCLONE_REMOTE=cim-backup # Remote name from rclone.conf
  - RCLONE_PATH= # Optional: subfolder path (e.g., "dev", "prod", "backups/db")
```

## Usage

### Production Mode (Automatic Backups)

Start the service with cron-based automatic backups:

```bash
cd docker/dbbackup
docker-compose up -d dbbackup
```

Monitor logs:

```bash
docker logs -f dbbackup-test
```

### Manual Mode (Testing)

Start container without cron for manual operations:

```bash
cd docker/dbbackup
docker-compose up -d dbbackup-manual
```

## Backup Operations

### Manual Backup

Create a backup and sync to Google Drive:

```bash
docker exec dbbackup-test /usr/local/bin/backup-db.sh
```

Or with manual container:

```bash
docker exec dbbackup-manual /usr/local/bin/backup-db.sh
```

### View Backup Files

```bash
docker exec dbbackup-test ls -lh /backup/dumps
```

## Recovery Operations

### Restore from Latest Backup

Restore database from the most recent backup:

```bash
docker exec dbbackup-test /usr/local/bin/recover-db.sh
```

**Warning:** This will overwrite the existing database!

### Check Available Backups

```bash
docker exec dbbackup-test ls -lh /backup/dumps
```

Backup files are named: `backup_<database>_<timestamp>.sql.gz`

## Monitoring

### View Logs

```bash
# Follow logs in real-time
docker logs -f dbbackup-test

# View last 50 lines
docker logs --tail 50 dbbackup-test
```

### Check Backup Status

```bash
# View backup log
docker exec dbbackup-test cat /backup/logs/backup.log

# List backup files with timestamps
docker exec dbbackup-test ls -lht /backup/dumps
```

## How It Works

1. **Container Startup**: Syncs existing backups from Google Drive to local storage
2. **Cron Schedule**: Runs backup every 3 minutes (configurable in `build/default-crontab`)
3. **Backup Process**:
   - Creates PostgreSQL dump
   - Compresses with gzip
   - Syncs to Google Drive
   - Cleans up local backups older than 7 days
4. **Recovery**: Finds latest backup file and restores to database

## Available Scripts

- `/usr/local/bin/backup-db.sh` - Create backup and sync to Google Drive
- `/usr/local/bin/recover-db.sh` - Restore from latest backup
- `/entrypoint.sh` - Production entrypoint (with cron)
- `/entrypoint-manual.sh` - Manual testing entrypoint (no cron)

## Environment Variables

| Variable            | Default                | Description                                               |
| ------------------- | ---------------------- | --------------------------------------------------------- |
| `POSTGRES_DB`       | (required)             | Database name                                             |
| `POSTGRES_USER`     | (required)             | Database user                                             |
| `POSTGRES_PASSWORD` | (required)             | Database password                                         |
| `PGHOST`            | `host.docker.internal` | Database host                                             |
| `RCLONE_REMOTE`     | `cim-backup`           | Rclone remote name from config (without colon)            |
| `RCLONE_PATH`       | (empty)                | Optional path within remote (e.g., `dev`, `prod/backups`) |

**Note:** `BACKUP_DIR` and `LOG_DIR` are set at build time in the Dockerfile and should not be overridden at runtime.

## Troubleshooting

### Container won't start

Check if Google Drive restore failed:

```bash
docker logs dbbackup-test
```

If restore fails, the container exits to prevent data loss.

### No backups in container

Backups are restored from Google Drive on startup. Check:

- rclone configuration is correct
- Service account has access to the folder
- Google Drive folder ID is valid

### Recovery fails

Ensure backup files exist:

```bash
docker exec dbbackup-test ls -lh /backup/dumps
```

If no files, run a manual backup first.

## Docker Compose Services

- **`dbbackup`** - Production service with automatic cron backups
- **`dbbackup-manual`** - Manual service for testing (no cron)

Both services share the same volumes for backup files.
