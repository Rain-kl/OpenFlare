# 仓库结构

你会学到：OpenFlare 仓库中 Server、Agent、前端、脚本和文档目录分别负责什么，以及贡献代码时应把逻辑放到哪一层。

| 路径 | 职责 |
| --- | --- |
| `openflare_server` | Gin + GORM + SQLite/PostgreSQL 单体控制面 |
| `openflare_server/web` | Next.js 15 App Router 管理端前端，静态导出后由 Go Server 托管 |
| `openflare_agent` | Go 单体 Agent，运行在节点侧 |
| `scripts` | Agent 安装、卸载等辅助脚本 |
| `docs` | VitePress 文档站、设计基线、开发规范、部署与配置文档 |

## Server 分层

| 目录 | 职责 |
| --- | --- |
| `controller/` | 参数解析、调用 service、返回响应 |
| `service/` | 业务逻辑、校验、事务编排、配置渲染 |
| `model/` | 模型定义、数据库版本与迁移 |
| `router/` | 路由注册 |
| `middleware/` | 认证、鉴权、限流等横切逻辑 |
| `common/` | 配置、全局状态与初始化入口 |
| `utils/` | 纯工具函数与通用 helper |

## Agent 模块

| 模块 | 职责 |
| --- | --- |
| `config` | 配置读取与默认值 |
| `heartbeat` | 心跳与版本摘要判断 |
| `sync` | 配置拉取与应用编排 |
| `nginx` / `openresty` | OpenResty 文件写入、校验、reload、启动与回滚 |
| `state` | 本地状态与观测补报缓冲 |
| `httpclient` | Server 通信 |
| `protocol` | Agent API 协议类型 |
| `internal/updater` | Agent 自更新 |

## Frontend 分层

| 目录 | 职责 |
| --- | --- |
| `app/` | 路由、布局、页面组装 |
| `features/` | 按业务域组织模块 |
| `components/` | 跨 feature 复用组件 |
| `lib/` | 请求客户端、环境变量、工具函数、常量 |
| `store/` | 少量跨页面 UI 状态 |
| `types/` | 共享类型定义 |
