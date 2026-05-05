#!/usr/bin/env bash
set -euo pipefail

BACKUP_DIR="/var/lib/lankeeper/backups"
CONFIG_DIR="/etc/lankeeper"
DATE=$(date +%Y%m%d-%H%M%S)
BACKUP_FILE="$BACKUP_DIR/lankeeper-backup-$DATE.tar.gz"

mkdir -p "$BACKUP_DIR"

tar czf "$BACKUP_FILE" \
    -C / \
    etc/lankeeper \
    etc/unbound \
    etc/dnsmasq.d \
    etc/chrony \
    etc/samba \
    etc/wireguard \
    etc/openvpn \
    2>/dev/null || true

KEEP_DAYS=${1:-30}
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +"$KEEP_DAYS" -delete

echo "Backup created: $BACKUP_FILE"
