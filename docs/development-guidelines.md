# ATSFlare 开发规范

## 1. 适用范围

本规范适用于当前代码基线下的所有 Server、Agent 与管理端前端开发工作。

当前状态：

* 第一版、第二版、第三版已完成
* `docs/design.md` 是当前系统边界的唯一设计基线
* `atsf_server/web` 新版前端已完成迁移并成为正式基线

超出设计边界的需求，必须先更新 [docs/design.md](./design.md)。

---

## 2. 技术基线

### 2.1 Server

`atsf_server` 继续作为单体控制面：

* Gin
* GORM
* SQLite
* 现有 ATSFlare 登录体系

约束：

* 默认不引入 Redis、MQ、对象存储等新基础设施
* 不为未确认的平台化能力预埋复杂抽象

### 2.2 Agent

`atsf_agent` 继续作为 Go 单体程序：

* 单二进制
* 节点本地执行
* `openresty_path` 优先
* 无 `openresty_path` 时默认 Docker OpenResty
* 生成资源默认写入 `./data`，由 `data_dir` 统一覆盖

### 2.3 Frontend

新版前端基线以当前 `atsf_server/web` 实现为准：

* Next.js 15 App Router
* React 19
* TypeScript
* Tailwind CSS 4
* TanStack Query
* React Hook Form + Zod
* Zustand（仅限轻量客户端状态）
* 静态导出并由 Go Server 托管

前端详细约束统一以 [docs/frontend-development-guidelines.md](./frontend-development-guidelines.md) 为准；本文件只保留跨项目层面的强约束。

---

## 3. 分层与目录约束

### 3.1 Server

* `controller/`：参数解析、调用 service、返回响应
* `service/`：业务逻辑、校验、渲染、事务编排
* `model/`：模型定义与持久化
* `router/`：路由注册
* `middleware/`：认证、鉴权、限流等横切逻辑
* `common/`：配置与通用工具

禁止：

* 在 `controller/` 堆积业务逻辑
* 在 `middleware/` 中实现业务流程
* 为简单需求新增平台层抽象

### 3.2 Agent

保持现有模块边界：

* `config`
* `heartbeat`
* `sync`
* `openresty`（保留目录名，内部负责 OpenResty 运行时管理）
* `state`
* `httpclient`
* `protocol`
* `internal/updater`

要求：

* 每个模块职责单一
* 外部命令调用集中封装
* 状态落盘与配置落盘分离

### 3.3 Frontend

前端分层与目录必须与当前工程保持一致：

* `app/`：路由、布局、页面组装
* `features/`：业务模块
* `components/`：跨模块复用组件
* `lib/`：请求、环境、工具、常量
* `store/`：少量跨页面 UI 状态
* `types/`：共享类型

要求：

* 页面路由与布局放在 `app/`
* API 请求统一收敛到 `lib/api/`
* 业务逻辑优先放在 `features/`
* 不重新引入旧版 CRA / Semantic UI 结构

---

## 4. 数据模型规范

当前有效实体：

* `proxy_routes`
* `config_versions`
* `nodes`
* `apply_logs`
* `tls_certificates`
* `managed_domains`

通用约束：

* 不新增平台化对象，除非设计文档明确要求
* `proxy_routes` 仍保持一条域名对应一个 `origin_url`
* `config_versions` 必须保存完整快照与渲染结果
* 全局同时只能有一个激活版本
* 回滚通过重新激活旧版本实现
* 域名证书匹配必须同时支持精确匹配与通配符匹配
* 节点专属 `agent_token` 必须可立即失效

---

## 5. API 与鉴权规范

### 5.1 API

* 管理端与 Agent API 统一使用 JSON
* 成功与失败都必须返回清晰 `message`
* 列表接口返回稳定字段
* Agent API 固定放在 `/api/agent/*`

统一响应结构保持现有风格：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

### 5.2 鉴权

管理端：

* 继续复用 ATSFlare 登录、角色与 session

Agent：

* 正式请求统一使用节点专属 `agent_token`
* 首次接入可使用全局 `discovery_token`
* 请求头统一使用 `X-Agent-Token`
* Agent 认证逻辑不得与用户登录态混用

禁止：

* 将本地 OpenResty 操作暴露为远程执行接口
* 在日志中打印完整 Token

---

## 6. 发布与运行规范

发布逻辑必须保持以下事实：

* 发布时读取全部启用的 `proxy_routes`
* 生成完整 OpenResty 配置
* 计算 `checksum`
* 写入 `config_versions`
* 通过切换 `is_active` 激活版本

版本号格式保持：

```text
YYYYMMDD-NNN
```

限制：

* 不在线修改历史版本
* 不做按节点分组的差异化版本
* 预览与 diff 是只读能力，不产生发布记录

Agent 必须满足：

* 启动后读取或生成本地 `node_id`
* 未显式配置 `node_name` 时自动获取主机名
* 未显式配置 `node_ip` 时自动探测本机 IP
* 周期性心跳与同步
* 发现新版本时先备份旧文件
* 写入新路由与必要证书文件
* 先执行 `openresty -t`
* 成功后执行 `openresty -s reload`
* 失败时自动回滚并上报最终结果
* 支持自动注册与 Token 置换
* 支持接收 Server 下发运行参数
* 支持自我更新，但失败不影响心跳与同步

---

## 7. 前端约束

前端新增开发必须遵循 [docs/frontend-development-guidelines.md](./frontend-development-guidelines.md)，其中以下要求属于项目级强约束：

* 页面与布局放在 `app/`，业务逻辑放在 `features/`
* 请求统一通过 `lib/api/`
* 构建产物必须保持可被 Go Server 静态托管
* 主题能力必须覆盖布局、基础组件与业务页面
* 不引入新的大型 UI 框架与旧式页面结构

---

## 8. 代码风格与日志

### 8.1 Go

* 错误必须显式处理
* 函数尽量单一职责
* 输入校验放在边界层
* 业务枚举使用明确常量
* 不写无意义注释

### 8.2 命名

* 统一使用 `route`、`version`、`node`、`agent`
* 不混用 `client`、`edge`、`worker` 指代 Agent

### 8.3 日志

必须覆盖关键事件：

* 发布成功/失败
* Agent 注册
* 心跳异常
* 配置下载失败
* OpenResty 校验或 reload 成功/失败
* 回滚触发

要求：

* 日志足够定位问题
* 不打印敏感凭证完整值

---

## 9. 测试与验收

基线回归至少覆盖：

* 路由校验与渲染
* 激活版本切换
* 节点在线状态判定
* 证书导入与匹配
* 自定义请求头渲染
* Agent 同步、回滚、本地状态读写
* 自动注册与 Token 置换
* Agent 设置下发与更新链路
* 预览与 diff 的只读行为

新增需求时：

* 先补单元测试或服务层测试
* 再补联调验证步骤
* 涉及发布链路、Agent 链路、鉴权链路的改动，必须补回归测试

---

## 10. 文档维护

出现以下情况必须同步更新文档：

* 产品范围或系统边界变化：更新 `docs/design.md`
* 开发约束、接口约定、前后端分层变化：更新本文件
* 前端目录分层、请求层、主题体系变化：更新 `docs/frontend-development-guidelines.md`
* 部署方式变化：更新 `docs/deployment.md` 和 `README.md`
* 环境变量或配置项变化：更新 `docs/app-config.md`

## 11. Swagger 约束

* Server 提供 Swagger UI 入口：`/swagger/index.html`
* Swagger UI 仅对已登录的管理端用户开放
* 新增或修改 API 时，必须同步更新 Swag 注解并重新生成 `atsf_server/docs`
