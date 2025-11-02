#!/bin/bash
# Database backup and Google Drive sync script
# This script creates a database dump and syncs only the new file to Google Drive

set -e

# Configuration
BACKUP_DIR="/backup/dumps"
DB_HOST="${PGHOST:-host.docker.internal}"
DB_NAME="${POSTGRES_DB}"
DB_USER="${POSTGRES_USER}"
BACKUP_DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/backup_${DB_NAME}_${BACKUP_DATE}.sql"

# Create backup directory if it doesn't exist
mkdir -p "${BACKUP_DIR}"

# Log backup start
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Starting database backup..."

# Create database dump
PGPASSWORD="${POSTGRES_PASSWORD}" pg_dump \
    -h "${DB_HOST}" \
    -U "${DB_USER}" \
    -d "${DB_NAME}" \
    -F p \
    -f "${BACKUP_FILE}"

# Compress the backup
gzip "${BACKUP_FILE}"
BACKUP_FILE_GZ="${BACKUP_FILE}.gz"

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Backup completed: ${BACKUP_FILE_GZ}"

# Sync all backup files to Google Drive
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Starting rclone sync to Google Drive..."

rclone sync "${BACKUP_DIR}/" cim-backup: \
    --config /root/.config/rclone/rclone.conf \
    --log-level INFO \
    --stats 1m

if [ $? -eq 0 ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Rclone sync completed successfully"
else
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Rclone sync failed with exit code: $?"
fi

# Clean up old backups (keep last 7 days)
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Cleaning up old backups..."
find "${BACKUP_DIR}" -name "backup_*.sql.gz" -type f -mtime +7 -delete

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Backup process completed"

