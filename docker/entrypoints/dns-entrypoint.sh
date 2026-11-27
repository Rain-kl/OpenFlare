#!/bin/sh
set -eu

if [ "$#" -gt 0 ]; then
  exec openflare-dns "$@"
fi

controller_url="${OPENFLARE_DNS_CONTROLLER_URL:-${OPENFLARE_SERVER_URL:-}}"
if [ -z "$controller_url" ]; then
  printf '%s\n' 'OPENFLARE_DNS_CONTROLLER_URL is required' >&2
  exit 1
fi

cache_dir="${OPENFLARE_DNS_CACHE_DIR:-/var/lib/openflare-dns}"
geoip_dir="${OPENFLARE_DNS_GEOIP_DIR:-${cache_dir}/geoip}"
listen_addr="${OPENFLARE_DNS_LISTEN:-0.0.0.0:5353}"

umask 077
mkdir -p "$cache_dir" "$geoip_dir"
rm -f "$cache_dir/ready.json"

has_inline=0
has_disc=0
has_file=0
[ -n "${OPENFLARE_DNS_TOKEN:-}" ] && has_inline=1
[ -n "${OPENFLARE_DNS_DISCOVERY_TOKEN:-}" ] && has_disc=1
[ -n "${OPENFLARE_DNS_TOKEN_FILE:-}" ] && has_file=1

if [ $((has_inline + has_disc + has_file)) -gt 1 ]; then
  printf '%s\n' 'Specify only one of OPENFLARE_DNS_TOKEN, OPENFLARE_DNS_DISCOVERY_TOKEN, or OPENFLARE_DNS_TOKEN_FILE.' >&2
  exit 1
fi

token_file="${OPENFLARE_DNS_TOKEN_FILE:-/run/secrets/openflare-dns-token}"
token_type="${OPENFLARE_DNS_TOKEN_TYPE:-discovery}"

write_bootstrap_token() {
  tmp_file="$(mktemp "$cache_dir/.bootstrap-token.XXXXXX")"
  printf '%s' "$1" > "$tmp_file"
  mv -f "$tmp_file" "$cache_dir/bootstrap-token"
  token_file="$cache_dir/bootstrap-token"
}

if [ "$has_inline" -eq 1 ]; then
  write_bootstrap_token "$OPENFLARE_DNS_TOKEN"
  token_type="${OPENFLARE_DNS_TOKEN_TYPE:-instance}"
  unset OPENFLARE_DNS_TOKEN
elif [ "$has_disc" -eq 1 ]; then
  write_bootstrap_token "$OPENFLARE_DNS_DISCOVERY_TOKEN"
  token_type="discovery"
  unset OPENFLARE_DNS_DISCOVERY_TOKEN
fi

if [ "$token_type" = "access" ]; then
  token_type="instance"
fi

set -- serve \
  --controller-url "$controller_url" \
  --token-type "$token_type" \
  --token-file "$token_file" \
  --listen "$listen_addr" \
  --cache-dir "$cache_dir" \
  --geoip-dir "$geoip_dir"

if [ -n "${OPENFLARE_DNS_INSTANCE_ID:-}" ]; then
  set -- "$@" --instance-id "$OPENFLARE_DNS_INSTANCE_ID"
fi
if [ -n "${OPENFLARE_DNS_NAME:-}" ]; then
  set -- "$@" --name "$OPENFLARE_DNS_NAME"
fi
if [ -n "${OPENFLARE_DNS_PUBLIC_ADDR:-}" ]; then
  set -- "$@" --public-addr "$OPENFLARE_DNS_PUBLIC_ADDR"
fi
if [ -n "${OPENFLARE_DNS_WORKERS:-}" ]; then
  set -- "$@" --workers "$OPENFLARE_DNS_WORKERS"
fi
if [ -n "${OPENFLARE_DNS_RRL_QPS:-}" ]; then
  set -- "$@" --rrl-qps "$OPENFLARE_DNS_RRL_QPS"
fi
if [ -n "${OPENFLARE_DNS_RRL_BURST:-}" ]; then
  set -- "$@" --rrl-burst "$OPENFLARE_DNS_RRL_BURST"
fi
if [ -n "${OPENFLARE_DNS_SNAPSHOT_INTERVAL_SECS:-}" ]; then
  set -- "$@" --snapshot-interval-secs "$OPENFLARE_DNS_SNAPSHOT_INTERVAL_SECS"
fi

exec openflare-dns "$@"
