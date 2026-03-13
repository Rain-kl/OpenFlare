# ATSFlare 部署说明

本文档仅保留当前可用基线的最小部署方式，并补充第五版（0.5.x）开发期间与 OpenResty 主配置接管相关的联调约束。

---

## 1. 前置条件

### 1.1 Server

* Go 1.23+
* Node.js 18+
* 可写 SQLite 文件目录

### 1.2 Agent

* Go 1.23+
* 对 Agent 数据目录有写权限
* 若使用独立 OpenResty 模式：可执行 `openresty -t` 与 `openresty -s reload`
* 若使用 Docker 模式：具备 Docker 执行权限
* 第五版主配置接管模式下，Agent 对 OpenResty 主配置目标路径必须具备写权限

---

## 2. Server 启动

### 2.1 构建前端

```bash
cd atsf_server/web
corepack enable
pnpm install
pnpm build
```

说明：

* 前端使用 Next.js 静态导出模式构建
* `pnpm build` 会生成供 Go Server 托管的 `atsf_server/web/build` 目录
* 如需覆盖默认接口地址，可在构建前设置 `NEXT_PUBLIC_API_BASE_URL`

### 2.2 启动服务

```bash
cd atsf_server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./atsflare.db'
export LOG_LEVEL='info'
go run .
```

说明：

* 默认不依赖全局 `AGENT_TOKEN`
* 节点接入凭证由数据库维护：节点专属 `agent_token` + 全局 `discovery_token`
* 默认监听端口为 `3000`
* `LOG_LEVEL` 支持 `debug` / `info` / `warn` / `error`

### 2.3 使用 docker-compose 启动 Server

适用于直接使用已发布的 Server 镜像部署控制面。

示例 `docker-compose.yml`：

```yaml
services:
  atsflare:
    image: ghcr.io/rain-kl/atsflare:latest
    container_name: atsflare
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      SESSION_SECRET: replace-with-random-string
      SQLITE_PATH: /data/atsflare.db
      GIN_MODE: release
    volumes:
      - atsflare-data:/data

volumes:
  atsflare-data:
```

启动命令：

```bash
docker compose up -d
```

说明：

* `SESSION_SECRET` 必须替换为随机字符串
* SQLite 数据文件持久化到 Docker volume `atsflare-data`
* 镜像默认监听容器内 `3000` 端口
* 若要固定版本，可将 `latest` 替换为具体 tag，例如 `ghcr.io/rain-kl/atsflare:v0.3.0`

版本升级说明：

* Root 用户可在管理端顶栏点击「版本」默认检查正式版 GitHub Release
* 若需尝试 preview 版本，可在同一弹窗中手动检查 preview 发布并选择是否升级
* 当前运行的是 Release 二进制且二进制目录可写时，可直接在弹窗内触发 Server 自升级
* Server 自升级会下载匹配当前平台的 `atsflare-server-*` 资产，替换当前二进制并自动重启进程
* 也可在同一弹窗中手动上传 Server 二进制，服务端先检测上传文件版本，前端确认后再执行替换与重启
* 节点 Agent 默认仅跟随正式版自动更新；如需 preview 版本，可在节点详情中手动检查 preview 发布并下发更新

### 2.4 首次登录

访问 `http://localhost:3000`

默认账号：

* 用户名：`root`
* 密码：`123456`

### 2.5 Swagger 文档使用

登录管理端后，访问：`http://localhost:3000/swagger/index.html`

使用说明：

* Swagger UI 受管理端登录态保护，未登录不可直接访问
* 可在浏览器中查看当前 Server API 与 Agent API 定义，并直接发起调试请求
* 当 Server API 新增或变更时，需要同步更新 Swag 注解并重新生成 `atsf_server/docs`

如需在本地重新生成 Swagger 文档，先安装 `swag`：

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

安装后请确保 Go 的二进制目录已加入 `PATH`，常见目录为：

* Linux / macOS：`$HOME/go/bin`
* Windows：`%USERPROFILE%\go\bin`

如需在本地重新生成 Swagger 文档，可在 `atsf_server` 目录执行：

```bash
swag init -g main.go -o docs
```

---

## 3. Agent 配置

当前支持两种接入模式。

