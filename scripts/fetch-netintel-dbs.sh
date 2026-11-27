#!/usr/bin/env bash
# Download open network intelligence databases (GeoIP/ASN MMDB + bgp.tools routing/ASN tables)
# for packaging into Docker images, release archives, or initial local seeding.
# NOTE: CAIDA AS-Relationships datasets are NEVER bundled here due to redistribution restrictions.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${NETINTEL_OUT_DIR:-${ROOT}/dist/netintel}"
GEOIP_SOURCE="${NETINTEL_GEOIP_SOURCE:-fyralabs}"
BGPTOOLS_UA="${NETINTEL_BGPTOOLS_UA:-OpenFlare-NetIntel/1.0 (+https://github.com/Rain-kl/OpenFlare)}"
SKIP_BGPTOOLS="${NETINTEL_SKIP_BGPTOOLS:-false}"
INCLUDE_TAG_MEMBERS="${NETINTEL_INCLUDE_TAG_MEMBERS:-true}"

mkdir -p "${OUT_DIR}"

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk '{print $1}'
  else
    echo ""
  fi
}

file_size() {
  wc -c <"$1" | tr -d ' '
}

download_raw() {
  local url="$1"
  local dest="$2"
  local ua="${3:-}"
  local tmp="${dest}.tmp"
  local curl_args=(-fsSL --retry 3 --retry-delay 2 --connect-timeout 15 --max-time 300)
  if [ -n "$ua" ]; then
    curl_args+=(-A "$ua")
  fi
  if curl "${curl_args[@]}" -o "$tmp" "$url" && [ -s "$tmp" ]; then
    mv "$tmp" "$dest"
    return 0
  fi
  rm -f "$tmp"
  return 1
}

download_mmdb() {
  local name="$1"
  local url="$2"
  local dest="${OUT_DIR}/$3"
  if download_raw "$url" "$dest" ""; then
    echo "[netintel] ${name} downloaded -> ${dest} ($(file_size "$dest") bytes)"
    return 0
  fi
  if [ -s "$dest" ]; then
    echo "[netintel] ${name} download failed, keeping existing ${dest}" >&2
    return 0
  fi
  echo "[netintel] ERROR: ${name} failed from ${url}" >&2
  return 1
}

COUNTRY_FILE=""
COUNTRY_URL=""
COUNTRY_LICENSE=""
CITY_FILE=""
CITY_URL=""
CITY_LICENSE=""
ASN_FILE=""
ASN_URL=""
ASN_LICENSE=""

case "${GEOIP_SOURCE}" in
  fyralabs)
    COUNTRY_FILE="GeoLite2-Country.mmdb"
    COUNTRY_URL="${GEOIP_MMDB_URL:-https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-Country.mmdb}"
    COUNTRY_LICENSE="CC BY-SA 4.0 / MaxMind GeoLite2 EULA (via FyraLabs)"
    CITY_FILE="GeoLite2-City.mmdb"
    CITY_URL="${GEOIP_CITY_MMDB_URL:-https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-City.mmdb}"
    CITY_LICENSE="CC BY-SA 4.0 / MaxMind GeoLite2 EULA (via FyraLabs)"
    ASN_FILE="GeoLite2-ASN.mmdb"
    ASN_URL="${GEOIP_ASN_MMDB_URL:-https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-ASN.mmdb}"
    ASN_LICENSE="CC BY-SA 4.0 / MaxMind GeoLite2 EULA (via FyraLabs)"
    download_mmdb "GeoLite2-Country" "$COUNTRY_URL" "$COUNTRY_FILE"
    download_mmdb "GeoLite2-City" "$CITY_URL" "$CITY_FILE"
    download_mmdb "GeoLite2-ASN" "$ASN_URL" "$ASN_FILE"
    ;;
  sapics)
    COUNTRY_FILE="sapics-country.mmdb"
    COUNTRY_URL="https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb"
    COUNTRY_LICENSE="PDDL-1.0 (sapics/ip-location-db)"
    ASN_FILE="sapics-asn.mmdb"
    ASN_URL="https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb"
    ASN_LICENSE="PDDL-1.0 (sapics/ip-location-db)"
    download_mmdb "sapics-country" "$COUNTRY_URL" "$COUNTRY_FILE"
    download_mmdb "sapics-asn" "$ASN_URL" "$ASN_FILE"
    ;;
  *)
    echo "[netintel] unsupported NETINTEL_GEOIP_SOURCE: ${GEOIP_SOURCE} (expected fyralabs or sapics)" >&2
    exit 1
    ;;
esac

ASNS_FILE="bgptools-asns.csv"
ASNS_URL="https://bgp.tools/asns.csv"
TAGS_FILE="bgptools-tags.txt"
TAGS_URL="https://bgp.tools/tags.txt"
ASN_TAGS_FILE="bgptools-asn-tags.txt"
TABLE_FILE="bgptools-table.jsonl.gz"
TABLE_URL="https://bgp.tools/table.jsonl"

