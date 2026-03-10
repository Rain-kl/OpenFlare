# ATSFlare 部署与联调说明

本文档用于指导在新环境手工完成 ATSFlare 当前版本的最小部署与联调。

当前文档已同步第二版 Phase 3 的节点接入方式：

- 预创建节点：管理端创建节点后，直接生成该节点专属 `auth token`（即 `agent_token`）
- 自动发现：管理端维护一个全局 `discovery token`，多个新节点可共用该 token 自动注册
- 自动发现注册成功后，Server 会为该节点下发新的专属 `agent_token`，Agent 自动完成本地 token 置换

## 1. 前置条件

### Server

- Go 1.18+
- Node.js 18+ 与 npm
- 本地可写 SQLite 文件目录

### Agent

- Go 1.18+
- Agent 对目标路由文件路径有写权限
- 如果使用独立 Nginx 模式：节点已安装 `nginx`，且 Agent 运行用户可以执行 `nginx -t` 和 `nginx -s reload`
- 如果使用 Docker 模式：节点已安装 Docker，且 Agent 运行用户有权限执行 Docker 命令

## 2. Server 启动

### 2.1 构建前端

```bash
cd atsf_server/web
npm install
npm run build
```

### 2.2 启动 Server

推荐在仓库根目录执行：

```bash
cd atsf_server
export SESSION_SECRET='replace-with-random-string'
export SQLITE_PATH='./atsflare.db'
go run .
```

说明：

- 如果未设置 `SQLITE_PATH`，默认也会落到 `atsf_server/atsflare.db`
- 当前不再依赖全局 `AGENT_TOKEN` 环境变量
- 节点接入凭证改由数据库保存：节点专属 `agent_token` + 系统级 `discovery token`
- 当前默认监听端口为 `3000`

### 2.3 首次登录

访问 `http://localhost:3000`

默认账号：

- 用户名：`root`
- 密码：`123456`

首次登录后建议立即修改密码。

## 3. Agent 启动

### 3.1 Agent 配置文件示例

Agent 现在支持两种接入模式。

#### 方式 A：使用预创建节点的专属 auth token

在节点上创建 `agent.json`：

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-node-auth-token",
  "agent_version": "0.1.0",
  "nginx_version": "1.25.5",
  "data_dir": "./data",
  "nginx_container_name": "atsflare-nginx",
  "nginx_docker_image": "nginx:stable-alpine",
  "heartbeat_interval": 30000000000,
  "sync_interval": 30000000000,
  "request_timeout": 10000000000
}
```

#### 方式 B：使用全局 discovery token 自动注册

```json
{
  "server_url": "http://127.0.0.1:3000",
  "discovery_token": "replace-with-global-discovery-token",
  "agent_version": "0.1.0",
  "nginx_version": "1.25.5",
  "data_dir": "./data",
  "nginx_container_name": "atsflare-nginx",
  "nginx_docker_image": "nginx:stable-alpine",
  "heartbeat_interval": 30000000000,
  "sync_interval": 30000000000,
  "request_timeout": 10000000000
}
```

注意：

- 时间字段单位是纳秒，因为当前实现直接使用 Go 的 `time.Duration` JSON 反序列化
- `agent_token` 与 `discovery_token` 至少填写一个
- 当填写节点专属 `agent_token` 时，Agent 会直接以该 Token 进行心跳、拉取配置和上报
- 当 `agent_token` 为空且填写 `discovery_token` 时，Agent 会自动注册，注册成功后会把新的专属 `agent_token` 写回本地配置文件，并清空 `discovery_token`
- `node_name` 与 `node_ip` 现在可省略；未填写时会自动探测主机名和本机 IPv4 地址，手动填写则视为覆盖
- 生成资源默认统一落在 `./data`
- 如果未指定 `nginx_path`，Agent 会自动使用以下固定路径：
  - `./data/etc/nginx/conf.d/atsflare_routes.conf`
  - `./data/var/lib/atsflare/agent-state.json`
- 如果希望修改保存位置，可通过 `data_dir` 统一覆盖生成资源目录
- Docker 模式默认使用 `nginx_container_name` 和 `nginx_docker_image`
- `nginx_path` 仅在独立 Nginx 路径模式下使用；此时如有需要，仍可单独覆盖 `route_config_path` 和 `state_path`

### 3.2 获取接入 Token

#### 方式 A：预创建节点

1. 登录管理端
2. 打开“节点”页面
3. 点击“新增节点”
4. 创建成功后，页面会显示该节点的专属 `Auth Token`
5. 将该 token 写入对应节点的 `agent.json` 中的 `agent_token`

适用场景：

- 固定节点、固定槽位管理
- 需要明确一台机器占据哪一个节点位

#### 方式 B：全局自动发现

1. 登录管理端
2. 打开“节点”页面
3. 查看页面顶部的全局 `Discovery Token`
4. 将同一个 token 分发给多台待接入节点，写入各自 `agent.json` 中的 `discovery_token`

适用场景：

- 批量部署 Agent
- 节点数量较多，不希望逐台预生成 token

### 3.3 启动 Agent

```bash
cd atsf_agent
go run ./cmd/agent -config /path/to/agent.json
```

如果需要编译二进制：

```bash
cd atsf_agent
go build -o atsflare-agent ./cmd/agent
./atsflare-agent -config /path/to/agent.json
```

## 4. 最小联调步骤

以下步骤用于验证完整闭环。

### 4.1 创建节点或准备 discovery token

二选一：

#### 方案 A：预创建节点

1. 登录管理端
2. 打开“节点”页面
3. 新增一个节点，例如：`edge-01`
4. 复制该节点展示的 `Auth Token`
5. 在目标机器的 `agent.json` 中填入该 `agent_token`

#### 方案 B：自动发现

1. 登录管理端
2. 打开“节点”页面
3. 复制全局 `Discovery Token`
4. 在目标机器的 `agent.json` 中填入该 `discovery_token`

### 4.2 创建规则

1. 登录管理端
2. 打开“规则”页面
3. 新增一条反代规则，例如：
   - 域名：`demo.example.com`
   - 源站：`http://127.0.0.1:8080`
   - 启用：开启

