#!/bin/bash
#
# Daily SQLite snapshot for LLM Wiki.
# Installed at /etc/cron.daily/learn-helper-backup on the server.
# Uses sqlite3 .backup (ACID-safe) and keeps 7 days of snapshots.
#
set -euo pipefail

APP_DIR=/opt/learn-helper
DB_PATH="$APP_DIR/learn-helper.db"
BACKUP_DIR="$APP_DIR/backups"
TS=$(date +%Y-%m-%d-%H%M)
RETENTION_DAYS=7

if [[ ! -f "$DB_PATH" ]]; then
    echo "Backup skipped: $DB_PATH not found" >&2
    exit 0
fi

mkdir -p "$BACKUP_DIR"

# Checkpoint WAL so the backup is a single file (no -wal companion).
sudo -u learnhelper sqlite3 "$DB_PATH" "PRAGMA wal_checkpoint(TRUNCATE);" >/dev/null

# Take the snapshot.
sudo -u learnhelper sqlite3 "$DB_PATH" ".backup '$BACKUP_DIR/lh-$TS.db'"

# Refresh the "latest" symlink for one-command restore.
ln -sfn "$BACKUP_DIR/lh-$TS.db" "$BACKUP_DIR/lh-latest.db"

# Retention: delete snapshots older than $RETENTION_DAYS.
find "$BACKUP_DIR" -name 'lh-*.db' -mtime +$RETENTION_DAYS -delete

echo "Backup OK: $BACKUP_DIR/lh-$TS.db"
