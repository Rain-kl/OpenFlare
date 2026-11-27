#!/usr/bin/env bash
set -euo pipefail

VERSION=""
CHANNEL=stable
CONTROLLER_URL=""
TOKEN_TYPE="discovery"
TOKEN_TYPE_EXPLICIT=false
TOKEN=""
TOKEN_FILE=""
INSTANCE_ID=""
INSTANCE_NAME=""
PUBLIC_ADDR=""
LISTEN_ADDR="0.0.0.0:53"
WORKERS=""
RRL_QPS=""
RRL_BURST=""
SNAPSHOT_INTERVAL=""
STATE_DIR="/var/lib/openflare-dns"
INSTALL_DIR="/opt/openflare-dns"
GEOIP_DIR=""
REPO="Rain-kl/OpenFlare"
SKIP_GEOIP=false
CREATE_SERVICE=true

log() { printf '[openflare-dns] %s\n' "$*"; }
die() { printf '[openflare-dns] ERROR: %s\n' "$*" >&2; exit 1; }
is_release_tag() { [[ "$1" =~ ^dns-v[0-9]+(\.[0-9]+){2}(-[0-9A-Za-z.-]+)?$ ]]; }

canary_releases() {
  # Read release fields at the top level of each object in GitHub's formatted response.
  awk -F '"' '
    /^  \{$/ { tag = draft = prerelease = published = "" }
    /^    "tag_name": / { tag = $4 }
    /^    "draft": false,?$/ { draft = "false" }
    /^    "prerelease": true,?$/ { prerelease = "true" }
    /^    "published_at": / { published = $4 }
    /^  \},?$/ {
      if (tag ~ /^dns-v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$/ &&
          draft == "false" && prerelease == "true" && published != "") {
        print published, tag
      }
    }
  '
}

usage() {
  cat <<'EOF'
OpenFlare SmartDNS Installer

Usage:
  install-openflare-dns.sh [canary] [OPTIONS]

Required:
  --controller-url URL           OpenFlare Controller URL (alias: --server-url)
  --token TOKEN                  Discovery or instance access token
  --discovery-token TOKEN        Discovery token (sets --token-type discovery)
  --access-token TOKEN           Instance access token (sets --token-type instance)
  --token-file PATH              Read token from PATH (alternative to inline token)

Options:
  canary                         Install the latest published SmartDNS canary prerelease
  --canary                       Same as canary
  --channel CHANNEL              Release channel: stable or canary (default: stable)
  --token-type TYPE              discovery or instance (default: discovery)
  --instance-id ID               Explicit SmartDNS instance ID
  --name NAME                    Display name for self-registration
  --public-addr ADDR             Public IP/host announced by this instance
  --listen ADDR                  UDP/TCP bind address (default: 0.0.0.0:53)
  --workers N                    SO_REUSEPORT UDP worker count (0 = auto)
  --rrl-qps N                    Per-prefix RRL steady rate (default: 1000)
  --rrl-burst N                  Per-prefix RRL burst size (default: 2000)
  --snapshot-interval N          Snapshot sync interval seconds (default: 10)
  --version TAG                  Install a specific dns-v* tag (mutually exclusive with canary)
  --repo OWNER/NAME              Release repository (default: Rain-kl/OpenFlare)
  --install-dir DIR              Default: /opt/openflare-dns
  --state-dir DIR                Default: /var/lib/openflare-dns
  --geoip-dir DIR                Default: <state-dir>/geoip
  --skip-geoip                   Skip downloading open GeoIP MMDB databases
  --no-service                   Do not create or restart systemd service
  -h, --help                     Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    canary|--canary) CHANNEL=canary; shift ;;
    --channel) CHANNEL="$2"; shift 2 ;;
    --controller-url|--server-url) CONTROLLER_URL="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    --discovery-token) TOKEN="$2"; TOKEN_TYPE="discovery"; TOKEN_TYPE_EXPLICIT=true; shift 2 ;;
    --access-token) TOKEN="$2"; TOKEN_TYPE="instance"; TOKEN_TYPE_EXPLICIT=true; shift 2 ;;
    --token-file) TOKEN_FILE="$2"; shift 2 ;;
    --token-type) TOKEN_TYPE="$2"; TOKEN_TYPE_EXPLICIT=true; shift 2 ;;
    --instance-id) INSTANCE_ID="$2"; shift 2 ;;
    --name) INSTANCE_NAME="$2"; shift 2 ;;
    --public-addr) PUBLIC_ADDR="$2"; shift 2 ;;
    --listen) LISTEN_ADDR="$2"; shift 2 ;;
    --workers) WORKERS="$2"; shift 2 ;;
    --rrl-qps) RRL_QPS="$2"; shift 2 ;;
    --rrl-burst) RRL_BURST="$2"; shift 2 ;;
    --snapshot-interval) SNAPSHOT_INTERVAL="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --state-dir) STATE_DIR="$2"; shift 2 ;;
    --geoip-dir) GEOIP_DIR="$2"; shift 2 ;;
    --skip-geoip) SKIP_GEOIP=true; shift ;;
    --no-service) CREATE_SERVICE=false; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ "$CHANNEL" == "stable" || "$CHANNEL" == "canary" ]] || die "--channel must be stable or canary"
