FROM alpine:3.18

# Install rclone, cron, and bash
RUN apk update && \
    apk add --no-cache \
    rclone \
    dcron \
    bash \
    postgresql-client \
    && rm -rf /var/cache/apk/*

# Create directories for volumes
RUN mkdir -p /backup/dumps \
    /backup/config \
    /backup/logs \
    /root/.config/rclone

# Create cron log file
RUN touch /var/log/cron.log

# Set container path environment variables
ENV BACKUP_DIR=/backup/dumps \
    LOG_DIR=/backup/logs

# Copy build files
COPY scripts/env-config.sh /usr/local/bin/env-config.sh
COPY scripts/backup-db.sh /usr/local/bin/backup-db.sh
COPY scripts/recover-db.sh /usr/local/bin/recover-db.sh
COPY scripts/entrypoint.sh /entrypoint.sh
COPY scripts/entrypoint-manual.sh /entrypoint-manual.sh

# Set proper permissions
RUN chmod 700 /root/.config/rclone && \
    chmod +x /usr/local/bin/env-config.sh && \
    chmod +x /usr/local/bin/backup-db.sh && \
    chmod +x /usr/local/bin/recover-db.sh && \
    chmod +x /entrypoint.sh && \
    chmod +x /entrypoint-manual.sh

VOLUME ["/backup/dumps", "/backup/config", "/backup/logs", "/root/.config/rclone"]

WORKDIR /backup

ENTRYPOINT ["/entrypoint.sh"]

