#!/bin/sh
set -eu

if [ "$#" -gt 0 ]; then
  exec openflare-edge "$@"
fi

: "${OPENFLARE_EDGE_SERVER_URL:?OPENFLARE_EDGE_SERVER_URL is required}"
state_dir="${OPENFLARE_EDGE_STATE_DIR:-/var/lib/openflare-edge}"
token_file="${OPENFLARE_EDGE_TOKEN_FILE:-/run/secrets/openflare-edge-token}"
tls_listen="${OPENFLARE_EDGE_TLS_LISTEN:-0.0.0.0:8443}"

umask 077
mkdir -p "$state_dir"
rm -f "$state_dir/ready.json"
if [ -n "${OPENFLARE_EDGE_TOKEN:-}" ]; then
  if [ -n "${OPENFLARE_EDGE_TOKEN_FILE:-}" ]; then
    printf '%s\n' 'Specify either OPENFLARE_EDGE_TOKEN or OPENFLARE_EDGE_TOKEN_FILE.' >&2
    exit 1
  fi
  token_file="$(mktemp "$state_dir/.bootstrap-token.XXXXXX")"
  printf '%s\n' "$OPENFLARE_EDGE_TOKEN" > "$token_file"
  mv -f "$token_file" "$state_dir/bootstrap-token"
  token_file="$state_dir/bootstrap-token"
  unset OPENFLARE_EDGE_TOKEN
fi

set -- run --server-url "$OPENFLARE_EDGE_SERVER_URL" \
  --state-dir "$state_dir" --token-type "${OPENFLARE_EDGE_TOKEN_TYPE:-discovery}" \
  --token-file "$token_file" --http-listen "${OPENFLARE_EDGE_HTTP_LISTEN:-0.0.0.0:8080}"
if [ "$tls_listen" != none ]; then
  set -- "$@" --tls-listen "$tls_listen"
fi
exec openflare-edge "$@"
