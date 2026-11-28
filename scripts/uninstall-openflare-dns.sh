#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/opt/openflare-dns"
STATE_DIR="/var/lib/openflare-dns"
SERVICE_NAME="openflare-dns"
REMOVE_STATE=false

log() { printf '[openflare-dns] %s\n' "$*"; }
usage() {
  cat <<'EOF'
OpenFlare SmartDNS Uninstaller

Usage:
  uninstall-openflare-dns.sh [OPTIONS]

Options:
  --install-dir DIR   Default: /opt/openflare-dns
  --state-dir DIR     Default: /var/lib/openflare-dns
  --remove-state      Also remove identity, cached snapshots, and GeoIP data
  -h, --help          Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --state-dir) STATE_DIR="$2"; shift 2 ;;
    --remove-state) REMOVE_STATE=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'Unknown option: %s\n' "$1" >&2; exit 1 ;;
  esac
done

[[ "$INSTALL_DIR" != "/" && "$INSTALL_DIR" != "." ]] || { printf 'Refusing unsafe install directory\n' >&2; exit 1; }
[[ "$STATE_DIR" != "/" && "$STATE_DIR" != "." ]] || { printf 'Refusing unsafe state directory\n' >&2; exit 1; }

if command -v systemctl >/dev/null 2>&1; then
  if [[ -f /etc/systemd/system/openflare-dns-update.path ]]; then
    systemctl disable --now openflare-dns-update.path
    systemctl stop openflare-dns-update.service
  fi
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    log "stopping $SERVICE_NAME"
    systemctl stop "$SERVICE_NAME"
  fi
  if systemctl is-enabled --quiet "$SERVICE_NAME" >/dev/null 2>&1; then
    systemctl disable "$SERVICE_NAME" >/dev/null 2>&1 || true
  fi
fi
rm -f "/etc/systemd/system/${SERVICE_NAME}.service" \
  "/etc/systemd/system/${SERVICE_NAME}.service.d/remote-update.conf" \
  /etc/systemd/system/openflare-dns-update.path /etc/systemd/system/openflare-dns-update.service
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl reset-failed "$SERVICE_NAME" >/dev/null 2>&1 || true
fi
rm -rf "$INSTALL_DIR"
if [[ "$REMOVE_STATE" == true ]]; then
  rm -rf "$STATE_DIR"
fi
log "uninstalled openflare-dns"
