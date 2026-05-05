#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="/etc/lankeeper"
DEFAULTS_DIR="/opt/lankeeper/configs/defaults"

echo "WARNING: This will reset ALL configuration to factory defaults."
read -rp "Continue? [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "Aborted."
    exit 0
fi

if [[ ! -d "$DEFAULTS_DIR" ]]; then
    echo "ERROR: Defaults directory not found: $DEFAULTS_DIR"
    exit 1
fi

cp "$DEFAULTS_DIR"/*.yaml "$CONFIG_DIR/"
echo "Configuration reset to factory defaults."

systemctl restart lankeeper.target
echo "Services restarted."
