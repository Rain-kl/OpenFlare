#!/usr/bin/env bash
set -euo pipefail

VERSION=""
SERVER_URL=""
TOKEN_TYPE="discovery"
TOKEN=""
TOKEN_FILE=""
STATE_DIR="/var/lib/openflare-edge"
INSTALL_DIR="/opt/openflare-edge"
HTTP_LISTEN="0.0.0.0:80"
TLS_LISTEN="0.0.0.0:443"
REPO="Rain-kl/OpenFlare"
CREATE_SERVICE=true

log() { printf '[openflare-edge] %s\n' "$*"; }
die() { printf '[openflare-edge] ERROR: %s\n' "$*" >&2; exit 1; }
is_release_tag() { [[ "$1" =~ ^edge-v[0-9]+(\.[0-9]+){2}(-[0-9A-Za-z.-]+)?$ ]]; }
usage() {
  cat <<'EOF'
OpenFlare Edge Installer

Usage:
  install-openflare-edge.sh [OPTIONS]

Required:
  --server-url URL               OpenFlare Server URL
  --token TOKEN                  Discovery or node token
  --token-file PATH              Read token from PATH (alternative to --token)

Options:
  --token-type TYPE              discovery or node (default: discovery)
  --version TAG                  Install a release tag; default is latest
  --repo OWNER/NAME              Release repository (default: Rain-kl/OpenFlare)
  --install-dir DIR              Default: /opt/openflare-edge
  --state-dir DIR                Default: /var/lib/openflare-edge
  --http-listen ADDR             Default: 0.0.0.0:80
  --tls-listen ADDR              Default: 0.0.0.0:443; use "none" to disable TLS
  --no-service                   Do not create or restart systemd service
  -h, --help                     Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-url) SERVER_URL="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    --token-file) TOKEN_FILE="$2"; shift 2 ;;
    --token-type) TOKEN_TYPE="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --state-dir) STATE_DIR="$2"; shift 2 ;;
    --http-listen) HTTP_LISTEN="$2"; shift 2 ;;
    --tls-listen) TLS_LISTEN="$2"; shift 2 ;;
    --no-service) CREATE_SERVICE=false; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ -n "$SERVER_URL" ]] || die "--server-url is required"
[[ -n "$TOKEN" || -n "$TOKEN_FILE" ]] || die "--token or --token-file is required"
[[ "$TOKEN_TYPE" == "discovery" || "$TOKEN_TYPE" == "node" ]] || die "--token-type must be discovery or node"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported architecture: $ARCH" ;;
esac
[[ "$OS" == "linux" ]] || die "this installer supports Linux only"

if [[ -n "$TOKEN_FILE" ]]; then
  [[ -r "$TOKEN_FILE" ]] || die "token file is not readable: $TOKEN_FILE"
  TOKEN="$(tr -d '[:space:]' < "$TOKEN_FILE")"
fi
[[ -n "$TOKEN" ]] || die "token is empty"

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar >/dev/null 2>&1 || die "tar is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"
command -v install >/dev/null 2>&1 || die "install is required"
command -v flock >/dev/null 2>&1 || die "flock is required"
SYSTEMCTL_AVAILABLE=false
if command -v systemctl >/dev/null 2>&1; then SYSTEMCTL_AVAILABLE=true; fi
if [[ "$CREATE_SERVICE" == true && "$SYSTEMCTL_AVAILABLE" != true ]]; then
  die "--no-service is required when systemd is unavailable"
fi

