# ATSFlare 开发规范

## 1. 适用范围

本规范适用于 ATSFlare 当前 MVP 阶段。

项目当前只做以下能力：

* 配置发布与同步
* 节点心跳检测
* Nginx 反向代理配置下发

当前明确不做：

* 多租户
* WAF、限流、Bot、防刷
* 灰度发布、分组发布、百分比发布
* Redis、MQ、对象存储、Prometheus
* 复杂缓存策略、证书托管、Purge、审批流
* mid-tier、分层缓存、复杂策略编排

超出以上范围的需求，必须先更新设计文档，再开始编码。

## 2. 技术基线

### 2.1 Server

控制中心基于现有 `atsf_server` 开发：

* Web 框架：Gin
* ORM：GORM
* 数据库：SQLite
* 前端：现有 `atsf_server/web`
* 登录体系：沿用 gin-template 现有能力

约束：

* 默认不配置 `SQL_DSN`
* 默认不配置 `REDIS_CONN_STRING`
* 不为了 MVP 引入新的基础设施依赖

### 2.2 Agent

Agent 放在 `atsf_agent`，使用 Go 单体程序开发。

约束：

* 单二进制
* systemd 运行
* 优先调用独立 Nginx，不依赖系统全局 Nginx
* 支持通过 `nginx_path` 显式指定独立 Nginx 可执行文件
* 未指定 `nginx_path` 时，默认通过 Docker 启动独立 Nginx 容器
* Agent 生成资源默认统一放在 `./data`，可通过 `data_dir` 统一覆盖
* 负责本机 Nginx 路由配置写入、校验、reload、状态上报

### 2.3 Nginx 配置边界

第一版控制面只管理独立生成的 Nginx 路由配置文件，例如 `/etc/nginx/conf.d/atsflare_routes.conf`。

以下内容先不纳入控制面：

* `nginx.conf`
* TLS 证书
* 缓存策略
* upstream 高级配置

这些内容先保持节点本地静态配置。

## 3. 仓库职责划分

### 3.1 `atsf_server`

负责：

* 管理端 UI
* 管理端 API
* Agent API
* 数据存储
* 配置渲染
* 版本发布
* 节点状态展示

### 3.2 `atsf_agent`

负责：

* 节点注册
* 心跳上报
* 拉取激活版本
* 写入本地 Nginx 路由配置
* 调用 `nginx -t` 和 `nginx -s reload`
* 失败回滚
* 上报应用结果
* 管理独立 Nginx 路径或 Docker Nginx 容器

### 3.3 `docs`

负责：

* 设计边界
* 开发规范
* 开发计划
* 部署与联调说明

## 4. 开发原则

所有实现都必须遵守以下原则：

* 先完成闭环，再做抽象。
* 不为了“以后可能会支持”提前引入复杂模型。
* Server 只管状态和配置，不直接 SSH 改节点。
* Agent 是唯一落地入口。
* 所有发布都是“新版本激活”，不是在线覆盖编辑。
* 所有节点默认拉同一份全量配置，不做差异化编排。
* 能用 SQLite 解决的问题，不引入额外中间件。
* 新功能优先复用现有 gin-template 结构，不平行造第二套框架。

## 5. 数据模型规范

第一版只允许引入以下核心实体：

* `proxy_routes`
* `config_versions`
* `nodes`
* `apply_logs`

约束：

* 不新增 `zone`、`origin_pool`、`policy`、`deployment` 这类平台化对象
* `proxy_routes` 一条域名只对应一个 `origin_url`
* `config_versions` 必须保存完整快照和渲染后的 Nginx 路由配置
* 激活版本全局只能有一个
* 回滚通过“激活旧版本”实现，不直接修改历史记录

如需新增表，必须先证明它服务于 MVP 主链路。

## 6. Server 开发规范

### 6.1 分层约束

Server 代码按以下职责拆分：

* `controller/`: 参数解析、调用 service、返回 JSON
* `service/`: 业务逻辑、校验、渲染、版本切换
* `model/`: 数据表结构、查询和持久化
* `router/`: 路由注册
* `middleware/`: 认证、鉴权、限流等横切逻辑
* `common/`: 通用工具和配置

禁止行为：

* controller 直接拼接复杂业务逻辑
* controller 直接操作多个 model 形成事务链
* middleware 承担业务逻辑
* 为简单需求引入新的平台层抽象

### 6.2 API 约定

管理端和 Agent API 统一使用 JSON。

响应结构沿用现有模板风格：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

约束：

* 成功或失败都返回清晰 `message`
* 列表接口返回稳定字段，不临时拼装结构
* 新接口命名优先使用复数资源风格
* Agent API 固定放在 `/api/agent/*`

### 6.3 鉴权规范

管理端：

* 继续复用 gin-template 的登录、角色和 session 体系

Agent：

* 第一版使用预共享 Token
* 请求头统一使用 `X-Agent-Token`
* Agent 与管理端认证逻辑必须分开

注意：

* 不要让 Agent 接口走用户登录态
* 不要把 Nginx 命令暴露成远程管理接口

### 6.4 数据库规范

SQLite 是唯一默认数据库。

要求：

