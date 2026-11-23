#!/usr/bin/env bash
set -euo pipefail

VERSION=""
SERVER_URL=""
TOKEN=""
TOKEN_FILE=""
STATE_DIR="/var/lib/openflared"
INSTALL_DIR="/opt/openflared"
CONNECTOR_ID=""
REPO="Rain-kl/OpenFlare"
CREATE_SERVICE=true

log() { printf '[openflared] %s\n' "$*"; }
die() { printf '[openflared] ERROR: %s\n' "$*" >&2; exit 1; }
is_release_tag() { [[ "$1" =~ ^tunnel-v[0-9]+(\.[0-9]+){2}(-[0-9A-Za-z.-]+)?$ ]]; }

usage() {
  cat <<'EOF'
OpenFlared Connector Installer

Usage:
  install-openflared.sh [OPTIONS]

Required:
  --token TOKEN              Tunnel token or bootstrap token (oftok1_...)
  --token-file PATH          Read token from PATH (alternative to --token)

Options:
  --server-url URL           OpenFlare Server URL (optional when token starts with oftok1_)
  --version TAG              Install a tunnel release tag (default: latest stable)
  --repo OWNER/NAME          Release repository (default: Rain-kl/OpenFlare)
  --install-dir DIR          Default: /opt/openflared
  --state-dir DIR            Default: /var/lib/openflared
  --connector-id ID          Explicit persistent connector ID
  --no-service               Do not create or restart systemd service
  -h, --help                 Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-url) SERVER_URL="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    --token-file) TOKEN_FILE="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --state-dir) STATE_DIR="$2"; shift 2 ;;
    --connector-id) CONNECTOR_ID="$2"; shift 2 ;;
    --no-service) CREATE_SERVICE=false; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ -n "$TOKEN" || -n "$TOKEN_FILE" ]] || die "--token or --token-file is required"

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
if [[ "$TOKEN" != oftok1_* && -z "$SERVER_URL" ]]; then
  die "--server-url is required unless using an oftok1_ bootstrap token"