if [[ -z "$VERSION" ]]; then
  log "resolving latest stable Edge release from ${REPO}"
  for ((page = 1; ; page++)); do
    releases_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=100&page=${page}")"
    mapfile -t release_tags < <(printf '%s' "$releases_json" | sed -nE 's/^[[:space:]]*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p')
    VERSION="$(printf '%s\n' "${release_tags[@]}" | grep -E '^edge-v[0-9]+(\.[0-9]+){2}$' | sort -Vr | sed -n '1p' || true)"
    if [[ -n "$VERSION" || ${#release_tags[@]} -lt 100 ]]; then break; fi
  done
  [[ -n "$VERSION" ]] || die "no stable Edge release found in ${REPO}"
fi
is_release_tag "$VERSION" || die "invalid Edge release version"
RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}")"
VERSION="$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
is_release_tag "$VERSION" || die "invalid Edge release version"
ASSET="openflare-edge_${VERSION}_linux_${ARCH}.tar.gz"
DOWNLOAD_URL="$(printf '%s' "$RELEASE_JSON" | grep '"browser_download_url"' | grep "/${ASSET}\"" | sed -E 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' | head -n1)"
[[ -n "$DOWNLOAD_URL" ]] || die "release asset not found: $ASSET"

TMP_DIR="$(mktemp -d "/tmp/openflare-edge.XXXXXX")"
STAGED_BIN=""
cleanup() {
  if [[ -n "$STAGED_BIN" ]]; then rm -f -- "$STAGED_BIN"; fi
  rm -rf -- "$TMP_DIR"
}
trap cleanup EXIT
log "downloading ${ASSET}"
curl -fsSL -o "$TMP_DIR/$ASSET" "$DOWNLOAD_URL"
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
PACKAGE_DIR="$(find "$TMP_DIR" -mindepth 1 -maxdepth 1 -type d -name "openflare-edge_${VERSION}_linux_${ARCH}" | head -n1)"
[[ -n "$PACKAGE_DIR" ]] || die "release archive layout is invalid"
[[ -x "$PACKAGE_DIR/openflare-edge" ]] || die "release binary is not executable"
(cd "$PACKAGE_DIR" && sha256sum -c SHA256SUMS)
[[ "$("$PACKAGE_DIR/openflare-edge" config-sync-version 2>/dev/null || true)" == 1 ]] ||
  die "${VERSION} does not support configuration sync protocol v1"

mkdir -p "$INSTALL_DIR" "$STATE_DIR"
exec 9>"$INSTALL_DIR/.install.lock"
flock -n 9 || die "another Edge installation or update holds $INSTALL_DIR/.install.lock"
STAGED_BIN="$INSTALL_DIR/.openflare-edge.new"
if [[ -x "$PACKAGE_DIR/scripts/fetch-agent-geoip-mmdb.sh" ]]; then
  log "refreshing GeoIP databases"
  GEOIP_OUT_DIR="$STATE_DIR" "$PACKAGE_DIR/scripts/fetch-agent-geoip-mmdb.sh" 9>&- || die "failed to download GeoIP databases"
fi
ensure_runtime_user() {
  if id openflare-edge >/dev/null 2>&1; then return; fi
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --home-dir "$STATE_DIR" --shell /usr/sbin/nologin openflare-edge
  else
    die "useradd is required to create the openflare-edge service account"
  fi
}
if [[ "$CREATE_SERVICE" == true ]]; then ensure_runtime_user; fi

INSTALL_TOKEN_FILE="${STATE_DIR}/bootstrap-token"
if [[ -n "$TOKEN_FILE" ]]; then
  if [[ "$TOKEN_FILE" -ef "$INSTALL_TOKEN_FILE" ]]; then
    chmod 600 "$INSTALL_TOKEN_FILE"
  else
    install -m 0600 "$TOKEN_FILE" "$INSTALL_TOKEN_FILE"
  fi
else
  printf '%s' "$TOKEN" > "$INSTALL_TOKEN_FILE"
  chmod 600 "$INSTALL_TOKEN_FILE"
fi
chown -R openflare-edge:openflare-edge "$STATE_DIR" 2>/dev/null || true
chmod 0750 "$STATE_DIR" 2>/dev/null || true

BIN="$INSTALL_DIR/current"

service_ready() {
  local pid
  [[ -r "$STATE_DIR/ready.json" ]] || return 1
  systemctl is-active --quiet openflare-edge || return 1
  pid="$(systemctl show -p MainPID --value openflare-edge)" || return 1
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || return 1
  [[ "$BIN" -ef "/proc/$pid/exe" ]] &&
    grep -Eq '"pid":'"$pid"'[,}]' "$STATE_DIR/ready.json" &&
    grep -Fq '"binary_version":"'${VERSION#edge-v}'"' "$STATE_DIR/ready.json"
}

for value in "$SERVER_URL" "$INSTALL_TOKEN_FILE" "$STATE_DIR" "$HTTP_LISTEN" "$TLS_LISTEN"; do
  [[ "$value" != *[[:space:]\'\"*?\[\]\{\}\$\`\;\&\|\<\>\#]* ]] || die "argument contains unsupported shell or systemd characters: $value"
done
RUN_ARGS=(run --server-url "$SERVER_URL" --token-type "$TOKEN_TYPE" --token-file "$INSTALL_TOKEN_FILE" --state-dir "$STATE_DIR" --http-listen "$HTTP_LISTEN")
if [[ "$TLS_LISTEN" != "none" ]]; then RUN_ARGS+=(--tls-listen "$TLS_LISTEN"); fi

install -m 0755 "$PACKAGE_DIR/openflare-edge" "$STAGED_BIN"
if [[ "$CREATE_SERVICE" == true && -f /etc/systemd/system/openflare-edge.service ]]; then
  systemctl stop openflare-edge 9>&- || die "could not stop openflare-edge"
fi
mv -Tf -- "$STAGED_BIN" "$BIN"
rm -f -- "$STATE_DIR/edge-update-next" "$STATE_DIR/edge-update-trial" "$STATE_DIR/edge-update-active"

# Migrate installations that stored each release in its own directory.
if [[ -d "$INSTALL_DIR/releases" && ! -L "$INSTALL_DIR/releases" ]]; then
  for release in "$INSTALL_DIR"/releases/edge-v*; do
    [[ -d "$release" && ! -L "$release" ]] && is_release_tag "${release##*/}" || continue
    rm -rf -- "$release"
  done
  rmdir -- "$INSTALL_DIR/releases" 2>/dev/null || true
fi
rm -f -- "$INSTALL_DIR/verified-releases" "$INSTALL_DIR/verified-releases.tmp" "$INSTALL_DIR/.current.tmp"

if [[ "$CREATE_SERVICE" == true ]]; then
  SERVICE_PATH="/etc/systemd/system/openflare-edge.service"
  log "writing openflare-edge.service"
  exec_args="${RUN_ARGS[*]}"
  cat > "${SERVICE_PATH}.tmp" <<SERVICE_EOF
[Unit]
Description=OpenFlare Edge (Pingora)
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=notify
NotifyAccess=main
User=openflare-edge
Group=openflare-edge
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
MemoryDenyWriteExecute=true
RestrictNamespaces=true
SystemCallFilter=@system-service
LimitCORE=0
UMask=0077
ReadWritePaths=$STATE_DIR
ExecStart=$BIN $exec_args
WorkingDirectory=$STATE_DIR
Restart=always
RestartSec=5
KillSignal=SIGTERM
TimeoutStartSec=90
TimeoutStartFailureMode=kill
TimeoutStopSec=300

[Install]
WantedBy=multi-user.target
SERVICE_EOF
  chmod 0644 "${SERVICE_PATH}.tmp"
  mv -Tf -- "${SERVICE_PATH}.tmp" "$SERVICE_PATH"
  systemctl daemon-reload 9>&-
  if ! systemctl is-active --quiet openflare-edge; then
    systemctl enable openflare-edge 9>&-
  fi
  log "starting openflare-edge after any running process has stopped (startup timeout: 90 seconds)"
  if ! systemctl restart openflare-edge 9>&- || ! service_ready; then
    journalctl -u openflare-edge -n 50 --no-pager 9>&- || true
    die "${VERSION} is installed at ${BIN}, but openflare-edge did not become ready"
  fi
  log "openflare-edge is ready"
else
  log "manual start command:"
  printf '  %q' "$BIN"
  printf ' %q' "${RUN_ARGS[@]}"
  printf '\n'
fi

log "installed ${VERSION} at ${BIN}"
