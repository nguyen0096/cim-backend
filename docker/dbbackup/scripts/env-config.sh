#!/bin/bash
# Shared environment configuration
# Source this file in other scripts: source /usr/local/bin/env-config.sh

# Rclone configuration
export RCLONE_REMOTE="${RCLONE_REMOTE:-cim-backup}"
export RCLONE_PATH="${RCLONE_PATH:-}"

# Database configuration
export DB_HOST="${PGHOST:-host.docker.internal}"
export DB_NAME="${POSTGRES_DB}"
export DB_USER="${POSTGRES_USER}"
export DB_PASSWORD="${POSTGRES_PASSWORD}"

