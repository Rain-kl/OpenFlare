# ATSFlare 部署与联调说明

本文档对应 Phase 5，用于指导在新环境手工完成 ATSFlare MVP 的最小部署与联调。

## 1. 前置条件

### Server

- Go 1.18+
- Node.js 18+ 与 npm
- 本地可写 SQLite 文件目录

### Agent

- Go 1.18+
- 节点已安装 `nginx`
- Agent 对目标路由文件路径有写权限
- Agent 运行用户可以执行 `nginx -t` 和 `nginx -s reload`

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
export SQLITE_PATH='./gin-template.db'
export AGENT_TOKEN='replace-with-shared-agent-token'
go run .
```

说明：

- 如果未设置 `SQLITE_PATH`，默认也会落到 `atsf_server/gin-template.db`
- 如果未设置 `AGENT_TOKEN`，Agent API 会拒绝访问
- 当前默认监听端口为 `3000`

### 2.3 首次登录

访问 `http://localhost:3000`

默认账号：

- 用户名：`root`
- 密码：`123456`

首次登录后建议立即修改密码。

## 3. Agent 启动

### 3.1 Agent 配置文件示例

在节点上创建 `agent.json`：

```json
{
  "server_url": "http://127.0.0.1:3000",
  "agent_token": "replace-with-shared-agent-token",
  "node_name": "edge-01",
  "node_ip": "10.0.0.8",
  "agent_version": "0.1.0",
  "nginx_version": "1.25.5",
  "nginx_path": "/opt/nginx/sbin/nginx",
  "nginx_container_name": "atsflare-nginx",
  "nginx_docker_image": "nginx:stable-alpine",
  "route_config_path": "/etc/nginx/conf.d/atsflare_routes.conf",
  "state_path": "/var/lib/atsflare/agent-state.json",
  "heartbeat_interval": 30000000000,
  "sync_interval": 30000000000,
  "request_timeout": 10000000000
}
```

注意：

- 时间字段单位是纳秒，因为当前实现直接使用 Go 的 `time.Duration` JSON 反序列化
- `agent_token` 必须与 Server 侧 `AGENT_TOKEN` 完全一致
- `route_config_path` 只应指向 ATSFlare 独立管理的路由文件
- `nginx_path` 用于显式指定独立 Nginx 可执行文件路径
- 如果未指定 `nginx_path`，Agent 会尝试通过 Docker 启动独立 Nginx 容器
- Docker 模式默认使用 `nginx_container_name` 和 `nginx_docker_image`

### 3.2 启动 Agent

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

### 4.1 创建规则

1. 登录管理端
2. 打开“规则”页面
3. 新增一条反代规则，例如：
   - 域名：`demo.example.com`
   - 源站：`http://127.0.0.1:8080`
   - 启用：开启

### 4.2 发布版本

1. 在“规则”页面点击“发布当前规则”
2. 或在“版本”页面点击“生成新版本”
3. 确认“版本”页面出现新的激活版本

### 4.3 验证 Agent 拉取与应用

启动 Agent 后，预期行为如下：

1. Agent 首次注册节点
2. Agent 拉取当前激活版本
3. Agent 写入 `route_config_path`
4. Agent 使用 `nginx_path` 指向的独立 Nginx，或自动准备 Docker Nginx 容器
5. Agent 执行 `nginx -t`
6. Agent 执行 `nginx -s reload`
7. Agent 上报成功结果

### 4.4 验证管理端状态

在管理端确认：

- “节点”页面中节点状态为“在线”
- 节点的“当前版本”与刚发布版本一致
- “应用记录”页面中存在成功记录

### 4.5 验证失败回滚

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
- Agent 运行器当前任一心跳或同步失败会直接退出，需要结合进程管理器拉起
- 尚未提供 systemd unit 文件
- 尚未提供 Docker Compose 或一键部署脚本
- Docker 模式当前默认直接启动单容器 Nginx，挂载与端口策略仍是 MVP 水平
- 前端页面已可用，但交互和校验仍是 MVP 水平
- 目前联调说明以手工步骤为主，未内置完整自动化端到端脚本

## 7. 下一阶段候选项

- 提供 `systemd` 服务文件与日志轮转建议
- 增加 Agent 配置文件示例模板
- 把 `time.Duration` 配置改成更易读的字符串格式
- 为 Agent 增加退避重试与更细粒度错误恢复
- 增加真实 Nginx 环境的集成测试
