#!/usr/bin/env bash
set -euo pipefail

# OpenFlare Agent Installer
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Rain-kl/OpenFlare/main/scripts/install-agent.sh | bash -s -- \
#     --server-url http://your-server:3000 \
#     --discovery-token your-token

INSTALL_DIR="/opt/openflare-agent"
REPO="Rain-kl/OpenFlare"
SERVER_URL=""
DISCOVERY_TOKEN=""
AGENT_TOKEN=""
CREATE_SERVICE="true"
SERVICE_NAME="openflare-agent"
OPENRESTY_PATH=""
INSTALL_METHOD=""

CURRENT_DIR="$(pwd)"
LOG_FILE="${CURRENT_DIR}/install.log"

# Translation text variables
TXT_DOCKER_RESTARTED="Docker 已启动/重启并运行中"
TXT_LOW_DOCKER_VERSION="Docker 版本较低，建议升级到 20.10 及以上版本"
TXT_INSTALL_DOCKER_CONFIRM="未检测到 Docker，是否现在安装？(y/n) [y]: "
TXT_DOCKER_INSTALL_ONLINE="开始在线安装 Docker..."
TXT_INSTALL_DOCKER_ONLINE="使用 opkg 在线安装 Docker..."
TXT_CHOOSE_LOWEST_LATENCY_SOURCE="选择延迟最低的镜像源:"
TXT_CHOOSE_LOWEST_LATENCY_DELAY="延迟为:"
TXT_TRY_NEXT_LINK="尝试使用链接:"
TXT_DOWNLOAD_DOCKER_SCRIPT="下载 Docker 安装脚本..."
TXT_DOWNLOAD_DOCKER_SCRIPT_SUCCESS="下载成功:"
TXT_SUCCESSFULLY_MESSAGE="开始执行安装..."
TXT_DOWNLOAD_FAIELD="下载失败:"
TXT_ALL_DOWNLOAD_ATTEMPTS_FAILED="所有 Docker 下载尝试均失败，请手动安装 Docker"
TXT_DOCKER_INSTALL_FAIL="Docker 安装失败"
TXT_DOCKER_INSTALL_SUCCESS="Docker 安装成功"
TXT_CANNOT_SELECT_SOURCE="无法选择合适的镜像源"
TXT_REGIONS_OTHER_THAN_CHINA="检测到非中国大陆地区，使用官方源安装 Docker..."
TXT_DOCKER_START_NOTICE="正在启动 Docker 服务..."
TXT_CANCEL_INSTALL_DOCKER="已取消 Docker 安装"
TXT_INVALID_YN_INPUT="无效的输入，请输入 y 或 n"