[[ "$CHANNEL" != "canary" || -z "$VERSION" ]] || die "canary and --version are mutually exclusive"
[[ -n "$CONTROLLER_URL" ]] || die "--controller-url is required"
[[ -n "$TOKEN" || -n "$TOKEN_FILE" ]] || die "--token, --discovery-token, --access-token, or --token-file is required"
if [[ "$TOKEN_TYPE" == "access" ]]; then TOKEN_TYPE="instance"; fi
[[ "$TOKEN_TYPE" == "discovery" || "$TOKEN_TYPE" == "instance" ]] || die "--token-type must be discovery or instance"
if [[ -n "$INSTANCE_ID" && "$TOKEN_TYPE_EXPLICIT" != true ]]; then
  TOKEN_TYPE="discovery"
fi
if [[ -z "$GEOIP_DIR" ]]; then
  GEOIP_DIR="${STATE_DIR}/geoip"
fi

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
  log "resolving latest ${CHANNEL} SmartDNS release from ${REPO}"
  latest_canary=""
  for ((page = 1; ; page++)); do
    releases_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=100&page=${page}")"
    mapfile -t release_tags < <(printf '%s' "$releases_json" | sed -nE 's/^[[:space:]]*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p')
    if [[ "$CHANNEL" == "canary" ]]; then
      candidate="$(printf '%s' "$releases_json" | canary_releases | LC_ALL=C sort -r | sed -n '1p')"
      if [[ "$candidate" > "$latest_canary" ]]; then
        latest_canary="$candidate"
        VERSION="${candidate#* }"
      fi
    else
      VERSION="$(printf '%s\n' "${release_tags[@]}" | grep -E '^dns-v[0-9]+(\.[0-9]+){2}$' | sort -Vr | sed -n '1p' || true)"
      if [[ -n "$VERSION" ]]; then break; fi
    fi
    if [[ ${#release_tags[@]} -lt 100 ]]; then break; fi
  done
  [[ -n "$VERSION" ]] || die "no ${CHANNEL} SmartDNS release found in ${REPO}"
fi
is_release_tag "$VERSION" || die "invalid SmartDNS release version: $VERSION"
RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}")"
VERSION="$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
is_release_tag "$VERSION" || die "invalid SmartDNS release version: $VERSION"

ASSET="openflare-dns_${VERSION}_linux_${ARCH}.tar.gz"
DOWNLOAD_URL="$(printf '%s' "$RELEASE_JSON" | grep '"browser_download_url"' | grep "/${ASSET}\"" | sed -E 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' | head -n1)"
[[ -n "$DOWNLOAD_URL" ]] || die "release asset not found: $ASSET"

TMP_DIR="$(mktemp -d "/tmp/openflare-dns.XXXXXX")"
STAGED_BIN=""
cleanup() {
  if [[ -n "$STAGED_BIN" ]]; then rm -f -- "$STAGED_BIN"; fi
  rm -rf -- "$TMP_DIR"
}
trap cleanup EXIT

log "downloading ${ASSET}"
curl -fsSL -o "$TMP_DIR/$ASSET" "$DOWNLOAD_URL"
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
PACKAGE_DIR="$(find "$TMP_DIR" -mindepth 1 -maxdepth 1 -type d -name "openflare-dns_${VERSION}_linux_${ARCH}" | head -n1)"
[[ -n "$PACKAGE_DIR" ]] || die "release archive layout is invalid"
[[ -x "$PACKAGE_DIR/openflare-dns" ]] || die "release binary is not executable"
[[ -r "$PACKAGE_DIR/SHA256SUMS" ]] || die "release archive missing SHA256SUMS"
(cd "$PACKAGE_DIR" && sha256sum -c SHA256SUMS)
VER_OUT="$("$PACKAGE_DIR/openflare-dns" --version 2>/dev/null || true)"
[[ "$VER_OUT" == *"openflare-dns"* && "$VER_OUT" == *"${VERSION#dns-v}"* ]] ||
  die "unexpected openflare-dns --version output: $VER_OUT"

mkdir -p "$INSTALL_DIR" "$STATE_DIR" "$GEOIP_DIR"
exec 9>"$INSTALL_DIR/.install.lock"
flock -n 9 || die "another SmartDNS installation or update holds $INSTALL_DIR/.install.lock"
STAGED_BIN="$INSTALL_DIR/.openflare-dns.new"

if [[ "$SKIP_GEOIP" != true && -x "$PACKAGE_DIR/scripts/fetch-netintel-dbs.sh" ]]; then
  log "refreshing open GeoIP databases in ${GEOIP_DIR} (best-effort)"
  if ! NETINTEL_OUT_DIR="$GEOIP_DIR" NETINTEL_SKIP_BGPTOOLS=true "$PACKAGE_DIR/scripts/fetch-netintel-dbs.sh" 9>&-; then
    log "WARNING: GeoIP database download failed; continuing without refreshed MMDB files"
  fi
fi

ensure_runtime_user() {
  if id openflare-dns >/dev/null 2>&1; then return; fi
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --home-dir "$STATE_DIR" --shell /usr/sbin/nologin openflare-dns
  else
    die "useradd is required to create the openflare-dns service account"
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
chown -R openflare-dns:openflare-dns "$STATE_DIR" "$GEOIP_DIR" 2>/dev/null || true
chmod 0750 "$STATE_DIR" "$GEOIP_DIR" 2>/dev/null || true

BIN="$INSTALL_DIR/current"

for value in "$CONTROLLER_URL" "$INSTALL_TOKEN_FILE" "$STATE_DIR" "$GEOIP_DIR" "$LISTEN_ADDR" "$INSTANCE_ID" "$INSTANCE_NAME" "$PUBLIC_ADDR" "$WORKERS" "$RRL_QPS" "$RRL_BURST" "$SNAPSHOT_INTERVAL"; do
  [[ "$value" != *[[:space:]\'\"*?\[\]\{\}\$\`\;\&\|\<\>\#]* ]] || die "argument contains unsupported shell or systemd characters: $value"
done

RUN_ARGS=(serve --controller-url "$CONTROLLER_URL" --token-type "$TOKEN_TYPE" --token-file "$INSTALL_TOKEN_FILE" --listen "$LISTEN_ADDR" --cache-dir "$STATE_DIR" --geoip-dir "$GEOIP_DIR")
if [[ -n "$INSTANCE_ID" ]]; then RUN_ARGS+=(--instance-id "$INSTANCE_ID"); fi
if [[ -n "$INSTANCE_NAME" ]]; then RUN_ARGS+=(--name "$INSTANCE_NAME"); fi
if [[ -n "$PUBLIC_ADDR" ]]; then RUN_ARGS+=(--public-addr "$PUBLIC_ADDR"); fi
if [[ -n "$WORKERS" ]]; then RUN_ARGS+=(--workers "$WORKERS"); fi
if [[ -n "$RRL_QPS" ]]; then RUN_ARGS+=(--rrl-qps "$RRL_QPS"); fi
if [[ -n "$RRL_BURST" ]]; then RUN_ARGS+=(--rrl-burst "$RRL_BURST"); fi
if [[ -n "$SNAPSHOT_INTERVAL" ]]; then RUN_ARGS+=(--snapshot-interval-secs "$SNAPSHOT_INTERVAL"); fi

service_ready() {
  local pid deadline
  deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    if [[ -r "$STATE_DIR/ready.json" ]] && systemctl is-active --quiet openflare-dns; then
      pid="$(systemctl show -p MainPID --value openflare-dns 2>/dev/null || true)"
      if [[ "$pid" =~ ^[1-9][0-9]*$ ]] &&
        [[ "$BIN" -ef "/proc/$pid/exe" ]] &&
        grep -Eq '"pid":'"$pid"'[,}]' "$STATE_DIR/ready.json" &&
        grep -Fq '"binary_version":"'"${VERSION#dns-v}"'"' "$STATE_DIR/ready.json"; then
        return 0
      fi
    fi
    sleep 0.5
  done
  return 1
}

install -m 0755 "$PACKAGE_DIR/openflare-dns" "$STAGED_BIN"
if [[ "$CREATE_SERVICE" == true && -f /etc/systemd/system/openflare-dns.service ]]; then
  systemctl stop openflare-dns 9>&- || die "could not stop openflare-dns"
fi
mv -Tf -- "$STAGED_BIN" "$BIN"
rm -f -- "$STATE_DIR/ready.json"

if [[ "$CREATE_SERVICE" == true ]]; then
  SERVICE_PATH="/etc/systemd/system/openflare-dns.service"
  log "writing openflare-dns.service"
  exec_args="${RUN_ARGS[*]}"
  rw_paths="$STATE_DIR"
  if [[ "$GEOIP_DIR" != "$STATE_DIR" && "$GEOIP_DIR" != "$STATE_DIR/"* ]]; then
    rw_paths="${rw_paths} ${GEOIP_DIR}"
  fi
  cat > "${SERVICE_PATH}.tmp" <<SERVICE_EOF
[Unit]
Description=OpenFlare SmartDNS Authoritative Server
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
User=openflare-dns
Group=openflare-dns
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
RestrictNamespaces=true
SystemCallFilter=@system-service
LimitNOFILE=65536
LimitCORE=0
UMask=0077
ReadWritePaths=$rw_paths
ExecStart=$BIN $exec_args
WorkingDirectory=$STATE_DIR
Restart=always
RestartSec=5
KillSignal=SIGTERM
TimeoutStartSec=30
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
SERVICE_EOF
  chmod 0644 "${SERVICE_PATH}.tmp"
  mv -Tf -- "${SERVICE_PATH}.tmp" "$SERVICE_PATH"
  systemctl daemon-reload 9>&-
  if ! systemctl is-active --quiet openflare-dns; then
    systemctl enable openflare-dns 9>&-
  fi
  log "starting openflare-dns (readiness timeout: 30 seconds)"
  if ! systemctl restart openflare-dns 9>&- || ! service_ready; then
    journalctl -u openflare-dns -n 50 --no-pager 9>&- || true
    die "${VERSION} is installed at ${BIN}, but openflare-dns did not become ready"
  fi
  log "openflare-dns is ready"
else
  log "manual start command:"
  printf '  %q' "$BIN"
  printf ' %q' "${RUN_ARGS[@]}"
  printf '\n'
fi

log "installed ${VERSION} at ${BIN}"