### 3.1 节点专属 `agent_token`

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "data_dir": "./data",
  "openresty_container_name": "atsflare-openresty",
  "openresty_docker_image": "openresty/openresty:alpine",
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

### 3.2 全局 `discovery_token`

```json
{
  "server_url": "http://127.0.0.1:3000",
  "discovery_token": "replace-with-global-discovery-token",
  "data_dir": "./data",
  "openresty_container_name": "atsflare-openresty",
  "openresty_docker_image": "openresty/openresty:alpine",
  "heartbeat_interval": 10000,
  "request_timeout": 10000
}
```

说明：

* `agent_version` 由 Agent 代码内常量提供，升级时同步修改代码
* 为兼容现有 Agent / Server API，运行时版本仍通过 `openresty_version` 字段上报，但其值现在表示 OpenResty 版本
* 时间字段使用毫秒整数
* `agent_token` 与 `discovery_token` 至少填写一个
* 若 `agent_token` 为空且 `discovery_token` 存在，Agent 会自动注册并写回新的专属 `agent_token`
* `node_name` 与 `node_ip` 可省略，未填写时自动探测
* 未配置 `openresty_path` 时，默认使用 Docker OpenResty 容器

### 3.3 第五版新增部署约束

第五版开发完成后，OpenResty 主配置将进入 Agent 受管范围。部署与联调时应满足：

* 本机 OpenResty 模式需要为 Agent 显式提供主配置文件写入路径
* Docker OpenResty 模式需要保证主配置、路由配置和证书目录位于同一套受管挂载路径中
* 节点现存手工维护的主配置如继续保留，必须先迁移为 Server 渲染模板的等价配置，再切换到受管模式
* 主配置切换前必须预留回滚副本，并通过一次 `openresty -t` 失败演练验证回滚

---

## 4. Agent 启动

### 4.1 直接运行

```bash
cd atsf_agent
export LOG_LEVEL='info'
go run ./cmd/agent -config /path/to/agent.json
```

### 4.2 编译后二进制运行

```bash
cd atsf_agent
go build -o atsflare-agent ./cmd/agent
export LOG_LEVEL='info'
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

1. Agent 完成心跳
2. 自动注册模式下完成 Token 置换
3. 心跳响应返回激活版本摘要；若版本或 checksum 不一致，则拉取激活版本
4. 写入主配置、路由配置与必要证书文件
5. 执行 `openresty -t`
6. 执行 `openresty -s reload`
7. 上报应用结果

### 5.3.1 Docker OpenResty 模式最小验证

建议使用最小 `agent.json`：

```json
{
  "server_url": "http://127.0.0.1:3000",
  "discovery_token": "replace-with-global-discovery-token",
  "data_dir": "./data",
  "openresty_container_name": "atsflare-openresty",
  "openresty_docker_image": "openresty/openresty:alpine"
}
```

验证点：

1. 首次启动后确认 `data/etc/nginx/nginx.conf`、`data/etc/nginx/conf.d/atsflare_routes.conf` 与 `data/etc/nginx/certs` 已由 Agent 创建
2. 确认容器实际挂载了主配置、路由目录和证书目录
3. 在管理端发布一次新版本后，确认节点 `current_version` 追平激活版本
4. 在节点详情查看“当前目标版本”与“最近应用”，确认主配置/路由配置快照和 checksum 已可见

推荐检查命令：

```bash
docker inspect atsflare-openresty
docker exec atsflare-openresty openresty -t
```

说明：

* `docker inspect` 重点确认主配置文件、`conf.d` 目录和证书目录都来自 Agent 受管路径
* 若容器名使用默认值，请将上述命令中的名称替换为 `atsflare-openresty`

### 5.3.2 本机 OpenResty 模式最小验证

建议显式提供以下路径：

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "openresty_path": "/usr/local/openresty/nginx/sbin/openresty",
  "main_config_path": "/usr/local/openresty/nginx/conf/nginx.conf",
  "route_config_path": "/usr/local/openresty/nginx/conf/conf.d/atsflare_routes.conf",
  "cert_dir": "/usr/local/openresty/nginx/conf/certs",
  "openresty_cert_dir": "/usr/local/openresty/nginx/conf/certs"
}
```

验证点：

