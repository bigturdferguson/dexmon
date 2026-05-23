#!/bin/sh
set -e

if [ -z "${CONFIG_TOML:-}" ]; then
    echo "ERROR: CONFIG_TOML environment variable is required" >&2
    exit 1
fi

mkdir -p /data
printf '%s' "$CONFIG_TOML" | base64 -d > /data/config.toml

exec /app/dexmon -config /data/config.toml -db /data/dexmon.db
