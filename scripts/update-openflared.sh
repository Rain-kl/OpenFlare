#!/usr/bin/env bash
set -euo pipefail

VERSION=""
CHANNEL=stable
REPO="Rain-kl/OpenFlare"
INSTALL_DIR="/opt/openflared"
STATE_DIR="/var/lib/openflared"
SERVICE_NAME="openflared"
SERVICE_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
INSTALL_DIR_EXPLICIT=false
STATE_DIR_EXPLICIT=false

log() { printf '[openflared] %s\n' "$*"; }
die() { printf '[openflared] ERROR: %s\n' "$*" >&2; exit 1; }
is_release_tag() { [[ "$1" =~ ^tunnel-v[0-9]+(\.[0-9]+){2}(-[0-9A-Za-z.-]+)?$ ]]; }

canary_releases() {
  awk -F '"' '
    /^  \{$/ { tag = draft = prerelease = published = "" }
    /^    "tag_name": / { tag = $4 }
    /^    "draft": false,?$/ { draft = "false" }
    /^    "prerelease": true,?$/ { prerelease = "true" }
    /^    "published_at": / { published = $4 }
    /^  \},?$/ {
      if (tag ~ /^tunnel-v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$/ &&
          draft == "false" && prerelease == "true" && published != "") {
        print published, tag
      }
    }
  '
}

usage() {
  cat <<'EOF'
OpenFlared Connector Updater

Usage:
  update-openflared.sh [canary] [OPTIONS]

Updates only the installed openflared binary without modifying service
configuration, tokens, or identity state.

Options:
  canary              Update to the most recently published Tunnel prerelease
  --version TAG       Update to a specific tunnel-v* tag (default: latest stable)
  --repo OWNER/NAME   Release repository (default: Rain-kl/OpenFlare)
  --install-dir DIR   Override install directory (auto-detected from systemd)
  --state-dir DIR     Override state directory (auto-detected from systemd)
  -h, --help          Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    canary) CHANNEL=canary; shift ;;
    --version) VERSION="$2"; shift 2 ;;
    --repo) REPO="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; INSTALL_DIR_EXPLICIT=true; shift 2 ;;
    --state-dir) STATE_DIR="$2"; STATE_DIR_EXPLICIT=true; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done
[[ "$CHANNEL" != canary || -z "$VERSION" ]] || die "canary and --version are mutually exclusive"

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
  die "openflared is not installed in ${INSTALL_DIR}; run install-openflared.sh first"

HAS_SYSTEMD_SERVICE=false
if [[ -f "$SERVICE_PATH" ]] && command -v systemctl >/dev/null 2>&1; then
  HAS_SYSTEMD_SERVICE=true
fi

if [[ -z "$VERSION" ]]; then
  log "resolving latest ${CHANNEL} Tunnel release from ${REPO}"
  latest_canary=""
  for ((page = 1; ; page++)); do
    releases_json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=100&page=${page}")"
    mapfile -t release_tags < <(printf '%s' "$releases_json" | sed -nE 's/^[[:space:]]*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p')
    if [[ "$CHANNEL" == canary ]]; then
      candidate="$(printf '%s' "$releases_json" | canary_releases | LC_ALL=C sort -r | sed -n '1p')"
      if [[ "$candidate" > "$latest_canary" ]]; then
        latest_canary="$candidate"
        VERSION="${candidate#* }"
      fi
    else
      VERSION="$(printf '%s\n' "${release_tags[@]}" | grep -E '^tunnel-v[0-9]+(\.[0-9]+){2}$' | sort -Vr | sed -n '1p' || true)"
      if [[ -n "$VERSION" ]]; then break; fi
    fi
    if [[ ${#release_tags[@]} -lt 100 ]]; then break; fi
  done
  [[ -n "$VERSION" ]] || die "no ${CHANNEL} Tunnel release found in ${REPO}"
fi
is_release_tag "$VERSION" || die "invalid Tunnel release version"
RELEASE_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/tags/${VERSION}")"
VERSION="$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
is_release_tag "$VERSION" || die "invalid Tunnel release version"

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

TMP_DIR="$(mktemp -d "/tmp/openflared-update.XXXXXX")"
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

exec 9>"$INSTALL_DIR/.install.lock"
flock -n 9 || die "another OpenFlared installation or update holds $INSTALL_DIR/.install.lock"
STAGED_BIN="$INSTALL_DIR/.openflared.new"
install -m 0755 "$PACKAGE_DIR/openflared" "$STAGED_BIN"

if [[ "$HAS_SYSTEMD_SERVICE" == true ]]; then
  log "stopping ${SERVICE_NAME}"
  systemctl stop "$SERVICE_NAME" 9>&- || die "could not stop ${SERVICE_NAME}"
fi

log "replacing ${BIN} with ${VERSION}"
mv -Tf -- "$STAGED_BIN" "$BIN"
rm -f -- "$STATE_DIR/openflared-next" "$STATE_DIR/openflared-trial" "$STATE_DIR/openflared-active"

if [[ "$HAS_SYSTEMD_SERVICE" == true ]]; then
  log "starting ${SERVICE_NAME}"
  if ! systemctl start "$SERVICE_NAME" 9>&-; then
    journalctl -u "$SERVICE_NAME" -n 50 --no-pager 9>&- || true
    die "${VERSION} is installed at ${BIN}, but ${SERVICE_NAME} did not start"
  fi
else
  log "restart your openflared process manually"
fi

log "updated to ${VERSION} at ${BIN}"
