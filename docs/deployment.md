# ATSFlare 部署说明

本文档仅保留当前可用基线的最小部署方式，用于第三版开发前后的本地部署、联调与回归验证。

---

## 1. 前置条件

### 1.1 Server

* Go 1.18+
* Node.js 18+
* 可写 SQLite 文件目录

### 1.2 Agent

* Go 1.18+
* 对 Agent 数据目录有写权限
* 若使用独立 Nginx 模式：可执行 `nginx -t` 与 `nginx -s reload`
* 若使用 Docker 模式：具备 Docker 执行权限

---

## 2. Server 启动

### 2.1 构建前端

```bash
cd atsf_server/web
npm install
npm run build
```

### 2.2 启动服务

```bash
cd atsf_server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./atsflare.db'
go run .
```

说明：

* 默认不依赖全局 `AGENT_TOKEN`
* 节点接入凭证由数据库维护：节点专属 `agent_token` + 全局 `discovery_token`
* 默认监听端口为 `3000`

### 2.3 首次登录

访问 `http://localhost:3000`

默认账号：

* 用户名：`root`
* 密码：`123456`

---

## 3. Agent 配置

当前支持两种接入模式。

### 3.1 节点专属 `agent_token`

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "./data",
  "nginx_container_name": "atsflare-nginx",
  "nginx_docker_image": "nginx:stable-alpine",
  "heartbeat_interval": 30000,
  "sync_interval": 30000,
  "request_timeout": 10000
}
```

### 3.2 全局 `discovery_token`

```json
{
  "server_url": "http://127.0.0.1:3000",
  "discovery_token": "replace-with-global-discovery-token",
  "data_dir": "./data",
  "nginx_container_name": "atsflare-nginx",
  "nginx_docker_image": "nginx:stable-alpine",
  "heartbeat_interval": 30000,
  "sync_interval": 30000,
  "request_timeout": 10000
}
```

说明：

* `agent_version` 由 Agent 代码内常量提供，升级时同步修改代码
* `nginx_version` 由 Agent 启动时执行命令自动探测
* 时间字段使用毫秒整数
* `agent_token` 与 `discovery_token` 至少填写一个
* 若 `agent_token` 为空且 `discovery_token` 存在，Agent 会自动注册并写回新的专属 `agent_token`
* `node_name` 与 `node_ip` 可省略，未填写时自动探测
* 未配置 `nginx_path` 时，默认使用 Docker Nginx 容器

---

## 4. Agent 启动

### 4.1 直接运行

```bash
cd atsf_agent
go run ./cmd/agent -config /path/to/agent.json
```

### 4.2 编译后二进制运行

```bash
cd atsf_agent
go build -o atsflare-agent ./cmd/agent
./atsflare-agent -config /path/to/agent.json
```

---

## 5. 最小联调步骤

### 5.1 准备节点接入

二选一：

* 在管理端预创建节点并复制专属 `agent_token`
* 在管理端查看全局 `discovery_token` 并写入节点配置

### 5.2 创建规则并发布

1. 在管理端新增一条启用中的反代规则
2. 在发布前查看预览或变更摘要
3. 生成并激活新版本

### 5.3 验证 Agent 应用

预期行为：

1. Agent 完成心跳与同步
2. 自动注册模式下完成 Token 置换
3. 拉取激活版本
4. 写入路由配置与必要证书文件
5. 执行 `nginx -t`
6. 执行 `nginx -s reload`
7. 上报应用结果

### 5.4 验证管理端状态

管理端应能看到：

* 节点在线状态
* 节点当前版本
* 最近一次应用结果
* 自动注册后节点已绑定专属 `agent_token`

### 5.5 验证失败回滚

人为制造 `nginx -t` 失败后再次发布，预期：

* Agent 回滚旧配置
* 节点 `last_error` 更新
* 应用记录中出现失败记录

---

## 6. 常用验证命令

### 6.1 Server

```bash
cd atsf_server
GOCACHE=/tmp/atsflare-go-cache go test ./...
```

### 6.2 Agent

```bash
cd atsf_agent
GOCACHE=/tmp/atsflare-go-cache go test ./...
```

### 6.3 前端

```bash
cd atsf_server/web
npm run build
```

### 6.4 发布工作流

当前仓库维护两套独立的 Release 工作流：

* GitHub 使用 [.github/workflows/release.yml](.github/workflows/release.yml)，保留制品上传/下载分阶段流程
* Gitea 使用 [.gitea/workflows/release.yml](.gitea/workflows/release.yml)，在单个 Job 内完成前端构建、服务端/Agent 多平台编译与 Release 发布，避免依赖 Gitea 目前不兼容的 `upload-artifact@v4`、`download-artifact@v4`

Docker 镜像发布使用 [.github/workflows/docker-image.yml](.github/workflows/docker-image.yml)：

* 仅构建 `atsf_server` 服务端镜像
* 发布到 GitHub Container Registry（`ghcr.io/<owner>/<repo>:<tag>`）
* 单个工作流同时产出 `linux/amd64` 与 `linux/arm64` 多架构镜像

---

## 7. Agent 一键部署（V3）

### 7.1 curl 安装

在目标机器上运行：

```bash
curl -fsSL https://raw.githubusercontent.com/Rain-kl/ATSFlare/main/scripts/install-agent.sh | bash -s -- \
  --server-url http://your-server:3000 \
  --discovery-token YOUR_DISCOVERY_TOKEN
