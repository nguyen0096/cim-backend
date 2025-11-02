#!/bin/bash
set -e

# Restore existing backups from Google Drive on container startup
echo "Restoring existing backups from Google Drive..."
rclone sync cim-backup: /backup/dumps/ \
    --config /root/.config/rclone/rclone.conf \
    --log-level INFO \
    --stats 1m

if [ $? -ne 0 ]; then
    echo "ERROR: Backup restore failed! Exiting to prevent data loss on Google Drive."
    exit 1
fi

echo "Backup restore completed successfully"

# Load crontab if exists in config volume
if [ -f /backup/config/crontab ]; then
    echo "Loading custom crontab..."
    crontab /backup/config/crontab
fi

# Tail log files to docker logs (in background)
echo "Starting log monitoring..."
touch /backup/logs/backup.log
tail -F /backup/logs/backup.log &

# Start cron in foreground
echo "Starting cron daemon..."
crond -f -l 2

