#!/bin/bash
set -e

# Load shared environment configuration
source /usr/local/bin/env-config.sh

# Restore existing backups from Google Drive on container startup
echo "Restoring existing backups from Google Drive..."
rclone sync "${RCLONE_REMOTE}:${RCLONE_PATH}" "${BACKUP_DIR}/" \
    --config /root/.config/rclone/rclone.conf \
    --log-level INFO \
    --stats 1m

if [ $? -ne 0 ]; then
    echo "ERROR: Backup restore failed! Exiting to prevent data loss on Google Drive."
    exit 1
fi

echo "Backup restore completed successfully"

# Set default cron schedule if not provided (every 3 hours)
: "${BACKUP_CRON_SCHEDULE:=0 */3 * * *}"

# Load crontab if exists in config volume, otherwise generate from environment variable
if [ -f /backup/config/crontab ]; then
    echo "Loading custom crontab from /backup/config/crontab..."
    crontab /backup/config/crontab
else
    echo "Generating crontab with schedule: ${BACKUP_CRON_SCHEDULE}"
    echo "${BACKUP_CRON_SCHEDULE} /usr/local/bin/backup-db.sh >> ${LOG_DIR}/backup.log 2>&1" | crontab -
fi

# Tail log files to docker logs (in background)
echo "Starting log monitoring..."
mkdir -p "${LOG_DIR}"
touch "${LOG_DIR}/backup.log"
tail -F "${LOG_DIR}/backup.log" &

# Start cron in foreground
echo "Starting cron daemon..."
crond -f -l 2

