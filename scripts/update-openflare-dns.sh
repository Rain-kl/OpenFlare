#!/usr/bin/env bash
set -euo pipefail

VERSION=""
CHANNEL=stable
REPO="Rain-kl/OpenFlare"
INSTALL_DIR="/opt/openflare-dns"
STATE_DIR="/var/lib/openflare-dns"
SERVICE_NAME="openflare-dns"
SERVICE_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
INSTALL_DIR_EXPLICIT=false
STATE_DIR_EXPLICIT=false

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
OpenFlare SmartDNS Updater

Usage:
  update-openflare-dns.sh [canary] [OPTIONS]

Updates only the installed openflare-dns binary while preserving identity state,
offline snapshots, GeoIP databases, and systemd configuration.

Options:
  canary              Update to the most recently published SmartDNS canary prerelease
  --canary            Same as canary
  --channel CHANNEL   Release channel: stable or canary (default: stable)
  --version TAG       Update to a specific dns-v* release tag (default: latest stable)
  --repo OWNER/NAME   Release repository (default: Rain-kl/OpenFlare)
  --install-dir DIR   Override install directory (auto-detected from systemd)
  --state-dir DIR     Override state directory (auto-detected from systemd)
  -h, --help          Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    canary|--canary) CHANNEL=canary; shift ;;
    --channel) CHANNEL="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; INSTALL_DIR_EXPLICIT=true; shift 2 ;;
    --state-dir) STATE_DIR="$2"; STATE_DIR_EXPLICIT=true; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ "$CHANNEL" == "stable" || "$CHANNEL" == "canary" ]] || die "--channel must be stable or canary"
[[ "$CHANNEL" != "canary" || -z "$VERSION" ]] || die "canary and --version are mutually exclusive"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "unsupported architecture: $ARCH" ;;
esac
[[ "$OS" == "linux" ]] || die "this updater supports Linux only"

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar >/dev/null 2>&1 || die "tar is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"
command -v install >/dev/null 2>&1 || die "install is required"
command -v flock >/dev/null 2>&1 || die "flock is required"

if [[ -f "$SERVICE_PATH" ]]; then
  exec_start="$(sed -nE 's/^ExecStart=//p' "$SERVICE_PATH" | head -n1)"
  exec_bin="${exec_start%% *}"
  if [[ "$INSTALL_DIR_EXPLICIT" != true && "$exec_bin" == */current ]]; then
    INSTALL_DIR="${exec_bin%/current}"
  fi
  if [[ "$STATE_DIR_EXPLICIT" != true && "$exec_start" =~ [[:space:]]--cache-dir[[:space:]]([^[:space:]]+) ]]; then
    STATE_DIR="${BASH_REMATCH[1]}"
  fi
fi

BIN="$INSTALL_DIR/current"
[[ -e "$BIN" || -L "$BIN" ]] ||
  die "openflare-dns is not installed in ${INSTALL_DIR}; run install-openflare-dns.sh first"

HAS_SYSTEMD_SERVICE=false
if [[ -f "$SERVICE_PATH" ]] && command -v systemctl >/dev/null 2>&1; then
  HAS_SYSTEMD_SERVICE=true
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

TMP_DIR="$(mktemp -d "/tmp/openflare-dns-update.XXXXXX")"
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

service_ready() {
  local pid deadline
  deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    if [[ -r "$STATE_DIR/ready.json" ]] && systemctl is-active --quiet "$SERVICE_NAME"; then
      pid="$(systemctl show -p MainPID --value "$SERVICE_NAME" 2>/dev/null || true)"
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

exec 9>"$INSTALL_DIR/.install.lock"
flock -n 9 || die "another SmartDNS installation or update holds $INSTALL_DIR/.install.lock"
STAGED_BIN="$INSTALL_DIR/.openflare-dns.new"
install -m 0755 "$PACKAGE_DIR/openflare-dns" "$STAGED_BIN"

if [[ "$HAS_SYSTEMD_SERVICE" == true ]]; then
  log "stopping ${SERVICE_NAME}"
  systemctl stop "$SERVICE_NAME" 9>&- || die "could not stop ${SERVICE_NAME}"
fi

log "replacing ${BIN} with ${VERSION}"
mv -Tf -- "$STAGED_BIN" "$BIN"
rm -f -- "$STATE_DIR/ready.json"

if [[ "$HAS_SYSTEMD_SERVICE" == true ]]; then
  log "starting ${SERVICE_NAME}"
  if ! systemctl start "$SERVICE_NAME" 9>&- || ! service_ready; then
    journalctl -u "$SERVICE_NAME" -n 50 --no-pager 9>&- || true
    die "${VERSION} is installed at ${BIN}, but ${SERVICE_NAME} did not become ready"
  fi
else
  log "restart your openflare-dns process manually"
fi

log "updated to ${VERSION} at ${BIN}"