### 4.3 发布版本

1. 在“规则”页面点击“发布当前规则”
2. 或在“版本”页面点击“生成新版本”
3. 确认“版本”页面出现新的激活版本

### 4.4 验证 Agent 拉取与应用

启动 Agent 后，预期行为如下：

1. 如果配置的是节点专属 `agent_token`：Agent 直接进入心跳与同步流程
2. 如果配置的是全局 `discovery_token`：Agent 先自动注册，拿到新的专属 `agent_token` 并写回本地配置
3. Agent 拉取当前激活版本
4. Agent 写入 `route_config_path`
5. Agent 使用 `nginx_path` 指向的独立 Nginx，或自动准备 Docker Nginx 容器
6. Agent 启动时先校验本地路由文件 checksum 与控制面激活版本是否一致
7. Docker 模式下会重建容器，避免复用故障容器
8. Agent 执行 `nginx -t`
9. Agent 执行 `nginx -s reload`
10. Agent 上报成功结果

### 4.5 验证管理端状态

在管理端确认：

- “节点”页面中节点状态为“在线”
- 预创建节点在被实际占用前状态为“待接入”，占用后变为“在线”或“离线”
- 节点的“当前版本”与刚发布版本一致
- “应用记录”页面中存在成功记录

如果使用自动发现模式，还应确认：

- 注册成功后，Agent 本地 `agent.json` 中已写入新的 `agent_token`
- 注册成功后，本地 `discovery_token` 已被清空

### 4.6 验证失败回滚

可以手工制造一次失败，例如：

- 给节点本地 Nginx 环境制造 `nginx -t` 失败条件
- 再次发布配置

预期结果：

- Agent 写入新文件后校验失败
- Agent 恢复旧路由文件
- Server 中节点 `last_error` 更新
- “应用记录”页面出现失败记录

## 5. 常用验证命令

### Server

```bash
cd atsf_server
GOCACHE=/tmp/atsflare-go-cache go test ./...
```

### Agent

```bash
cd atsf_agent
GOCACHE=/tmp/atsflare-go-cache go test ./...
```

### 前端

```bash
cd atsf_server/web
npm run build
```

## 6. 已知限制

- Agent 配置中的时间字段目前使用纳秒整数，不够友好
- 尚未提供 systemd unit 文件
- 尚未提供 Docker Compose 或一键部署脚本
- Docker 模式当前默认直接重建单容器 Nginx，挂载与端口策略仍是 MVP 水平
- 前端页面已可用，但交互和校验仍是 MVP 水平
- 目前联调说明以手工步骤为主，未内置完整自动化端到端脚本

补充说明：

- Agent 当前已按守护进程式行为实现，心跳失败、同步失败或“当前没有激活版本”不会导致进程直接退出
- 但生产环境仍建议结合 `systemd`、Supervisor 或容器重启策略托管进程

## 7. 下一阶段候选项

- 提供 `systemd` 服务文件与日志轮转建议
- 增加 Agent 配置文件示例模板
- 把 `time.Duration` 配置改成更易读的字符串格式
- 为 Agent 增加退避重试与更细粒度错误恢复
- 增加真实 Nginx 环境的集成测试