log() {
    echo -e "[$(date +'%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

configure_accelerator() {
    log "配置 Docker 国内镜像加速源..."
    local docker_config_folder="/etc/docker"
    if [[ ! -d "$docker_config_folder" ]]; then
        mkdir -p "$docker_config_folder"
    fi
    cat > /etc/docker/daemon.json <<EOF
{
  "registry-mirrors": [
    "https://registry.docker-cn.com",
    "https://docker.mirrors.ustc.edu.cn",
    "https://hub-mirror.c.163.com",
    "https://mirror.baidubce.com"
  ]
}
EOF
    if command -v systemctl &>/dev/null; then
        systemctl daemon-reload
        systemctl restart docker
    else
        service dockerd restart
    fi
}

function Install_Docker(){
    if which docker >/dev/null 2>&1; then
        docker_version=$(docker --version | grep -oE '[0-9]+\.[0-9]+' | head -n 1)
        major_version=${docker_version%%.*}
        minor_version=${docker_version##*.}
        local service_cmd="service dockerd start && service dockerd status"
        if command -v systemctl &>/dev/null; then
            service_cmd="systemctl start docker && systemctl status docker"
        fi
        
        set +e
        local service_status=$($service_cmd 2>&1)
        set -e
        
        if [[ $service_status == *running* ]]; then
            log "$TXT_DOCKER_RESTARTED"
        else
            if [[ $major_version -lt 20 ]]; then
                log "$TXT_LOW_DOCKER_VERSION"
            fi

            set +e
            local country_code=$(curl -s --max-time 5 ipinfo.io/country)
            set -e
            if [[ "$country_code" == "CN" ]]; then
                configure_accelerator
            fi
        fi
    else
        while true; do
            read -p "$TXT_INSTALL_DOCKER_CONFIRM" install_docker_choice
            install_docker_choice=${install_docker_choice:-y}
            case "$install_docker_choice" in
                [yY])
                    log "$TXT_DOCKER_INSTALL_ONLINE"

                    if command -v opkg &>/dev/null; then
                        log "$TXT_INSTALL_DOCKER_ONLINE"
                        opkg update
                        opkg install luci-i18n-dockerman-zh-cn
                        opkg install zoneinfo-asia
                        service system restart
                        set +e
                        local country_code=$(curl -s --max-time 5 ipinfo.io/country)
                        set -e
                        if [[ "$country_code" == "CN" ]]; then
                            configure_accelerator
                        fi
                    else
                        set +e
                        local country_code=$(curl -s --max-time 5 ipinfo.io/country)
                        set -e
                        if [[ "$country_code" == "CN" ]]; then
                            sources=(
                                "https://mirrors.aliyun.com/docker-ce"
                                "https://mirrors.tencent.com/docker-ce"
                                "https://mirrors.163.com/docker-ce"
                                "https://mirrors.cernet.edu.cn/docker-ce"
                            )

                            docker_install_scripts=(
                                "https://get.docker.com"
                                "https://testingcf.jsdelivr.net/gh/docker/docker-install@master/install.sh"
                                "https://cdn.jsdelivr.net/gh/docker/docker-install@master/install.sh"
                                "https://fastly.jsdelivr.net/gh/docker/docker-install@master/install.sh"
                                "https://gcore.jsdelivr.net/gh/docker/docker-install@master/install.sh"
                                "https://raw.githubusercontent.com/docker/docker-install/master/install.sh"
                            )

                            get_average_delay() {
                                local source=$1
                                local total_delay=0
                                local iterations=2
                                local timeout=2

                                for ((i = 0; i < iterations; i++)); do
                                    delay=$(curl -o /dev/null -s -m $timeout -w "%{time_total}\n" "$source")
                                    if [ $? -ne 0 ]; then
                                        delay=$timeout
                                    fi
                                    total_delay=$(awk "BEGIN {print $total_delay + $delay}")
                                done

                                average_delay=$(awk "BEGIN {print $total_delay / $iterations}")
                                echo "$average_delay"
                            }

                            min_delay=99999999
                            selected_source=""

                            for source in "${sources[@]}"; do
                                average_delay=$(get_average_delay "$source" &)

                                if (( $(awk 'BEGIN { print '"$average_delay"' < '"$min_delay"' }') )); then
                                    min_delay=$average_delay
                                    selected_source=$source
                                fi
                            done
                            wait

                            if [ -n "$selected_source" ]; then
                                log "$TXT_CHOOSE_LOWEST_LATENCY_SOURCE $selected_source，$TXT_CHOOSE_LOWEST_LATENCY_DELAY $min_delay"
                                export DOWNLOAD_URL="$selected_source"

                                for alt_source in "${docker_install_scripts[@]}"; do
                                    log "$TXT_TRY_NEXT_LINK $alt_source $TXT_DOWNLOAD_DOCKER_SCRIPT"
                                    if curl -fsSL --retry 2 --retry-delay 3 --connect-timeout 5 --max-time 10 "$alt_source" -o get-docker.sh; then
                                        log "$TXT_DOWNLOAD_DOCKER_SCRIPT_SUCCESS $alt_source $TXT_SUCCESSFULLY_MESSAGE"
                                        break
                                    else
                                        log "$TXT_DOWNLOAD_FAIELD $alt_source $TXT_TRY_NEXT_LINK"
                                    fi
                                done

                                if [ ! -f "get-docker.sh" ]; then
                                    log "$TXT_ALL_DOWNLOAD_ATTEMPTS_FAILED"
                                    log "bash <(curl -sSL https://linuxmirrors.cn/docker.sh)"
                                    exit 1
                                fi

                                sh get-docker.sh 2>&1 | tee -a "${CURRENT_DIR}/install.log"

                                docker_config_folder="/etc/docker"
                                if [[ ! -d "$docker_config_folder" ]]; then
                                    mkdir -p "$docker_config_folder"
                                fi

                                set +e
                                docker version >/dev/null 2>&1
                                local docker_ver_res=$?
                                set -e
                                if [[ $docker_ver_res -ne 0 ]]; then
                                    log "$TXT_DOCKER_INSTALL_FAIL"
                                    exit 1
                                else
                                    log "$TXT_DOCKER_INSTALL_SUCCESS"
                                    if command -v systemctl &>/dev/null; then
                                        systemctl enable docker 2>&1 | tee -a "${LOG_FILE}"
                                    fi
                                    configure_accelerator
                                fi
                            else
                                log "$TXT_CANNOT_SELECT_SOURCE"
                                exit 1
                            fi
                        else
                            log "$TXT_REGIONS_OTHER_THAN_CHINA"
                            export DOWNLOAD_URL="https://download.docker.com"
                            curl -fsSL "https://get.docker.com" -o get-docker.sh
                            sh get-docker.sh 2>&1 | tee -a "${LOG_FILE}"

                            log "$TXT_DOCKER_START_NOTICE"
                            if command -v systemctl &>/dev/null; then
                                systemctl enable docker; systemctl daemon-reload; systemctl start docker 2>&1 | tee -a "${LOG_FILE}"
                            else
                                service dockerd start 2>&1 | tee -a "${LOG_FILE}"
                                sleep 1
                            fi

                            docker_config_folder="/etc/docker"
                            if [[ ! -d "$docker_config_folder" ]]; then
                                mkdir -p "$docker_config_folder"
                            fi

                            set +e
                            docker version >/dev/null 2>&1
                            local docker_ver_res=$?
                            set -e
                            if [[ $docker_ver_res -ne 0 ]]; then
                                log "$TXT_DOCKER_INSTALL_FAIL"
                                exit 1
                            else
                                log "$TXT_DOCKER_INSTALL_SUCCESS"
                            fi
                        fi
                    fi

                    break
                    ;;
                [nN])
                    echo "$TXT_CANCEL_INSTALL_DOCKER"
                    exit 1
                    ;;
                *)
                    log "$TXT_INVALID_YN_INPUT"
                    continue
                    ;;
            esac
        done
    fi
}