fi

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
  log "resolving latest stable Tunnel release from ${REPO}"
  for ((page = 1; ; page++)); do
    releases_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=100&page=${page}")"
    mapfile -t release_tags < <(printf '%s' "$releases_json" | sed -nE 's/^[[:space:]]*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p')
    VERSION="$(printf '%s\n' "${release_tags[@]}" | grep -E '^tunnel-v[0-9]+(\.[0-9]+){2}$' | sort -Vr | sed -n '1p' || true)"
    if [[ -n "$VERSION" || ${#release_tags[@]} -lt 100 ]]; then break; fi
  done
  [[ -n "$VERSION" ]] || die "no stable Tunnel release found in ${REPO}"
fi
is_release_tag "$VERSION" || die "invalid Tunnel release version: $VERSION"
RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}")"
VERSION="$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
is_release_tag "$VERSION" || die "invalid Tunnel release version: $VERSION"

COMBINED_ASSET="openflare-tunnel_${VERSION}_linux_${ARCH}.tar.gz"
SPECIFIC_ASSET="openflared_${VERSION}_linux_${ARCH}.tar.gz"
DOWNLOAD_URL=""
ASSET=""
for candidate_asset in "$COMBINED_ASSET" "$SPECIFIC_ASSET"; do
  DOWNLOAD_URL="$(printf '%s' "$RELEASE_JSON" | grep '"browser_download_url"' | grep "/${candidate_asset}\"" | sed -E 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' | head -n1 || true)"
  if [[ -n "$DOWNLOAD_URL" ]]; then
    ASSET="$candidate_asset"
    break
  fi
done
[[ -n "$DOWNLOAD_URL" ]] || die "release asset not found: $COMBINED_ASSET or $SPECIFIC_ASSET"

TMP_DIR="$(mktemp -d "/tmp/openflared.XXXXXX")"
STAGED_BIN=""
cleanup() {
  if [[ -n "$STAGED_BIN" ]]; then rm -f -- "$STAGED_BIN"; fi
  rm -rf -- "$TMP_DIR"
}
trap cleanup EXIT

log "downloading ${ASSET}"
curl -fsSL -o "$TMP_DIR/$ASSET" "$DOWNLOAD_URL"
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
PACKAGE_DIR="$(find "$TMP_DIR" -mindepth 1 -maxdepth 1 -type d -name "${ASSET%.tar.gz}" | head -n1)"
[[ -n "$PACKAGE_DIR" ]] || die "release archive layout is invalid"
[[ -x "$PACKAGE_DIR/openflared" ]] || die "release binary is not executable"
[[ -r "$PACKAGE_DIR/SHA256SUMS" ]] || die "release archive missing SHA256SUMS"
CHECKSUM_LINE="$(grep -E "^[0-9a-fA-F]{64}[[:space:]]+\*?openflared$" "$PACKAGE_DIR/SHA256SUMS" | head -n1 || true)"
[[ -n "$CHECKSUM_LINE" ]] || die "SHA256SUMS missing entry for openflared"
(cd "$PACKAGE_DIR" && printf '%s\n' "$CHECKSUM_LINE" | sha256sum -c -)
VER_OUT="$("$PACKAGE_DIR/openflared" --version 2>/dev/null || true)"
[[ "$VER_OUT" == *"openflared"* && "$VER_OUT" == *"${VERSION#tunnel-v}"* ]] ||
  die "unexpected openflared --version output: $VER_OUT"

mkdir -p "$INSTALL_DIR" "$STATE_DIR"
exec 9>"$INSTALL_DIR/.install.lock"
flock -n 9 || die "another OpenFlared installation or update holds $INSTALL_DIR/.install.lock"
STAGED_BIN="$INSTALL_DIR/.openflared.new"

ensure_runtime_user() {
  if id openflared >/dev/null 2>&1; then return; fi
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --home-dir "$STATE_DIR" --shell /usr/sbin/nologin openflared
  else
    die "useradd is required to create the openflared service account"
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
chown -R openflared:openflared "$STATE_DIR" 2>/dev/null || true
chmod 0750 "$STATE_DIR" 2>/dev/null || true

BIN="$INSTALL_DIR/current"

for value in "$SERVER_URL" "$INSTALL_TOKEN_FILE" "$STATE_DIR" "$CONNECTOR_ID"; do
  [[ "$value" != *[[:space:]\'\"*?\[\]\{\}\$\`\;\&\|\<\>\#]* ]] || die "argument contains unsupported shell or systemd characters: $value"
done

RUN_ARGS=(--token-file "$INSTALL_TOKEN_FILE" --state-dir "$STATE_DIR")
if [[ -n "$SERVER_URL" ]]; then RUN_ARGS+=(--server-url "$SERVER_URL"); fi
if [[ -n "$CONNECTOR_ID" ]]; then RUN_ARGS+=(--connector-id "$CONNECTOR_ID"); fi

install -m 0755 "$PACKAGE_DIR/openflared" "$STAGED_BIN"
if [[ "$CREATE_SERVICE" == true && -f /etc/systemd/system/openflared.service ]]; then
  systemctl stop openflared 9>&- || die "could not stop openflared"
fi
mv -Tf -- "$STAGED_BIN" "$BIN"
rm -f -- "$STATE_DIR/openflared-next" "$STATE_DIR/openflared-trial" "$STATE_DIR/openflared-active"

if [[ "$CREATE_SERVICE" == true ]]; then
  SERVICE_PATH="/etc/systemd/system/openflared.service"
  log "writing openflared.service"
  exec_args="${RUN_ARGS[*]}"
  cat > "${SERVICE_PATH}.tmp" <<SERVICE_EOF
[Unit]
Description=OpenFlared Native Tunnel Connector
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=notify
NotifyAccess=main
User=openflared
Group=openflared
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
TimeoutStopSec=60

[Install]
WantedBy=multi-user.target
SERVICE_EOF
  chmod 0644 "${SERVICE_PATH}.tmp"
  mv -Tf -- "${SERVICE_PATH}.tmp" "$SERVICE_PATH"
  systemctl daemon-reload 9>&-
  if ! systemctl is-active --quiet openflared; then
    systemctl enable openflared 9>&-
  fi
  log "starting openflared"
  if ! systemctl restart openflared 9>&- || ! systemctl is-active --quiet openflared; then
    journalctl -u openflared -n 50 --no-pager 9>&- || true
    die "${VERSION} is installed at ${BIN}, but openflared did not become ready"
  fi
  log "openflared is active"
else
  log "manual start command:"
  printf '  %q' "$BIN"
  printf ' %q' "${RUN_ARGS[@]}"
  printf '\n'
fi

log "installed ${VERSION} at ${BIN}"
