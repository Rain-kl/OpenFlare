#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="/opt/openflare-agent"
SERVICE_NAME="openflare-agent"
INSTALL_METHOD=""

usage() {
  cat <<EOF
OpenFlare Agent Uninstaller

Usage:
  uninstall-agent.sh [OPTIONS]

Options:
  --install-dir DIR         Installation directory (default: /opt/openflare-agent)
  --service-name NAME       systemd service name (default: openflare-agent)
  --docker                  Uninstall Docker container agent
  --method METHOD           Uninstall method: 'local' or 'docker' (default: local)
  -h, --help                Show this help message

Behavior:
  1. Stop the agent service/process and remove the entire installation directory
  2. Remove the systemd service definition when present
  3. Uninstall Docker container when --docker is specified
  4. Leave the local OpenResty installation untouched

Examples:
  # Interactive uninstall (prompts for options)
  uninstall-agent.sh

  # Automated local uninstall
  uninstall-agent.sh --install-dir /opt/openflare-agent

  # Automated Docker uninstall
  uninstall-agent.sh --docker
EOF
  exit 0
}

HAS_ARGS="false"
if [[ $# -gt 0 ]]; then
  HAS_ARGS="true"
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)  INSTALL_DIR="$2"; shift 2 ;;
    --service-name) SERVICE_NAME="$2"; shift 2 ;;
    --docker)       INSTALL_METHOD="docker"; shift ;;
    --method)       INSTALL_METHOD="$2"; shift 2 ;;
    -h|--help)      usage ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [[ "$HAS_ARGS" == "false" ]]; then
  echo "=================================================="
  echo "Welcome to the OpenFlare Agent Uninstaller"
  echo "=================================================="
  echo "Please choose an uninstall method:"
  echo "  1) Local  (stop the service and remove the binary and configuration)"
  echo "  2) Docker (stop and remove the Docker container)"
  read -p "Enter an option [1-2] (default: 1): " method_choice
  method_choice=${method_choice:-1}
  case "$method_choice" in
    2) INSTALL_METHOD="docker" ;;
    *) INSTALL_METHOD="local" ;;
  esac
else
  if [[ -z "$INSTALL_METHOD" ]]; then
    INSTALL_METHOD="local"
  fi
fi

if [[ "$INSTALL_METHOD" == "docker" ]]; then
  echo "Uninstalling OpenFlare Agent (Docker)..."

  # Check if container exists
  if command -v docker >/dev/null 2>&1; then
    if docker ps -a --format '{{.Names}}' | grep -Eq "^openflare-agent$"; then
      echo "Stopping and removing the openflare-agent container..."
      docker stop openflare-agent || true
      docker rm -f openflare-agent || true
      echo "The openflare-agent container was removed successfully."
    else
      echo "No openflare-agent container was found."
    fi

    # In interactive mode, ask if user wants to remove docker image
    if [[ "$HAS_ARGS" == "false" ]]; then
      read -p "Remove the OpenFlare Agent image (ghcr.io/rain-kl/openflare-agent:latest)? (y/n) [n]: " remove_image_choice
      remove_image_choice=${remove_image_choice:-n}
      case "$remove_image_choice" in
        [yY])
          echo "Removing the Docker image..."
          docker rmi ghcr.io/rain-kl/openflare-agent:latest || true
          ;;
        *)
          echo "Keeping the Docker image."
          ;;
      esac
    fi
  else
    echo "Error: Docker was not found; cannot uninstall a Docker deployment."
    exit 1
  fi

  echo "OpenFlare Agent (Docker) uninstalled."
  exit 0
fi

if [[ -z "$INSTALL_DIR" || "$INSTALL_DIR" == "/" || "$INSTALL_DIR" == "." ]]; then
  echo "Refusing to remove unsafe install directory: '${INSTALL_DIR}'"
  exit 1
fi

AGENT_BINARY="${INSTALL_DIR}/openflare-agent"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

SYSTEMCTL_AVAILABLE="false"
if command -v systemctl >/dev/null 2>&1; then
  SYSTEMCTL_AVAILABLE="true"
fi

echo "Uninstalling OpenFlare Agent from ${INSTALL_DIR}..."

if [[ "$SYSTEMCTL_AVAILABLE" == "true" ]]; then
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    echo "Stopping service: ${SERVICE_NAME}"
    systemctl stop "$SERVICE_NAME"
  fi

  if systemctl is-enabled --quiet "$SERVICE_NAME" >/dev/null 2>&1; then
    echo "Disabling service: ${SERVICE_NAME}"
    systemctl disable "$SERVICE_NAME" >/dev/null 2>&1 || true
  fi
fi

if command -v pgrep >/dev/null 2>&1; then
  mapfile -t agent_pids < <(pgrep -f "$AGENT_BINARY" || true)
  if (( ${#agent_pids[@]} > 0 )); then
    echo "Stopping agent process: ${agent_pids[*]}"
    kill "${agent_pids[@]}" || true
    sleep 1

    mapfile -t remaining_agent_pids < <(pgrep -f "$AGENT_BINARY" || true)
    if (( ${#remaining_agent_pids[@]} > 0 )); then
      echo "Force stopping remaining agent process: ${remaining_agent_pids[*]}"
      kill -9 "${remaining_agent_pids[@]}" || true
    fi
  fi
fi

if [[ -f "$SERVICE_FILE" ]]; then
  echo "Removing service file: ${SERVICE_FILE}"
  rm -f "$SERVICE_FILE"
fi

if [[ "$SYSTEMCTL_AVAILABLE" == "true" ]]; then
  systemctl daemon-reload || true
  systemctl reset-failed "$SERVICE_NAME" >/dev/null 2>&1 || true
fi

if [[ -d "$INSTALL_DIR" ]]; then
  echo "Removing installation directory: ${INSTALL_DIR}"
  rm -rf "$INSTALL_DIR"
else
  echo "Installation directory not found, skipping: ${INSTALL_DIR}"
fi

echo "Agent uninstall complete."
echo ""
echo "Local OpenResty was not modified. Remove it manually if you no longer need it."
echo "OpenFlare Agent uninstall finished."