usage() {
  cat <<EOF
OpenFlare Agent Installer

Usage:
  install-agent.sh [OPTIONS]

Options:
  --server-url URL          Server URL
  --discovery-token TOKEN   Discovery token for auto-registration
  --agent-token TOKEN       Node-specific agent token
  --install-dir DIR         Installation directory (default: /opt/openflare-agent)
  --openresty-path PATH     OpenResty binary path (default: auto-detect from PATH)
  --repo REPO               GitHub repository (default: Rain-kl/OpenFlare)
  --no-service              Do not create systemd service
  --docker                  Install via Docker container
  --method METHOD           Installation method: 'local' or 'docker' (default: local)
  -h, --help                Show this help message

Examples:
  # Interactive installation (prompts for options)
  install-agent.sh

  # Automated local installation
  install-agent.sh --server-url http://10.0.0.1:3000 --discovery-token abc123

  # Automated Docker installation
  install-agent.sh --server-url http://10.0.0.1:3000 --discovery-token abc123 --docker
EOF
  exit 0
}

HAS_ARGS="false"
if [[ $# -gt 0 ]]; then
  HAS_ARGS="true"
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-url)   SERVER_URL="$2"; shift 2 ;;
    --discovery-token) DISCOVERY_TOKEN="$2"; shift 2 ;;
    --agent-token)  AGENT_TOKEN="$2"; shift 2 ;;
    --install-dir)  INSTALL_DIR="$2"; shift 2 ;;
    --openresty-path) OPENRESTY_PATH="$2"; shift 2 ;;
    --repo)         REPO="$2"; shift 2 ;;
    --no-service)   CREATE_SERVICE="false"; shift ;;
    --docker)       INSTALL_METHOD="docker"; shift ;;
    --method)       INSTALL_METHOD="$2"; shift 2 ;;
    -h|--help)      usage ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [[ "$HAS_ARGS" == "false" ]]; then
  echo "=================================================="
  echo "欢迎使用 OpenFlare Agent 安装脚本"
  echo "Welcome to the OpenFlare Agent Installer"
  echo "=================================================="
  echo "请选择安装方式 / Please choose installation method:"
  echo "  1) Local  (本地安装: 下载二进制运行，需要本地有 OpenResty)"
  echo "  2) Docker (容器安装: 运行内置 OpenResty 的 Agent 容器)"
  read -p "请输入序号 [1-2] (默认 1): " method_choice
  method_choice=${method_choice:-1}
  case "$method_choice" in
    2) INSTALL_METHOD="docker" ;;
    *) INSTALL_METHOD="local" ;;
  esac

  if [[ "$INSTALL_METHOD" == "docker" ]]; then
    Install_Docker
  fi

  # Prompt for Server URL
  while [[ -z "$SERVER_URL" ]]; do
    read -p "请输入 OpenFlare Server 地址 (例如 http://127.0.0.1:3000): " SERVER_URL
  done

  # Prompt for Token
  while [[ -z "$DISCOVERY_TOKEN" && -z "$AGENT_TOKEN" ]]; do
    echo "请选择认证 Token 类型 / Please choose token type:"
    echo "  1) Discovery Token (自动注册凭证 - 适合新节点上线)"
    echo "  2) Agent Token     (专属接入凭证 - 适合预创建节点)"
    read -p "请输入序号 [1-2] (默认 1): " token_type_choice
    token_type_choice=${token_type_choice:-1}
    if [[ "$token_type_choice" == "2" ]]; then
      read -p "请输入 Agent Token: " AGENT_TOKEN
    else
      read -p "请输入 Discovery Token: " DISCOVERY_TOKEN
    fi
  done
