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
  log "resolving latest edge-v release from ${REPO}"
  RELEASES_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=100")"
  VERSION="$(printf '%s' "$RELEASES_JSON" | grep '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' | grep -m1 '^edge-v' || true)"
  [[ -n "$VERSION" ]] || die "no edge-v release found in ${REPO}"
  RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}")"
else
  RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}")"
fi
VERSION="$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
is_release_tag "$VERSION" || die "invalid Edge release version"
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
[[ "$("$PACKAGE_DIR/openflare-edge" config-sync-version 2>/dev/null || true)" == 1 ]] ||
  die "${VERSION} does not support configuration sync protocol v1"

mkdir -p "$INSTALL_DIR" "$STATE_DIR"
exec 9>"$INSTALL_DIR/.install.lock"
flock -x 9
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

RELEASES_DIR="${INSTALL_DIR}/releases"
RELEASE_DIR="${RELEASES_DIR}/${VERSION}"
CURRENT="${INSTALL_DIR}/current"
VERIFIED_FILE="${INSTALL_DIR}/verified-releases"
PREVIOUS_TARGET=""
if [[ -e "$CURRENT" || -L "$CURRENT" ]]; then
  PREVIOUS_TARGET="$(readlink "$CURRENT")" || die "current must be a symbolic link"
fi
installed_release() {
  is_release_tag "$1" && [[ -x "$RELEASES_DIR/$1/openflare-edge" ]]
}
service_ready() {
  local version="$1" pid executable
  [[ -r "$STATE_DIR/ready.json" ]] || return 1
  systemctl is-active --quiet openflare-edge || return 1
  pid="$(systemctl show -p MainPID --value openflare-edge)" || return 1
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || return 1
  executable="$(readlink -f -- "$RELEASES_DIR/$version/openflare-edge")" || return 1
  [[ "$(readlink "/proc/$pid/exe" 2>/dev/null)" == "$executable" ]] &&
    grep -Eq '"pid":'"$pid"'[,}]' "$STATE_DIR/ready.json" &&
    grep -Fq '"binary_version":"'"${version#edge-v}"'"' "$STATE_DIR/ready.json"
}
last_verified=""
previous_verified=""
rollback_target="$PREVIOUS_TARGET"
if [[ "$CREATE_SERVICE" == true ]]; then
  verified=()
  if [[ -f "$VERIFIED_FILE" ]]; then mapfile -t verified < "$VERIFIED_FILE"; fi
  if installed_release "${verified[0]:-}"; then last_verified="${verified[0]}"; fi
  if installed_release "${verified[1]:-}" && [[ "${verified[1]}" != "$last_verified" ]]; then
    previous_verified="${verified[1]}"
  fi
  if [[ -z "$last_verified" ]]; then
    last_verified="$previous_verified"
    previous_verified=""
  fi
  before="${PREVIOUS_TARGET#releases/}"
  before="${before%/openflare-edge}"
  if [[ "$PREVIOUS_TARGET" != "releases/$before/openflare-edge" ]]; then before=""; fi
  for candidate in "$before" "$last_verified" "$previous_verified"; do
    installed_release "$candidate" && service_ready "$candidate" || continue
    # A manual install may have moved current without restarting the service.
    rollback_target="releases/$candidate/openflare-edge"
    if [[ "$candidate" != "$last_verified" ]]; then
      previous_verified="$last_verified"
      last_verified="$candidate"
    fi
    break
  done
fi
if [[ -e "$RELEASE_DIR/openflare-edge" ]]; then
  cmp -s "$PACKAGE_DIR/openflare-edge" "$RELEASE_DIR/openflare-edge" ||
    die "a different binary is already installed for ${VERSION}"
else
  mkdir -p "$RELEASE_DIR"
  install -m 0755 "$PACKAGE_DIR/openflare-edge" "$RELEASE_DIR/openflare-edge"
fi

switch_release() {
  ln -sfn -- "$1" "${INSTALL_DIR}/.current.tmp"
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
TimeoutStartFailureMode=kill
TimeoutStopSec=300

[Install]
WantedBy=multi-user.target
SERVICE_EOF
  was_active=false
  if systemctl is-active --quiet openflare-edge; then
    was_active=true
  fi
  switch_release "releases/$VERSION/openflare-edge"
  systemctl daemon-reload
  if [[ "$was_active" == true ]]; then
    service_action=restart
  else
    service_action=start
    systemctl enable openflare-edge
  fi
  log "waiting for systemd readiness (startup timeout: 90 seconds)"
  if ! systemctl "$service_action" openflare-edge || ! service_ready "$VERSION"; then
    journalctl -u openflare-edge -n 50 --no-pager || true
    systemctl stop openflare-edge || die "could not stop the failed release"
    if [[ -n "$rollback_target" ]]; then switch_release "$rollback_target"; else rm -f -- "$CURRENT"; fi
    if [[ -f "$TMP_DIR/previous.service" ]]; then
      install -m 0644 "$TMP_DIR/previous.service" "$SERVICE_PATH"
    else
      systemctl disable openflare-edge
      rm -f -- "$SERVICE_PATH"
    fi
    systemctl daemon-reload
    if [[ "$was_active" == true ]]; then
      systemctl reset-failed openflare-edge
      systemctl start openflare-edge || die "rollback service also failed; inspect journalctl -u openflare-edge"
    fi
    if [[ "$rollback_target" != "releases/$VERSION/openflare-edge" && "$VERSION" != "$last_verified" && "$VERSION" != "$previous_verified" ]]; then
      rm -rf -- "$RELEASE_DIR"
    fi
    die "upgrade failed; previous installation restored"
  fi
  log "new release is ready"

  if [[ -n "$last_verified" && "$last_verified" != "$VERSION" ]]; then
    previous_verified="$last_verified"
  fi
  printf '%s\n%s\n' "$VERSION" "$previous_verified" > "$VERIFIED_FILE.tmp"
  mv -Tf -- "$VERIFIED_FILE.tmp" "$VERIFIED_FILE"
  for release in "$RELEASES_DIR"/edge-v*; do
    tag="${release##*/}"
    [[ -d "$release" && ! -L "$release" ]] && is_release_tag "$tag" || continue
    if [[ "$tag" != "$VERSION" && "$tag" != "$previous_verified" ]]; then rm -rf -- "$release"; fi
  done
  if [[ -n "$previous_verified" ]]; then log "retained rollback release ${previous_verified}"; fi
else
  switch_release "releases/$VERSION/openflare-edge"
  log "manual start command:"
  printf '  %s/current' "$INSTALL_DIR"
  printf ' %q' "${RUN_ARGS[@]}"
  printf '\n'
fi

log "installed ${VERSION} to ${INSTALL_DIR}/releases/${VERSION}"
