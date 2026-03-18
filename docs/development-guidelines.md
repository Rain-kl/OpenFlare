# OpenFlare 开发规范

本文档描述 OpenFlare `1.0.0` 正式版之后的开发基线。

超出 [docs/design.md](./design.md) 边界的需求，必须先更新设计文档。

## 1. 技术基线

### 1.1 Server

`openflare_server` 继续作为单体控制面：

* Go 1.24+
* Gin
* GORM
* SQLite / PostgreSQL
* 现有登录体系

### 1.2 Agent

`openflare_agent` 继续作为 Go 单体程序：

* Go 1.23+
* 单二进制
* 节点本地执行
* `openresty_path` 优先
* 无 `openresty_path` 时默认 Docker OpenResty

### 1.3 Frontend

前端基线以 `openflare_server/web` 为准：

* Next.js 15 App Router
* React 19
* TypeScript
* Tailwind CSS 4
* TanStack Query
* React Hook Form + Zod
* Zustand 仅用于轻量客户端状态

前端细则见 [docs/frontend-development-guidelines.md](./frontend-development-guidelines.md)。

## 2. 分层与目录约束

### 2.1 Server

* `controller/`：参数解析、调用 service、返回响应
* `service/`：业务逻辑、校验、事务编排、渲染
* `model/`：模型定义与持久化
* `router/`：路由注册
* `middleware/`：认证、鉴权、限流等横切逻辑
* `common/`：配置、全局状态与初始化入口
* `utils/`：纯工具函数与通用 helper

禁止：

* 在 `controller/` 堆积业务逻辑
* 在 `middleware/` 实现业务流程
* 为简单需求新增平台层抽象

### 2.2 Agent

保持现有模块边界：

* `config`
* `heartbeat`
* `sync`
* `openresty`
* `state`
* `httpclient`
* `protocol`
* `internal/updater`

要求：

* 每个模块职责单一
* 外部命令调用集中封装
* 状态落盘与配置落盘分离

### 2.3 Frontend

前端分层保持：

* `app/`
* `features/`
* `components/`
* `lib/`
* `store/`
* `types/`

要求：

* 页面路由与布局放在 `app/`
* API 请求统一收敛到 `lib/api/`
* 业务逻辑优先放在 `features/`

## 3. 数据模型规范

当前有效实体：

* `proxy_routes`
* `config_versions`
* `nodes`
* `node_system_profiles`
* `apply_logs`
* `tls_certificates`
* `managed_domains`
* `node_request_reports`
* `node_access_logs`
* `node_metric_snapshots`
* `traffic_analytics_rollups`
* `node_health_events`
* `options`

通用约束：

* 不新增平台化对象，除非设计文档明确要求
* `proxy_routes` 维持一条域名对应一条规则；规则内允许保存一个或多个上游地址用于负载均衡，但不引入独立 `origin_pool`
* `proxy_routes.origin_host` 为可选字段，仅用于覆盖回源 `Host` 请求头，不引入新的平台化对象
* `config_versions` 必须保存完整快照与渲染结果
* 全局同时只能有一个激活版本
* 回滚通过重新激活旧版本实现
* `nodes` 只保留控制面状态与低频摘要
* 观测数据必须按节点与时间窗口关联
* 快照与聚合结果采用追加式模型，不覆盖历史
* 原始访问明细必须有受控保留策略

## 4. API 与鉴权规范

### 4.1 API

* 管理端与 Agent API 统一使用 JSON
* 成功与失败都必须返回清晰 `message`
* Agent API 固定放在 `/api/agent/*`
* 总览与节点详情优先使用专用聚合接口
* 管理端变更类接口统一使用 `POST`；只读接口使用 `GET`

统一响应结构：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

### 4.2 鉴权

管理端：

* 继续复用现有登录、角色与 Session

Agent：

* 正式请求统一使用节点专属 `agent_token`
* 首次接入可使用全局 `discovery_token`
* 请求头统一使用 `X-Agent-Token`

禁止：

* 暴露远程 shell 或任意命令执行入口
* 在日志中打印完整 Token
* 允许绕过占位符约束保存不可渲染的主配置模板

## 5. 发布与运行规范

发布逻辑必须保持以下事实：

* 发布时读取全部启用的 `proxy_routes`
* 同时读取 OpenResty 主配置参数、反代性能参数与缓存参数
* 生成完整 OpenResty 配置
* 计算 `checksum`
* 写入 `config_versions`
* 通过切换 `is_active` 激活版本

版本约束：

* 版本号格式固定为 `YYYYMMDD-NNN`
* 不在线修改历史版本
* 不做按节点分组的差异化版本
* 预览与 diff 是只读能力，不产生发布记录

Agent 必须满足：

* 启动后读取或生成本地 `node_id`
* 周期性心跳与同步
* 常规同步优先依据 heartbeat 返回的版本摘要判断
* 发现新版本时先备份旧文件
* 写入主配置、路由配置与必要证书文件
* 写入新配置后以运行态恢复为目标执行激活，Docker 模式优先重建容器并确认容器保持运行
* 新配置激活失败时必须先尝试用目标配置恢复运行，再回滚到旧配置并重新拉起 OpenResty
* 回滚后 OpenResty 恢复正常时上报警告；回滚后仍无法恢复运行时上报失败

## 6. 测试与交付要求

* 关键业务逻辑必须有单元测试或等效回归测试
* Agent 主链路修改必须验证同步、应用与回滚
* 前端页面至少覆盖加载态、空态、错误态与成功反馈
* Go 版本调整时，同步检查 `go.mod`、Dockerfile 与 CI 工作流

## 7. 文档维护要求

当以下内容变化时，必须同步更新对应文档：

* 产品范围或系统边界变化：更新 `docs/design.md`
* 开发约束、接口约定、测试基线变化：更新本文档
* 前端工程约束变化：更新 `docs/frontend-development-guidelines.md`
* 配置项或部署方式变化：更新 `docs/app-config.md`、`docs/deployment.md` 与 `README.md`
