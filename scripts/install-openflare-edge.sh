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
KEEP_RELEASES=3

log() { printf '[openflare-edge] %s\n' "$*"; }
die() { printf '[openflare-edge] ERROR: %s\n' "$*" >&2; exit 1; }
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
SYSTEMCTL_AVAILABLE=false
if command -v systemctl >/dev/null 2>&1; then SYSTEMCTL_AVAILABLE=true; fi
if [[ "$CREATE_SERVICE" == true && "$SYSTEMCTL_AVAILABLE" != true ]]; then
  die "--no-service is required when systemd is unavailable"
fi

if [[ -z "$VERSION" ]]; then
  log "resolving latest edge-v release from ${REPO}"
  RELEASES_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=100")"
  VERSION="$(printf '%s' "$RELEASES_JSON" | grep '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' | grep -m1 '^edge-v' || true)"
  [[ -n "$VERSION" ]] || die "no edge-v release found in ${REPO}"
  RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}")"
else
  RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}")"
fi
VERSION="$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
[[ "$VERSION" =~ ^edge-v[0-9]+(\.[0-9]+){2}(-[0-9A-Za-z.-]+)?$ ]] || die "invalid Edge release version"
ASSET="openflare-edge_${VERSION}_linux_${ARCH}.tar.gz"
DOWNLOAD_URL="$(printf '%s' "$RELEASE_JSON" | grep '"browser_download_url"' | grep "/${ASSET}\"" | sed -E 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' | head -n1)"
[[ -n "$DOWNLOAD_URL" ]] || die "release asset not found: $ASSET"

TMP_DIR="$(mktemp -d "/tmp/openflare-edge.XXXXXX")"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT
log "downloading ${ASSET}"
curl -fsSL -o "$TMP_DIR/$ASSET" "$DOWNLOAD_URL"
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
PACKAGE_DIR="$(find "$TMP_DIR" -mindepth 1 -maxdepth 1 -type d -name "openflare-edge_${VERSION}_linux_${ARCH}" | head -n1)"
[[ -n "$PACKAGE_DIR" ]] || die "release archive layout is invalid"
[[ -x "$PACKAGE_DIR/openflare-edge" ]] || die "release binary is not executable"
(cd "$PACKAGE_DIR" && sha256sum -c SHA256SUMS)

mkdir -p "$INSTALL_DIR" "$STATE_DIR"
if [[ -x "$PACKAGE_DIR/scripts/fetch-agent-geoip-mmdb.sh" ]]; then
  log "refreshing GeoIP databases"
  GEOIP_OUT_DIR="$STATE_DIR" "$PACKAGE_DIR/scripts/fetch-agent-geoip-mmdb.sh" || die "failed to download GeoIP databases"
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

if [[ -n "$TOKEN_FILE" ]]; then
  INSTALL_TOKEN_FILE="${STATE_DIR}/bootstrap-token"
  install -m 0600 "$TOKEN_FILE" "$INSTALL_TOKEN_FILE"
else
  INSTALL_TOKEN_FILE="${STATE_DIR}/bootstrap-token"
  printf '%s' "$TOKEN" > "$INSTALL_TOKEN_FILE"
  chmod 600 "$INSTALL_TOKEN_FILE"
fi
chown -R openflare-edge:openflare-edge "$STATE_DIR" 2>/dev/null || true

RELEASE_DIR="${INSTALL_DIR}/releases/${VERSION}"
CURRENT="${INSTALL_DIR}/current"
PREVIOUS_VERSION="unknown"
if [[ -e "$CURRENT" || -L "$CURRENT" ]]; then
  PREVIOUS_VERSION="$(readlink "$CURRENT" | sed -E 's#^releases/([^/]+)/openflare-edge$#\1#')"
  [[ -n "$PREVIOUS_VERSION" ]] || PREVIOUS_VERSION="unknown"
  log "preserving rollback binary ${PREVIOUS_VERSION}"
fi
if [[ -e "$RELEASE_DIR/openflare-edge" ]]; then
  cmp -s "$PACKAGE_DIR/openflare-edge" "$RELEASE_DIR/openflare-edge" ||
    die "a different binary is already installed for ${VERSION}"
else
  mkdir -p "$RELEASE_DIR"
  install -m 0755 "$PACKAGE_DIR/openflare-edge" "$RELEASE_DIR/openflare-edge"
fi

