#!/usr/bin/env bash
set -euo pipefail

VERSION=""
REPO="Rain-kl/OpenFlare"
INSTALL_DIR="/opt/openflare-edge"
STATE_DIR="/var/lib/openflare-edge"
SERVICE_NAME="openflare-edge"
SERVICE_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
INSTALL_DIR_EXPLICIT=false
STATE_DIR_EXPLICIT=false

log() { printf '[openflare-edge] %s\n' "$*"; }
die() { printf '[openflare-edge] ERROR: %s\n' "$*" >&2; exit 1; }
is_release_tag() { [[ "$1" =~ ^edge-v[0-9]+(\.[0-9]+){2}(-[0-9A-Za-z.-]+)?$ ]]; }

usage() {
  cat <<'EOF'
OpenFlare Edge Updater

Usage:
  update-openflare-edge.sh [OPTIONS]

Updates only the installed openflare-edge binary without modifying service
configuration, tokens, snapshots, or GeoIP databases. No arguments are
required for standard installations.

Options:
  --version TAG       Update to a specific release tag; default is latest
  --repo OWNER/NAME   Release repository (default: Rain-kl/OpenFlare)
  --install-dir DIR   Override install directory (auto-detected from systemd)
  --state-dir DIR     Override state directory (auto-detected from systemd)
  -h, --help          Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; INSTALL_DIR_EXPLICIT=true; shift 2 ;;
    --state-dir) STATE_DIR="$2"; STATE_DIR_EXPLICIT=true; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

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
  if [[ "$INSTALL_DIR_EXPLICIT" != true ]]; then
    exec_start="$(sed -nE 's/^ExecStart=//p' "$SERVICE_PATH" | head -n1)"
    exec_bin="${exec_start%% *}"
    if [[ "$exec_bin" == */current ]]; then
      INSTALL_DIR="${exec_bin%/current}"
    fi
  fi
  if [[ "$STATE_DIR_EXPLICIT" != true ]]; then
    working_dir="$(sed -nE 's/^WorkingDirectory=//p' "$SERVICE_PATH" | head -n1)"
    if [[ -n "$working_dir" ]]; then
      STATE_DIR="$working_dir"
    fi
  fi
fi

[[ -d "$INSTALL_DIR" && ( -e "${INSTALL_DIR}/current" || -L "${INSTALL_DIR}/current" ) ]] ||
  die "openflare-edge is not installed in ${INSTALL_DIR}; run install-openflare-edge.sh first"

HAS_SYSTEMD_SERVICE=false
if [[ -f "$SERVICE_PATH" ]] && command -v systemctl >/dev/null 2>&1; then
  HAS_SYSTEMD_SERVICE=true
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

BIN="$INSTALL_DIR/current"

service_ready() {
  local pid
  [[ -r "$STATE_DIR/ready.json" ]] || return 1
  systemctl is-active --quiet "$SERVICE_NAME" || return 1
  pid="$(systemctl show -p MainPID --value "$SERVICE_NAME")" || return 1
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || return 1
  [[ "$BIN" -ef "/proc/$pid/exe" ]] &&
    grep -Eq '"pid":'"$pid"'[,}]' "$STATE_DIR/ready.json" &&
    grep -Fq '"binary_version":"'${VERSION#edge-v}'"' "$STATE_DIR/ready.json"
}

exec 9>"$INSTALL_DIR/.install.lock"
flock -x 9
STAGED_BIN="$INSTALL_DIR/.openflare-edge.new"
rm -f -- "$STAGED_BIN"

# A legacy symlink still needs to be converted even when the release is current.
if [[ -x "$BIN" && ! -L "$BIN" && ! -e "$INSTALL_DIR/verified-releases" && ! -d "$INSTALL_DIR/releases" ]]; then
  if [[ "$HAS_SYSTEMD_SERVICE" == true ]] && service_ready; then
    log "already running ${VERSION}; nothing to update"
    exit 0
  fi
  if [[ "$HAS_SYSTEMD_SERVICE" != true && "$("$BIN" --version)" == "openflare-edge ${VERSION#edge-v}" ]]; then
    log "already installed ${VERSION}; nothing to update"
    exit 0
  fi
fi

ASSET="openflare-edge_${VERSION}_linux_${ARCH}.tar.gz"
DOWNLOAD_URL="$(printf '%s' "$RELEASE_JSON" | grep '"browser_download_url"' | grep "/${ASSET}\"" | sed -E 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' | head -n1)"
[[ -n "$DOWNLOAD_URL" ]] || die "release asset not found: $ASSET"

TMP_DIR="$(mktemp -d "/tmp/openflare-edge-update.XXXXXX")"
cleanup() { rm -rf -- "$TMP_DIR"; rm -f -- "$STAGED_BIN"; }
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

if [[ ! -x "$BIN" || -L "$BIN" ]] || ! cmp -s "$PACKAGE_DIR/openflare-edge" "$BIN"; then
  install -m 0755 "$PACKAGE_DIR/openflare-edge" "$STAGED_BIN"
  mv -Tf -- "$STAGED_BIN" "$BIN"
fi

# Migrate installations that stored each release in its own directory.
if [[ -d "$INSTALL_DIR/releases" && ! -L "$INSTALL_DIR/releases" ]]; then
  for release in "$INSTALL_DIR"/releases/edge-v*; do
    [[ -d "$release" && ! -L "$release" ]] && is_release_tag "${release##*/}" || continue
    rm -rf -- "$release"
  done
  rmdir -- "$INSTALL_DIR/releases" 2>/dev/null || true
fi
rm -f -- "$INSTALL_DIR/verified-releases" "$INSTALL_DIR/verified-releases.tmp" "$INSTALL_DIR/.current.tmp"

if [[ "$HAS_SYSTEMD_SERVICE" == true ]]; then
  if ! service_ready; then
    log "starting ${SERVICE_NAME} after any running process has stopped"
    if ! systemctl restart "$SERVICE_NAME" || ! service_ready; then
      journalctl -u "$SERVICE_NAME" -n 50 --no-pager || true
      die "${VERSION} is installed at ${BIN}, but ${SERVICE_NAME} did not become ready"
    fi
  fi
  log "${SERVICE_NAME} is ready"
else
  log "restart your openflare-edge process manually"
fi

log "updated to ${VERSION} at ${BIN}"
