#!/usr/bin/env bash
set -euo pipefail

# ATSFlare Agent Installer
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/scripts/install-agent.sh | bash -s -- \
#     --server-url http://your-server:3000 \
#     --discovery-token your-token

INSTALL_DIR="/opt/atsflare-agent"
REPO="Rain-kl/ATSFlare"
SERVER_URL=""
DISCOVERY_TOKEN=""
AGENT_TOKEN=""
CREATE_SERVICE="true"
SERVICE_NAME="atsflare-agent"

usage() {
  cat <<EOF
ATSFlare Agent Installer

Usage:
  install-agent.sh [OPTIONS]

Options:
  --server-url URL          Server URL (required)
  --discovery-token TOKEN   Discovery token for auto-registration
  --agent-token TOKEN       Node-specific agent token
  --install-dir DIR         Installation directory (default: /opt/atsflare-agent)
  --repo REPO               GitHub repository (default: Rain-kl/ATSFlare)
  --no-service              Do not create systemd service
  -h, --help                Show this help message

Examples:
  # Install with discovery token (auto-register)
  install-agent.sh --server-url http://10.0.0.1:3000 --discovery-token abc123

  # Install with node-specific token
  install-agent.sh --server-url http://10.0.0.1:3000 --agent-token node-token-xyz

Notes:
  Reinstall will remove the entire install directory before installing again,
  including the old agent.json, local state, cached data, and downloaded binary.
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-url)   SERVER_URL="$2"; shift 2 ;;
    --discovery-token) DISCOVERY_TOKEN="$2"; shift 2 ;;
    --agent-token)  AGENT_TOKEN="$2"; shift 2 ;;
    --install-dir)  INSTALL_DIR="$2"; shift 2 ;;
    --repo)         REPO="$2"; shift 2 ;;
    --no-service)   CREATE_SERVICE="false"; shift ;;
    -h|--help)      usage ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [[ -z "$SERVER_URL" ]]; then
  echo "Error: --server-url is required"
  exit 1
fi

if [[ -z "$DISCOVERY_TOKEN" && -z "$AGENT_TOKEN" ]]; then
  echo "Error: either --discovery-token or --agent-token is required"
  exit 1
fi

# Detect platform
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

if [[ "$OS" != "linux" && "$OS" != "darwin" ]]; then
  echo "Unsupported OS: $OS"
  exit 1
fi

ASSET_NAME="atsflare-agent-${OS}-${ARCH}"
echo "Detected platform: ${OS}/${ARCH}"

SYSTEMCTL_AVAILABLE="false"
if command -v systemctl >/dev/null 2>&1; then
  SYSTEMCTL_AVAILABLE="true"
fi

# Get latest release download URL
echo "Fetching latest release from ${REPO}..."
RELEASE_INFO=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")
DOWNLOAD_URL=$(echo "$RELEASE_INFO" | grep -o "\"browser_download_url\"[[:space:]]*:[[:space:]]*\"[^\"]*${ASSET_NAME}\"" | grep -o 'https://[^"]*' || true)

if [[ -z "$DOWNLOAD_URL" ]]; then
  echo "Error: no matching asset '${ASSET_NAME}' found in latest release"
  exit 1
fi

TAG=$(echo "$RELEASE_INFO" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | grep -o '"[^"]*"$' | tr -d '"')
echo "Latest release: ${TAG}"

# Download binary
echo "Downloading ${ASSET_NAME}..."
TMP_BINARY="$(mktemp "/tmp/atsflare-agent.tmp.XXXXXX")"
cleanup() {
  rm -f "$TMP_BINARY"
}
trap cleanup EXIT

curl -fsSL -o "$TMP_BINARY" "$DOWNLOAD_URL"
chmod +x "$TMP_BINARY"

SERVICE_WAS_ACTIVE="false"
if [[ "$OS" == "linux" && "$SYSTEMCTL_AVAILABLE" == "true" ]] && systemctl is-active --quiet "$SERVICE_NAME"; then
  SERVICE_WAS_ACTIVE="true"
  echo "Stopping running service before reinstall..."
  systemctl stop "$SERVICE_NAME"
fi

if [[ -d "$INSTALL_DIR" ]]; then
  echo "Removing existing installation directory: ${INSTALL_DIR}"
  rm -rf "$INSTALL_DIR"
fi

echo "Installing to ${INSTALL_DIR}..."
mkdir -p "${INSTALL_DIR}/data"

mv -f "$TMP_BINARY" "${INSTALL_DIR}/atsflare-agent"
trap - EXIT

# Generate config
CONFIG_FILE="${INSTALL_DIR}/agent.json"
echo "Generating agent.json..."
if [[ -n "$AGENT_TOKEN" ]]; then
  cat > "$CONFIG_FILE" <<CFGEOF
{
  "server_url": "${SERVER_URL}",
  "agent_token": "${AGENT_TOKEN}",
  "data_dir": "${INSTALL_DIR}/data",
  "heartbeat_interval": 30000,
  "sync_interval": 30000,
  "request_timeout": 10000
}
CFGEOF
else
  cat > "$CONFIG_FILE" <<CFGEOF
{
  "server_url": "${SERVER_URL}",
  "discovery_token": "${DISCOVERY_TOKEN}",
  "data_dir": "${INSTALL_DIR}/data",
  "heartbeat_interval": 30000,
  "sync_interval": 30000,
  "request_timeout": 10000
}
CFGEOF
fi

# Create systemd service
if [[ "$CREATE_SERVICE" == "true" && "$OS" == "linux" && -d /etc/systemd/system && "$SYSTEMCTL_AVAILABLE" == "true" ]]; then
  echo "Creating systemd service..."
  cat > /etc/systemd/system/atsflare-agent.service <<SVCEOF
[Unit]
Description=ATSFlare Agent
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/atsflare-agent -config ${CONFIG_FILE}
WorkingDirectory=${INSTALL_DIR}
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
SVCEOF

  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  systemctl start "$SERVICE_NAME"
  if [[ "$SERVICE_WAS_ACTIVE" == "true" ]]; then
    echo "Service restarted with updated binary: ${SERVICE_NAME}"
  else
    echo "Service created and started: ${SERVICE_NAME}"
  fi
else
  echo ""
  echo "To start the agent manually:"
  echo "  ${INSTALL_DIR}/atsflare-agent -config ${CONFIG_FILE}"
fi

echo ""
echo "ATSFlare Agent installed successfully!"
echo "  Binary: ${INSTALL_DIR}/atsflare-agent"
echo "  Config: ${CONFIG_FILE}"
echo "  Data:   ${INSTALL_DIR}/data"
