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

RELEASES_DIR="${INSTALL_DIR}/releases"
RELEASE_DIR="${RELEASES_DIR}/${VERSION}"
CURRENT="${INSTALL_DIR}/current"
VERIFIED_FILE="${INSTALL_DIR}/verified-releases"

PREVIOUS_TARGET="$(readlink "$CURRENT")" || die "current must be a symbolic link"

service_ready() {
  local version="$1" pid executable
  [[ -r "$STATE_DIR/ready.json" ]] || return 1
  systemctl is-active --quiet "$SERVICE_NAME" || return 1
  pid="$(systemctl show -p MainPID --value "$SERVICE_NAME")" || return 1
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || return 1
  executable="$(readlink -f -- "$RELEASES_DIR/$version/openflare-edge")" || return 1
  [[ "$(readlink "/proc/$pid/exe" 2>/dev/null)" == "$executable" ]] &&
    grep -Eq '"pid":'"$pid"'[,}]' "$STATE_DIR/ready.json" &&
    grep -Fq '"binary_version":"'"${version#edge-v}"'"' "$STATE_DIR/ready.json"
}

if [[ "$PREVIOUS_TARGET" == "releases/${VERSION}/openflare-edge" && -x "${RELEASE_DIR}/openflare-edge" ]]; then
  if [[ "$HAS_SYSTEMD_SERVICE" != true ]] || service_ready "$VERSION"; then
    log "already running ${VERSION}; nothing to update"
    exit 0
  fi
fi

ASSET="openflare-edge_${VERSION}_linux_${ARCH}.tar.gz"
DOWNLOAD_URL="$(printf '%s' "$RELEASE_JSON" | grep '"browser_download_url"' | grep "/${ASSET}\"" | sed -E 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/' | head -n1)"
[[ -n "$DOWNLOAD_URL" ]] || die "release asset not found: $ASSET"

TMP_DIR="$(mktemp -d "/tmp/openflare-edge-update.XXXXXX")"
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

exec 9>"$INSTALL_DIR/.install.lock"
flock -x 9

PREVIOUS_TARGET="$(readlink "$CURRENT")" || die "current must be a symbolic link"

installed_release() {
  is_release_tag "$1" && [[ -x "$RELEASES_DIR/$1/openflare-edge" ]]
}

switch_release() {
  ln -sfn -- "$1" "${INSTALL_DIR}/.current.tmp"
  mv -Tf "${INSTALL_DIR}/.current.tmp" "$CURRENT"
}

last_verified=""
previous_verified=""
rollback_target="$PREVIOUS_TARGET"
before="${PREVIOUS_TARGET#releases/}"
before="${before%/openflare-edge}"
if [[ "$PREVIOUS_TARGET" != "releases/$before/openflare-edge" ]]; then before=""; fi

if [[ "$HAS_SYSTEMD_SERVICE" == true ]]; then
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
  for candidate in "$before" "$last_verified" "$previous_verified"; do
    installed_release "$candidate" && service_ready "$candidate" || continue
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

if [[ "$HAS_SYSTEMD_SERVICE" == true ]]; then
  was_active=false
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    was_active=true
    service_action=restart
  else
    service_action=start
  fi

  switch_release "releases/$VERSION/openflare-edge"
  log "waiting for systemd readiness (startup timeout: 90 seconds)"
  if ! systemctl "$service_action" "$SERVICE_NAME" || ! service_ready "$VERSION"; then
    journalctl -u "$SERVICE_NAME" -n 50 --no-pager || true
    systemctl stop "$SERVICE_NAME" || die "could not stop the failed release"
    if [[ -n "$rollback_target" ]]; then switch_release "$rollback_target"; else rm -f -- "$CURRENT"; fi
    if [[ "$was_active" == true ]]; then
      systemctl reset-failed "$SERVICE_NAME" || true
      systemctl start "$SERVICE_NAME" || die "rollback service also failed; inspect journalctl -u ${SERVICE_NAME}"
    fi
    if [[ "$rollback_target" != "releases/$VERSION/openflare-edge" && "$VERSION" != "$last_verified" && "$VERSION" != "$previous_verified" ]]; then
      rm -rf -- "$RELEASE_DIR"
    fi
    die "update failed; previous installation restored"
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
  if [[ -n "$before" && "$before" != "$VERSION" ]]; then
    previous_verified="$before"
    for release in "$RELEASES_DIR"/edge-v*; do
      tag="${release##*/}"
      [[ -d "$release" && ! -L "$release" ]] && is_release_tag "$tag" || continue
      if [[ "$tag" != "$VERSION" && "$tag" != "$previous_verified" ]]; then rm -rf -- "$release"; fi
    done
  fi
  log "updated ${CURRENT} -> releases/${VERSION}/openflare-edge (restart your openflare-edge process manually)"
fi

log "updated to ${VERSION} in ${RELEASE_DIR}"
