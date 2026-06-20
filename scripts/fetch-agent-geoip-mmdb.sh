#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${ROOT}/internal/apps/agent/geoipdata/GeoLite2-Country.mmdb"
URL="${GEOIP_MMDB_URL:-https://raw.githubusercontent.com/Loyalsoldier/geoip/release/GeoLite2-Country.mmdb}"

mkdir -p "$(dirname "$TARGET")"

if curl -fsSL -o "${TARGET}.tmp" "$URL"; then
  mv "${TARGET}.tmp" "$TARGET"
  echo "GeoIP database downloaded: $TARGET"
elif [ -s "$TARGET" ]; then
  rm -f "${TARGET}.tmp"
  echo "GeoIP download failed, using committed database: $TARGET"
else
  rm -f "${TARGET}.tmp"
  echo "GeoIP database missing and download failed: $URL" >&2
  exit 1
fi

if [ ! -s "$TARGET" ]; then
  echo "GeoIP database is empty: $TARGET" >&2
  exit 1
fi