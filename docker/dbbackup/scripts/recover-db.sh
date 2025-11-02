#!/bin/bash
# Database recovery script - restores from the latest backup file

set -e

# Load shared environment configuration
source /usr/local/bin/env-config.sh

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Starting database recovery..."

# Find the latest backup file (Alpine-compatible)
LATEST_BACKUP=$(ls -t "${BACKUP_DIR}"/backup_*.sql.gz 2>/dev/null | head -1)

if [ -z "$LATEST_BACKUP" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: No backup files found in ${BACKUP_DIR}"
    exit 1
fi

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Found latest backup: ${LATEST_BACKUP}"

# Verify the backup file exists and is readable
if [ ! -r "$LATEST_BACKUP" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Backup file is not readable: ${LATEST_BACKUP}"
    exit 1
fi

# Restore the database
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Restoring database from backup..."
echo "[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: This will overwrite the existing database!"

# Decompress and restore
gunzip -c "${LATEST_BACKUP}" | PGPASSWORD="${DB_PASSWORD}" psql \
    -h "${DB_HOST}" \
    -U "${DB_USER}" \
    -d "${DB_NAME}" \
    -q

if [ $? -eq 0 ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Database recovery completed successfully from: ${LATEST_BACKUP}"
else
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: Database recovery failed"
    exit 1
fi