if [ "${SKIP_BGPTOOLS}" != "true" ]; then
  if download_raw "$ASNS_URL" "${OUT_DIR}/${ASNS_FILE}" "$BGPTOOLS_UA"; then
    echo "[netintel] bgp.tools asns.csv downloaded -> ${OUT_DIR}/${ASNS_FILE} ($(file_size "${OUT_DIR}/${ASNS_FILE}") bytes)"
  else
    echo "[netintel] ERROR: failed to download ${ASNS_URL}" >&2
    exit 1
  fi

  if download_raw "$TAGS_URL" "${OUT_DIR}/${TAGS_FILE}" "$BGPTOOLS_UA"; then
    echo "[netintel] bgp.tools tags.txt downloaded -> ${OUT_DIR}/${TAGS_FILE} ($(file_size "${OUT_DIR}/${TAGS_FILE}") bytes)"
  else
    echo "[netintel] ERROR: failed to download ${TAGS_URL}" >&2
    exit 1
  fi

  if [ "${INCLUDE_TAG_MEMBERS}" = "true" ]; then
    tmp_asn_tags="${OUT_DIR}/${ASN_TAGS_FILE}.tmp"
    : > "${tmp_asn_tags}"
    while IFS=, read -r tag_name tag_count || [ -n "$tag_name" ]; do
      tag_name="$(printf '%s' "$tag_name" | tr -d '\r' | xargs)"
      tag_count="$(printf '%s' "${tag_count:-0}" | tr -d '\r' | xargs)"
      [ -z "$tag_name" ] && continue
      if [ "${tag_count:-0}" = "0" ]; then
        continue
      fi
      tag_tmp="${OUT_DIR}/.tag_${tag_name}.tmp"
      if download_raw "https://bgp.tools/tags/${tag_name}.txt" "$tag_tmp" "$BGPTOOLS_UA"; then
        asns_joined="$(tr -d '\r' < "$tag_tmp" | grep -E '^AS[0-9]+' | paste -sd, - || true)"
        if [ -n "$asns_joined" ]; then
          printf '%s %s\n' "$tag_name" "$asns_joined" >> "${tmp_asn_tags}"
        fi
        rm -f "$tag_tmp"
      else
        echo "[netintel] WARN: skipped tag ${tag_name}" >&2
      fi
      sleep 0.1
    done < "${OUT_DIR}/${TAGS_FILE}"
    mv "${tmp_asn_tags}" "${OUT_DIR}/${ASN_TAGS_FILE}"
    echo "[netintel] bgp.tools tag memberships built -> ${OUT_DIR}/${ASN_TAGS_FILE} ($(file_size "${OUT_DIR}/${ASN_TAGS_FILE}") bytes)"
  fi

  tmp_table_gz="${OUT_DIR}/${TABLE_FILE}.tmp"
  if curl -fsSL --retry 3 --retry-delay 2 --connect-timeout 15 --max-time 600 -A "$BGPTOOLS_UA" "$TABLE_URL" | gzip -9 > "$tmp_table_gz" && gzip -t "$tmp_table_gz" 2>/dev/null; then
    mv "$tmp_table_gz" "${OUT_DIR}/${TABLE_FILE}"
    echo "[netintel] bgp.tools table.jsonl.gz downloaded -> ${OUT_DIR}/${TABLE_FILE} ($(file_size "${OUT_DIR}/${TABLE_FILE}") bytes)"
  else
    rm -f "$tmp_table_gz"
    echo "[netintel] ERROR: failed to download ${TABLE_URL}" >&2
    exit 1
  fi
fi

now_unix="$(date -u +%s)"
manifest_tmp="${OUT_DIR}/manifest.json.tmp"
{
  printf '{\n  "generated_at_unix": %s,\n  "geoip_source": "%s",\n  "datasets": {\n' "$now_unix" "$GEOIP_SOURCE"
  first=true
  emit_entry() {
    local key="$1"
    local filename="$2"
    local source_url="$3"
    local license="$4"
    if [ -z "$filename" ] || [ ! -s "${OUT_DIR}/${filename}" ]; then
      return 0
    fi
    local sha size
    sha="$(sha256_file "${OUT_DIR}/${filename}")"
    size="$(file_size "${OUT_DIR}/${filename}")"
    if [ "$first" = "true" ]; then
      first=false
    else
      printf ',\n'
    fi
    printf '    "%s": {\n      "filename": "%s",\n      "source_url": "%s",\n      "license": "%s",\n      "sha256": "%s",\n      "size_bytes": %s,\n      "fetched_at_unix": %s\n    }' \
      "$key" "$filename" "$source_url" "$license" "$sha" "$size" "$now_unix"
  }
  emit_entry "geoip_country" "$COUNTRY_FILE" "$COUNTRY_URL" "$COUNTRY_LICENSE"
  emit_entry "geoip_city" "$CITY_FILE" "$CITY_URL" "$CITY_LICENSE"
  emit_entry "geoip_asn" "$ASN_FILE" "$ASN_URL" "$ASN_LICENSE"
  if [ "${SKIP_BGPTOOLS}" != "true" ]; then
    emit_entry "bgptools_asns" "$ASNS_FILE" "$ASNS_URL" "bgp.tools (Attribution Required)"
    emit_entry "bgptools_tags" "$TAGS_FILE" "$TAGS_URL" "bgp.tools (Attribution Required)"
    emit_entry "bgptools_asn_tags" "$ASN_TAGS_FILE" "https://bgp.tools/tags/*.txt" "bgp.tools (Attribution Required)"
    emit_entry "bgptools_table" "$TABLE_FILE" "$TABLE_URL" "bgp.tools (Attribution Required)"
  fi
  printf '\n  }\n}\n'
} > "${manifest_tmp}"
mv "${manifest_tmp}" "${OUT_DIR}/manifest.json"
echo "[netintel] manifest written -> ${OUT_DIR}/manifest.json"