* 新增模型后必须在 `model.InitDB()` 中加入 `AutoMigrate`
* 不写 MySQL/PostgreSQL 特有 SQL
* 不依赖外部迁移工具作为 MVP 前提
* 时间字段统一使用 GORM 常规时间类型

### 6.5 发布与渲染规范

发布逻辑必须满足：

* 发布时读取全部启用的 `proxy_routes`
* 生成完整的 Nginx 路由配置
* 计算 checksum
* 保存快照到 `config_versions`
* 通过切换 `is_active` 激活版本

版本号格式固定为：

```text
YYYYMMDD-NNN
```

例如：

```text
20260309-001
```

## 7. Agent 开发规范

### 7.1 模块边界

建议目录如下：

```text
atsf_agent/
  cmd/agent/
  internal/config/
  internal/heartbeat/
  internal/sync/
  internal/nginx/
  internal/state/
  internal/httpclient/
```

职责要求：

* `config`: 本地配置读取
* `heartbeat`: 心跳请求和状态组装
* `sync`: 检查版本、下载配置、触发应用
* `nginx`: 封装 Nginx 校验、reload 和文件写入
* `state`: 本地成功版本和运行状态缓存
* `httpclient`: Server API 调用

### 7.2 行为规范

Agent 必须满足以下行为：

* 启动后生成或读取本地 `node_id`
* 周期性心跳
* 周期性检查激活版本
* 发现新版本后先备份旧文件
* 写入新路由配置文件
* 先执行 `nginx -t`
* 校验通过后执行 `nginx -s reload`
* 失败时回滚备份并再次校验和 reload
* 上报最终应用结果
* 优先使用 `nginx_path`
* 未配置 `nginx_path` 时自动准备并使用 Docker Nginx 容器
* 启动时先校验本地路由文件 checksum 与控制面激活版本是否一致
* Docker 模式启动时应重建容器，而不是继续复用异常停止的旧容器

### 7.3 容错规范

第一版最少保证：

* Server 不可用时，Nginx 继续使用旧配置
* 下载失败时，不修改本地配置
* 配置校验或 reload 失败时，自动尝试回滚
* 本地状态文件损坏时，允许重新初始化，但不能删除正在生效的 Nginx 配置
* Docker 容器异常停止时，启动阶段应自动重建容器并重新校验配置

### 7.4 外部命令规范

Agent 调用 Nginx 命令时必须：

* 明确记录执行命令和返回错误
* 设置合理超时
* 不依赖交互式输入
* 不通过 shell 拼接不可信参数

## 8. 前端开发规范

MVP 前端只做最小管理界面，不重做整套后台。

要求：

* 继续使用现有 React 结构和 `semantic-ui-react`
* 页面只增加 MVP 必需页面
* 不额外引入新的大型前端框架
* API 请求统一放在 `web/src/helpers/api.js` 或同类 helper 中
* 页面状态优先保持简单，不提前引入复杂全局状态管理

第一版只需要以下页面：

* 反代规则页
* 发布版本页
* 节点状态页
* 应用记录页

## 9. 代码风格规范

### 9.1 Go

* 保持 package 名称简短且小写
* 错误必须显式处理，不允许静默吞错
* 函数尽量只做一件事
* 输入校验放在 controller 或 service 边界
* 业务枚举值使用明确常量，不使用魔法字符串散落代码
* 仅在复杂逻辑前添加简短注释，不写废话注释

### 9.2 命名

* 表名和模型名使用业务语义，不沿用模板示例语义
* 统一使用 `route`, `version`, `node`, `apply log` 这些术语
* 不混用 `client`、`edge`、`agent` 指代同一模块，统一叫 `agent`

### 9.3 日志

要求记录这些关键事件：

* 发布成功/失败
* Agent 注册
* 心跳异常
* 配置下载失败
* Nginx 校验或 reload 成功/失败
* 回滚触发

日志内容要可定位问题，但不要打印敏感 Token。

## 10. 测试与验收规范

### 10.1 最低测试要求

Server 至少覆盖：

* `origin_url` 校验
* `domain` 重复校验
* Nginx 路由配置渲染结果
* 激活版本切换逻辑
* 节点在线状态判定逻辑

Agent 至少覆盖：

* 版本比较逻辑
* 配置文件备份和回滚逻辑
* `nginx -t` 或 `nginx -s reload` 失败分支
* 本地状态文件读写

### 10.2 联调验收标准

MVP 完成至少要通过以下手工验证：

1. 管理端新增反代规则并成功发布版本
2. Agent 能检测到新版本并拉取
3. Agent 成功写入 Nginx 路由配置文件
4. Agent 成功执行 `nginx -t` 和 `nginx -s reload`
5. 节点页能看到当前版本和最后心跳
6. 当 reload 失败时，Agent 能回滚到旧配置并上报失败

## 11. 文档维护规范

出现以下情况时必须同步更新文档：

* MVP 范围变化
* API 发生破坏性变更
* 数据模型新增或删除
* Agent 本地文件路径变更
* 部署方式变化

优先更新：

* `docs/design.md`
* `docs/development-guidelines.md`
* `docs/development-plan.md`