switch_release() {
  ln -sfn "releases/$1/openflare-edge" "${INSTALL_DIR}/.current.tmp"
  mv -Tf "${INSTALL_DIR}/.current.tmp" "$CURRENT"
}
for value in "$SERVER_URL" "$INSTALL_TOKEN_FILE" "$STATE_DIR" "$HTTP_LISTEN" "$TLS_LISTEN"; do
  [[ "$value" != *[[:space:]\'\"*?\[\]\{\}\$\`\;\&\|\<\>\#]* ]] || die "argument contains unsupported shell or systemd characters: $value"
done
RUN_ARGS=(run --server-url "$SERVER_URL" --token-type "$TOKEN_TYPE" --token-file "$INSTALL_TOKEN_FILE" --state-dir "$STATE_DIR" --http-listen "$HTTP_LISTEN")
if [[ "$TLS_LISTEN" != "none" ]]; then RUN_ARGS+=(--tls-listen "$TLS_LISTEN"); fi

if [[ "$CREATE_SERVICE" == true ]]; then
  SERVICE_PATH="/etc/systemd/system/openflare-edge.service"
  if [[ -f "$SERVICE_PATH" ]]; then
    cp -- "$SERVICE_PATH" "$TMP_DIR/previous.service"
  fi
  log "writing openflare-edge.service"
  exec_args="${RUN_ARGS[*]}"
  cat > "$SERVICE_PATH" <<SERVICE_EOF
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
ReadWritePaths=$STATE_DIR
ExecStart=$INSTALL_DIR/current $exec_args
WorkingDirectory=$STATE_DIR
Restart=always
RestartSec=5
KillSignal=SIGTERM
TimeoutStartSec=90
TimeoutStopSec=300

[Install]
WantedBy=multi-user.target
SERVICE_EOF
  was_active=false
  if systemctl is-active --quiet openflare-edge; then
    was_active=true
  fi
  switch_release "$VERSION"
  systemctl daemon-reload
  if [[ "$was_active" == true ]]; then
    service_action=restart
  else
    service_action=start
    systemctl enable openflare-edge
  fi
  if systemctl "$service_action" openflare-edge && systemctl is-active --quiet openflare-edge; then
    new_pid="$(systemctl show -p MainPID --value openflare-edge)"
    if [[ "$(readlink "/proc/$new_pid/exe" 2>/dev/null || true)" == "$RELEASE_DIR/openflare-edge" ]] &&
       grep -Fq "\"binary_version\":\"${VERSION#edge-v}\"" "$STATE_DIR/ready.json"; then
      log "new release is ready"
    else
      service_action=failed
    fi
  else
    service_action=failed
  fi
  if [[ "$service_action" == failed ]]; then
    journalctl -u openflare-edge -n 50 --no-pager || true
    if [[ "$PREVIOUS_VERSION" == "unknown" || ! -x "${INSTALL_DIR}/releases/${PREVIOUS_VERSION}/openflare-edge" ]]; then
      die "service failed to start and no previous binary is available"
    fi
    switch_release "$PREVIOUS_VERSION"
    if [[ -f "$TMP_DIR/previous.service" ]]; then
      install -m 0644 "$TMP_DIR/previous.service" "$SERVICE_PATH"
    fi
    systemctl daemon-reload
    systemctl reset-failed openflare-edge
    systemctl restart openflare-edge || die "rollback service also failed; inspect journalctl -u openflare-edge"
    die "upgrade failed and service was rolled back to ${PREVIOUS_VERSION}"
  fi
else
  switch_release "$VERSION"
  log "manual start command:"
  printf '  %s/current' "$INSTALL_DIR"
  printf ' %q' "${RUN_ARGS[@]}"
  printf '\n'
fi

mapfile -t releases < <(find "$INSTALL_DIR/releases" -mindepth 1 -maxdepth 1 -type d -name 'edge-v*' -printf '%f\n' | sort -rV)
for ((i=KEEP_RELEASES; i<${#releases[@]}; i++)); do
  if [[ "${releases[i]}" != "$VERSION" && "${releases[i]}" != "$PREVIOUS_VERSION" ]]; then
    rm -rf -- "$INSTALL_DIR/releases/${releases[i]}"
  fi
done

log "installed ${VERSION} to ${INSTALL_DIR}/releases/${VERSION}"
if [[ "$PREVIOUS_VERSION" != "unknown" ]]; then
  log "rollback command: $0 --version ${PREVIOUS_VERSION} --server-url ${SERVER_URL} --token-file ${INSTALL_TOKEN_FILE} --state-dir ${STATE_DIR}"
fi
