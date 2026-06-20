#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${ROOT}/internal/apps/agent/geoipdata/GeoLite2-Country.mmdb"
URL="${GEOIP_MMDB_URL:-https://raw.githubusercontent.com/Loyalsoldier/geoip/release/GeoLite2-Country.mmdb}"

mkdir -p "$(dirname "$TARGET")"
curl -fsSL -o "$TARGET" "$URL"

if [ ! -s "$TARGET" ]; then
  echo "downloaded GeoIP database is empty: $TARGET" >&2
  exit 1
fi

echo "GeoIP database ready: $TARGET"