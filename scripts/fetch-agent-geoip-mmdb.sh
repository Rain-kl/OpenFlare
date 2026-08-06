#!/usr/bin/env bash
# Download MaxMind GeoLite2 Country/City databases for packaging (Docker image COPY),
# not for go:embed into the agent binary.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${GEOIP_OUT_DIR:-${ROOT}/dist/geoip}"
COUNTRY_URL="${GEOIP_MMDB_URL:-https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-Country.mmdb}"
CITY_URL="${GEOIP_CITY_MMDB_URL:-https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-City.mmdb}"

mkdir -p "${OUT_DIR}"

download_one() {
  local url="$1"
  local dest="$2"
  local name="$3"
  if curl -fsSL -o "${dest}.tmp" "$url" && [ -s "${dest}.tmp" ]; then
    mv "${dest}.tmp" "$dest"
    echo "GeoIP ${name} downloaded: $dest ($(wc -c <"$dest" | tr -d ' ') bytes)"
    return 0
  fi
  rm -f "${dest}.tmp"
  if [ -s "$dest" ]; then
    echo "GeoIP ${name} download failed, keeping existing: $dest" >&2
    return 0
  fi
  echo "GeoIP ${name} missing and download failed: $url" >&2
  return 1
}

download_one "$COUNTRY_URL" "${OUT_DIR}/GeoLite2-Country.mmdb" "Country"
download_one "$CITY_URL" "${OUT_DIR}/GeoLite2-City.mmdb" "City"