else
  # Non-interactive mode validation
  if [[ -z "$INSTALL_METHOD" ]]; then
    INSTALL_METHOD="local"
  fi

  if [[ -z "$SERVER_URL" ]]; then
    echo "Error: --server-url is required"
    exit 1
  fi

  if [[ -z "$DISCOVERY_TOKEN" && -z "$AGENT_TOKEN" ]]; then
    echo "Error: either --discovery-token or --agent-token is required"
    exit 1
  fi
fi

# Run Docker container installation if Docker method selected
if [[ "$INSTALL_METHOD" == "docker" ]]; then
  echo "拉取最新的 OpenFlare Agent 镜像..."
  docker pull ghcr.io/rain-kl/openflare-agent:latest

  echo "停止并移除旧的 openflare-agent 容器 (如果存在)..."
  docker rm -f openflare-agent 2>/dev/null || true

  echo "启动 openflare-agent 容器..."
  if [[ -n "$AGENT_TOKEN" ]]; then
    docker run -d --name openflare-agent --restart unless-stopped \
      -p 80:80 -p 443:443/tcp -p 443:443/udp \
      -v openflare-agent-pages:/data/var/lib/openflare/pages \
      -e OPENFLARE_SERVER_URL="${SERVER_URL}" \
      -e OPENFLARE_AGENT_TOKEN="${AGENT_TOKEN}" \
      ghcr.io/rain-kl/openflare-agent:latest
  else
    docker run -d --name openflare-agent --restart unless-stopped \
      -p 80:80 -p 443:443/tcp -p 443:443/udp \
      -v openflare-agent-pages:/data/var/lib/openflare/pages \
      -e OPENFLARE_SERVER_URL="${SERVER_URL}" \
      -e OPENFLARE_DISCOVERY_TOKEN="${DISCOVERY_TOKEN}" \
      ghcr.io/rain-kl/openflare-agent:latest
  fi

  echo ""
  echo "OpenFlare Agent (Docker) 安装成功!"
  echo "您可以执行以下命令查看运行日志:"
  echo "  docker logs -f openflare-agent"
  exit 0