1. 发布前先备份 `main_config_path` 与 `route_config_path`
2. 首次发布后执行 `openresty -t`，确认主配置已由 Server 模板接管且 include 指向 Agent 写入的路由文件
3. 再次发布修改后的规则或 OpenResty 参数，确认 `openresty -s reload` 成功且节点版本更新
4. 在节点详情与应用记录页确认主配置 checksum、路由配置 checksum 和支持文件数已上报

### 5.4 验证管理端状态

管理端应能看到：

* 节点在线状态
* 节点当前版本
* 最近一次应用结果
* 自动注册后节点已绑定专属 `agent_token`

### 5.5 验证失败回滚

人为制造 `openresty -t` 失败后再次发布，预期：

* Agent 回滚旧配置
* 主配置与路由配置一起回滚
* 节点 `last_error` 更新
* 应用记录中出现失败记录

### 5.5.1 建议的失败演练方式

建议只在测试节点进行，避免直接污染生产节点。

本机 OpenResty 模式：

1. 先备份当前 `agent.json`
2. 将 `openresty_path` 临时改为一个包装脚本，在收到 `-t` 时固定返回非零，其余参数转发到真实 `openresty`
3. 触发一次新版本发布，确认应用失败、主配置与路由配置回滚、节点 `last_error` 更新
4. 恢复真实 `openresty_path` 后再次发布，确认节点重新追平版本

Docker OpenResty 模式：

1. 在测试节点上保留默认受管路径
2. 临时将 `openresty_docker_image` 指向一个不包含 `openresty` 运行时的错误镜像标签，或在测试环境用包装镜像让 `openresty -t` 固定失败
3. 再次发布，确认节点应用失败但 `data/etc/nginx/nginx.conf` 与 `data/etc/nginx/conf.d/atsflare_routes.conf` 已回滚为旧版本
4. 恢复正确镜像后重新发布，确认节点恢复健康

说明：

* 第五版的失败演练重点不在“如何制造错误”，而在确认失败后旧主配置、旧路由配置和支持文件都会被一起恢复
* 若不方便做真实环境演练，至少应运行 Agent 回归测试，覆盖主配置写入、Docker 挂载与失败回滚

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
pnpm build
```

### 6.4 发布工作流

当前仓库维护两套独立的 Release 工作流：

* GitHub 使用 [.github/workflows/release.yml](.github/workflows/release.yml)，保留制品上传/下载分阶段流程
* Gitea 使用 [.gitea/workflows/release.yml](.gitea/workflows/release.yml)，在单个 Job 内完成前端构建、服务端/Agent 多平台编译与 Release 发布，避免依赖 Gitea 目前不兼容的 `upload-artifact@v4`、`download-artifact@v4`

Docker 镜像发布使用 [.github/workflows/docker-image.yml](.github/workflows/docker-image.yml)：

* 仅构建 `atsf_server` 服务端镜像
* 发布到 GitHub Container Registry（`ghcr.io/<owner>/<repo>:<tag>`）
* 单个工作流通过分架构原生构建再合并 manifest 的方式产出 `linux/amd64` 与 `linux/arm64` 多架构镜像，避免 `arm64` 长时间模拟编译

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
| NodeOfflineThreshold   | 节点离线阈值（毫秒） | -                  |
| AgentUpdateRepo        | 自动更新仓库         | update_repo        |

节点管理页下发的字段：

| 参数                   | 说明               | Agent 字段  |
| ---------------------- | ------------------ | ----------- |
| Node.AutoUpdateEnabled | 节点是否自动更新   | auto_update |
| Node.UpdateRequested   | 节点一次性更新请求 | update_now  |

---

## 10. 第五版性能优化联调重点

第五版联调时，至少补以下验证：

* 调整连接类参数后，能成功生成新版本并由 Agent 应用
* 启用或关闭代理缓存后，主配置预览、diff 与实际落盘一致
* 缓存目录、缓存大小、失效时间等参数非法时，Server 拒绝保存
* 主配置渲染异常或 `openresty -t` 失败时，Agent 不应留下半更新状态
* 本机模式与 Docker 模式都要验证一次主配置接管

---

## 11. 当前已知限制

* Docker 模式仍是 MVP 级封装
* 联调以手工步骤为主

---

## 12. 文档维护要求

当部署方式、配置字段、节点接入方式或联调流程变化时，同步更新本文档。