```

支持参数：

| 参数                | 说明                 | 默认值                |
| ------------------- | -------------------- | --------------------- |
| `--server-url`      | Server 地址（必填）  | -                     |
| `--discovery-token` | 全局 Discovery Token | -                     |
| `--agent-token`     | 节点专属 Token       | -                     |
| `--install-dir`     | 安装目录             | `/opt/atsflare-agent` |
| `--repo`            | GitHub Release 仓库  | `Rain-kl/ATSFlare`    |
| `--no-service`      | 不创建 systemd 服务  | -                     |

安装脚本会：

1. 从 GitHub Releases 下载最新 Agent 二进制（`atsflare-agent-{os}-{arch}`）
2. 先下载到临时文件，再替换安装目录中的 Agent 二进制，避免覆盖运行中进程导致写入失败
3. 生成 `agent.json` 配置文件
4. 创建 systemd 服务 `atsflare-agent.service`
5. 启动并启用自启

说明：

* 脚本可重复执行，用于升级到最新 Release
* 若检测到已运行的 `atsflare-agent` systemd 服务，会先停止服务、替换二进制，再重新启动
* 已存在的 `agent.json` 不会被覆盖

### 7.2 管理端生成部署命令

在管理端 **系统设置 → 运维设置** 中查看已生成的一键部署命令，直接复制到目标节点执行。

### 7.3 Agent 自动更新

Agent 自动更新默认为关闭。

在管理端 **节点管理** 页面中可以：

* 为单个节点开启「自动更新」
* 为单个节点点击「立即更新」，下发一次性更新指令

节点在收到对应心跳响应后，会检查 GitHub Releases，发现新版本时自动下载并重启。

---

## 8. Agent 二进制命名规则

GitHub Release 中的 Agent 二进制命名格式：

* `atsflare-agent-linux-amd64`
* `atsflare-agent-linux-arm64`
* `atsflare-agent-darwin-arm64`

---

## 9. 运维设置热更新（V3）

以下参数可通过管理端 **运维设置** 修改，修改后通过心跳响应下发到 Agent，无需重启：

| 参数                   | 说明                 | Agent 字段         |
| ---------------------- | -------------------- | ------------------ |
| AgentHeartbeatInterval | 心跳间隔（毫秒）     | heartbeat_interval |
| AgentSyncInterval      | 同步间隔（毫秒）     | sync_interval      |
| NodeOfflineThreshold   | 节点离线阈值（毫秒） | -                  |
| AgentUpdateRepo        | 自动更新仓库         | update_repo        |

节点管理页下发的字段：

| 参数                   | 说明               | Agent 字段  |
| ---------------------- | ------------------ | ----------- |
| Node.AutoUpdateEnabled | 节点是否自动更新   | auto_update |
| Node.UpdateRequested   | 节点一次性更新请求 | update_now  |

---

## 10. 当前已知限制

* Docker 模式仍是 MVP 级封装
* 联调以手工步骤为主

---

## 8. 文档维护要求

当部署方式、配置字段、节点接入方式或联调流程变化时，同步更新本文档。