fi

# Local installation starts below
if [[ -z "$OPENRESTY_PATH" ]]; then
  if command -v openresty >/dev/null 2>&1; then
    OPENRESTY_PATH="$(command -v openresty)"
  elif [[ "$HAS_ARGS" == "false" ]]; then
    read -p "未在 PATH 中检测到 openresty，请输入 OpenResty 二进制路径: " OPENRESTY_PATH
    if [[ -z "$OPENRESTY_PATH" ]]; then
      echo "Error: OpenResty binary path is required for local installation."
      exit 1
    fi
  else
    echo "Error: openresty was not found in PATH. Install OpenResty first or pass --openresty-path."
    exit 1
  fi
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

if [[ ! -x "$OPENRESTY_PATH" ]]; then
  echo "Error: OpenResty binary is not executable: ${OPENRESTY_PATH}"
  exit 1
fi

ASSET_NAME="openflare-agent-${OS}-${ARCH}"
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
TMP_BINARY="$(mktemp "/tmp/openflare-agent.tmp.XXXXXX")"
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

ensure_runtime_user() {
  if [[ "$OS" != "linux" ]]; then
    return
  fi
  if id openflare >/dev/null 2>&1; then
    return
  fi
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --home-dir "${INSTALL_DIR}/data" --shell /usr/sbin/nologin openflare
  fi
}

echo "Installing to ${INSTALL_DIR}..."
mkdir -p "${INSTALL_DIR}/data"
ensure_runtime_user

mv -f "$TMP_BINARY" "${INSTALL_DIR}/openflare-agent"
trap - EXIT

# Generate config
CONFIG_FILE="${INSTALL_DIR}/agent.json"
echo "Generating agent.json..."
if [[ -n "$AGENT_TOKEN" ]]; then
  cat > "$CONFIG_FILE" <<CFGEOF
{
  "server_url": "${SERVER_URL}",
  "agent_token": "${AGENT_TOKEN}",
  "openresty_path": "${OPENRESTY_PATH}",
  "data_dir": "${INSTALL_DIR}/data",
  "heartbeat_interval": 30000,
  "request_timeout": 10000
}
CFGEOF
else
  cat > "$CONFIG_FILE" <<CFGEOF
{
  "server_url": "${SERVER_URL}",
  "discovery_token": "${DISCOVERY_TOKEN}",
  "openresty_path": "${OPENRESTY_PATH}",
  "data_dir": "${INSTALL_DIR}/data",
  "heartbeat_interval": 30000,
  "request_timeout": 10000
}
CFGEOF
fi

if id openflare >/dev/null 2>&1; then
  chown -R openflare:openflare "${INSTALL_DIR}"
fi

# Create systemd service
if [[ "$CREATE_SERVICE" == "true" && "$OS" == "linux" && -d /etc/systemd/system && "$SYSTEMCTL_AVAILABLE" == "true" ]]; then
  echo "Creating systemd service..."
  cat > /etc/systemd/system/openflare-agent.service <<SVCEOF
[Unit]
Description=OpenFlare Agent
After=network.target

[Service]
Type=simple
User=openflare
Group=openflare
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ExecStart=${INSTALL_DIR}/openflare-agent -config ${CONFIG_FILE}
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
  echo "  ${INSTALL_DIR}/openflare-agent -config ${CONFIG_FILE}"
fi

echo ""
echo "OpenFlare Agent installed successfully!"
echo "  Binary: ${INSTALL_DIR}/openflare-agent"
echo "  Config: ${CONFIG_FILE}"
echo "  Data:   ${INSTALL_DIR}/data"
echo "  OpenResty: ${OPENRESTY_PATH}"
