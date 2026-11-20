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
  exec_start="$(sed -nE 's/^ExecStart=//p' "$SERVICE_PATH" | head -n1)"
  exec_bin="${exec_start%% *}"
  if [[ "$INSTALL_DIR_EXPLICIT" != true && "$exec_bin" == */current ]]; then
    INSTALL_DIR="${exec_bin%/current}"
  fi
  if [[ "$STATE_DIR_EXPLICIT" != true && "$exec_start" =~ [[:space:]]--state-dir[[:space:]]([^[:space:]]+) ]]; then
    STATE_DIR="${BASH_REMATCH[1]}"
  fi
fi

BIN="$INSTALL_DIR/current"
[[ -e "$BIN" || -L "$BIN" ]] ||
  die "openflare-edge is not installed in ${INSTALL_DIR}; run install-openflare-edge.sh first"

HAS_SYSTEMD_SERVICE=false
if [[ -f "$SERVICE_PATH" ]] && command -v systemctl >/dev/null 2>&1; then
  HAS_SYSTEMD_SERVICE=true
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

TMP_DIR="$(mktemp -d "/tmp/openflare-edge-update.XXXXXX")"
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

exec 9>"$INSTALL_DIR/.install.lock"
flock -n 9 || die "another Edge installation or update holds $INSTALL_DIR/.install.lock"
STAGED_BIN="$INSTALL_DIR/.openflare-edge.new"
install -m 0755 "$PACKAGE_DIR/openflare-edge" "$STAGED_BIN"

if [[ "$HAS_SYSTEMD_SERVICE" == true ]]; then
  log "stopping ${SERVICE_NAME}"
  systemctl stop "$SERVICE_NAME" 9>&- || die "could not stop ${SERVICE_NAME}"
fi

log "replacing ${BIN} with ${VERSION}"
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

if [[ "$HAS_SYSTEMD_SERVICE" == true ]]; then
  log "starting ${SERVICE_NAME}"
  if ! systemctl start "$SERVICE_NAME" 9>&-; then
    journalctl -u "$SERVICE_NAME" -n 50 --no-pager 9>&- || true
    die "${VERSION} is installed at ${BIN}, but ${SERVICE_NAME} did not start"
  fi
else
  log "restart your openflare-edge process manually"
fi

log "updated to ${VERSION} at ${BIN}"
