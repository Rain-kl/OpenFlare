# ATSFlare 开发计划

## 1. 目标

当前开发目标是完成 ATSFlare 的 MVP 闭环：

```text
后台改规则 -> 点击发布 -> Agent 拉到新版本 -> 写入 Nginx 路由配置 -> nginx 校验并 reload -> 节点状态可见
```

MVP 已于第一版完成。当前进入第二版迭代。

## 2. 第一版里程碑（已完成）

### Phase 1: Server 数据层与发布闭环 ✅

交付：

* 新模型（proxy_routes、config_versions）
* AutoMigrate
* 路由 CRUD API
* 发布 API
* 激活版本 API
* 渲染 service

### Phase 2: Agent API 与节点状态 ✅

交付：

* `nodes`、`apply_logs`
* Agent Token 鉴权（全局单 Token）
* 节点在线状态计算
* 节点与应用日志查询接口

### Phase 3: Agent 本体 ✅

交付：

* 本地配置文件读取
* `node_id` 持久化
* 心跳循环
* 版本检查
* 下载配置
* 写入 Nginx 路由配置
* 配置校验、reload 与失败回滚
* 应用结果上报

### Phase 4: 管理端页面 ✅

交付：

* 反代规则页
* 版本页
* 节点页
* 应用记录页

### Phase 5: 联调与收尾 ✅

交付：

* 手工部署说明
* Agent 配置示例
* 联调验证记录

## 3. 第二版里程碑

### V2 Phase 1: HTTPS/TLS 支持

目标：

* 支持通过控制面配置 HTTPS 路由
* 渲染出包含 443 端口的 `server` 块
* 支持 HTTP → HTTPS 重定向块

交付：

* `proxy_routes` 新增字段：`enable_https`、`ssl_cert_path`、`ssl_key_path`、`redirect_http`
* 渲染器支持 HTTPS server 块生成
* 前端反代规则页增加 HTTPS 配置表单
* AutoMigrate 覆盖新字段

完成标准：

* 创建含 HTTPS 字段的路由并发布，Agent 拉取后 Nginx 能以 HTTPS 正确转发
* HTTP 重定向配置生效
* 未开启 HTTPS 的路由渲染行为与第一版保持一致

### V2 Phase 2: Agent Token 管理

目标：

* 支持多个命名 Token
* Token 可通过管理界面创建和撤销
* 中间件改为查库验证

交付：

* `agent_tokens` 表与模型
* agent-auth 中间件改造（查表验证）
* 全局 Token 环境变量降级为 bootstrap 模式
* Token CRUD API
* 前端 Token 管理页

完成标准：

* 新建 Token 后 Agent 可用该 Token 访问 Agent API
* 撤销 Token 后 Agent 请求立即返回 401
* 全局环境变量 Token 仅在数据库无有效记录时生效

### V2 Phase 3: 节点分组与差异化下发

目标：

* 节点可按分组管理
* 发布时可选择目标分组，生成分组专属版本
* Agent 按分组拉取对应激活版本

交付：

* `node_groups` 表与模型
* `nodes` 新增 `group_id` 字段
* `config_versions` 新增 `group_id` 字段（nullable，null 表示全量）
* 发布 API 接受可选 `group_id` 参数
* Agent API 按节点分组返回对应激活版本
* Agent `agent.json` 新增 `group_id` 配置项
* 前端节点分组管理页与节点页分组字段

完成标准：

* 同一系统内 staging 和 production 分组各有独立激活版本
* staging 节点拉取 staging 版本，production 节点拉取 production 版本
* 不属于任何分组的节点拉取全量（group_id = null）版本

### V2 Phase 4: 路由自定义头

目标：

* 每条路由支持追加自定义 `proxy_set_header` 指令

交付：

* `proxy_routes` 新增 `custom_headers` 字段（JSON，`[{"key":"X-My-Header","value":"foo"}]`）
* 渲染器按 `custom_headers` 在 `location /` 块中注入额外 header 指令
* 前端反代规则页增加自定义头编辑器

完成标准：

* 路由配置自定义头后发布，渲染结果包含对应 `proxy_set_header` 指令
* 不配置自定义头的路由渲染行为与之前保持一致

### V2 Phase 5: 配置预览与变更摘要

目标：

* 发布前可预览渲染结果
* 发布前可查看与当前激活版本的变更摘要

交付：

* `GET /api/config-versions/preview` 接口（返回渲染后的 Nginx 配置，不写库）
* `GET /api/config-versions/diff` 接口（返回新增/删除/修改的域名列表）
* 前端发布版本页增加"预览"按钮和变更摘要展示弹窗

完成标准：

* 点击预览可查看即将生成的 Nginx 配置文本
* 变更摘要正确列出相对于当前激活版本的域名变化
* 预览和 diff 操作不产生版本记录

## 4. 第二版建议执行顺序

建议严格按以下顺序开发：

1. HTTPS/TLS 支持（对现有渲染链路影响最小，独立可测）
2. Agent Token 管理（安全性改善，早做早稳）
3. 节点分组（数据模型扩展，影响面最广，需要 Agent 配合）
4. 路由自定义头（纯增量，对现有结构影响小）
5. 配置预览与变更摘要（纯只读接口，最后补充）

不要先做以下内容：

* 证书托管
* 灰度百分比发布
* WAF、限流
* Redis、Prometheus
* 多租户

## 5. 第二版每阶段验收检查

### V2 Phase 1 检查项

* `enable_https=true` 的路由发布后生成 443 端口 server 块
* `redirect_http=true` 的路由生成 80 → 443 重定向块
* 未开启 HTTPS 的路由渲染结果不受影响
* Agent 拉取后 Nginx reload 成功

### V2 Phase 2 检查项

* 可通过管理界面创建 Token
* 新 Token 可被 Agent 使用
* 撤销 Token 后访问立即失败
* 全局 Token 环境变量在数据库有记录时失效

### V2 Phase 3 检查项

* 可创建分组并将节点归入分组
* 可发布面向指定分组的版本
* 分组节点只拉取该分组的激活版本
* 无分组节点拉取全量激活版本

### V2 Phase 4 检查项

* 路由可添加自定义头
* 渲染结果中正确包含自定义 header 指令
* 无自定义头的路由渲染结果不受影响

### V2 Phase 5 检查项

* 预览接口返回正确的 Nginx 配置文本
* diff 接口返回正确的域名变更列表
* 两个接口均不产生数据库写入

## 6. 变更控制

开发中如果出现以下情况，需要先调整计划再继续编码：

* V2 目标发生变化
* 需要引入新的中间件（Redis、MQ 等）
* 需要新增核心数据模型
* 需要把控制面扩展到 Nginx 全局配置

计划更新时，应同步修改：

* `docs/design.md`
* `docs/development-guidelines.md`
* `docs/development-plan.md`
