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
echo "Container is ready for manual operations. Use 'docker exec' to run commands."
echo "Available commands:"
echo "  - Backup:  docker exec <container-name> /usr/local/bin/backup-db.sh"
echo "  - Recover: docker exec <container-name> /usr/local/bin/recover-db.sh"

# Keep container running (hang)
tail -f /dev/null

