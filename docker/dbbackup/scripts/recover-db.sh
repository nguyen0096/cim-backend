#!/bin/bash
# Database recovery script - restores from a specified backup file

set -e

# Load shared environment configuration
source /usr/local/bin/env-config.sh

# Check if backup file path is provided
if [ -z "$1" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Backup file path is required"
    echo "Usage: $0 <backup_file_path>"
    echo "Example: $0 /backup/dumps/backup_cim-db_20251121_000002.sql.gz"
    exit 1
fi

BACKUP_FILE="$1"

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Starting database recovery..."
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Backup file: ${BACKUP_FILE}"

# Verify the backup file exists and is readable
if [ ! -f "$BACKUP_FILE" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Backup file does not exist: ${BACKUP_FILE}"
    exit 1
fi

if [ ! -r "$BACKUP_FILE" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Backup file is not readable: ${BACKUP_FILE}"
    exit 1
fi

# Warning and confirmation
echo "[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: This will overwrite the existing database!"
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Database: ${DB_NAME} on ${DB_HOST}"
read -p "Are you sure you want to continue? (yes/no): " CONFIRMATION

if [ "$CONFIRMATION" != "yes" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Recovery cancelled by user"
    exit 0
fi

# Restore the database
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Restoring database from backup..."

# Decompress and restore
gunzip -c "${BACKUP_FILE}" | PGPASSWORD="${DB_PASSWORD}" psql \
    -h "${DB_HOST}" \
    -U "${DB_USER}" \
    -d "${DB_NAME}" \
    -q

if [ $? -eq 0 ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Database recovery completed successfully from: ${BACKUP_FILE}"
else
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Database recovery failed"
    exit 1
fi

