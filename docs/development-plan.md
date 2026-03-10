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

### V2 Phase 1: HTTPS/TLS 支持 ✅

目标：

* 支持通过控制面配置 HTTPS 路由
* 渲染出包含 443 端口的 `server` 块
* 支持 HTTP → HTTPS 重定向块
* 控制面支持托管证书

交付：

* `proxy_routes` 新增字段：`enable_https`、`cert_id`、`redirect_http`
* 渲染器支持 HTTPS server 块生成
* 前端反代规则页增加 HTTPS 配置表单
* `tls_certificates` 表与模型
* 证书导入能力：手动导入（粘贴 PEM）与文件导入
* AutoMigrate 覆盖新字段

完成标准：

* 创建含 HTTPS 字段的路由并发布，Agent 拉取后 Nginx 能以 HTTPS 正确转发
* HTTP 重定向配置生效
* 证书可通过控制面导入并被 HTTPS 路由引用
* 未开启 HTTPS 的路由渲染行为与第一版保持一致

### V2 Phase 2: 域名管理与证书自动匹配 ✅

目标：

* 控制面可管理域名并绑定证书
* 反代规则编辑时按输入域名自动匹配证书
* 支持 `*.example.com` 通配符证书匹配

交付：

* `managed_domains` 表与模型
* 域名管理 CRUD API
* 证书匹配 API（精确匹配 + 通配符匹配）
* 前端域名管理页
* 前端反代规则页接入证书自动匹配

完成标准：

* 创建域名并绑定证书后，反代规则输入域名可自动匹配证书
* 通配符证书可匹配子域名（如 `api.example.com` 匹配 `*.example.com`）
* 无匹配证书时前端给出明确提示

### V2 Phase 3: Agent 管理 ✅

目标：

* 实现对节点的增删改查
* 实现节点自动发现机制


交付：

* agent-auth 中间件改造（查表验证）
* 移除全局 Token 环境变量, 节点不再通过该方式连接server
* 用户手动创建节点时，直接生成节点专属 auth token
* 引入全局自动发现 TOKEN，任意新节点持有同一个 TOKEN 即可自动连接到 SERVER
* Node CRUD API
* 前端 节点 管理页

完成标准：

* 用户启动 server 后手动在节点页添加节点，会直接生成该节点的专属 auth token，持有该 token 的节点可占据该节点位
* 用户可在管理界面查看全局 discovery token，批量部署的节点可共用该 token 自动注册到 server
* 用户可以编辑节点, 包括节点名
* 删除节点后 Agent 请求立即返回 401
* 节点配置文件不再要求填写节点名和IP地址, 节点名默认从主机名获取, IP也是自动获取. 手动指定则为覆盖
* 节点配置文件填写节点专属 `agent_token` 时，可直接上线；若 `agent_token` 为空且填写全局 discovery token，则会自动注册并完成 token 置换


### V2 Phase 4: 路由自定义头 ✅

目标：

* 每条路由支持追加自定义 `proxy_set_header` 指令

交付：

* `proxy_routes` 新增 `custom_headers` 字段（JSON，`[{"key":"X-My-Header","value":"foo"}]`）
* 渲染器按 `custom_headers` 在 `location /` 块中注入额外 header 指令
* 前端反代规则页增加自定义头编辑器

完成标准：

* 路由配置自定义头后发布，渲染结果包含对应 `proxy_set_header` 指令
* 不配置自定义头的路由渲染行为与之前保持一致

### V2 Phase 5: 配置预览与变更摘要 ✅

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
2. 域名管理与证书自动匹配（与 HTTPS 强相关，尽早完成闭环）
3. Agent 管理（节点 CRUD + 自动发现 + token 置换）
4. 路由自定义头（纯增量，对现有结构影响小）
5. 配置预览与变更摘要（纯只读接口，最后补充）

不要先做以下内容：

* 节点分组与差异化下发
* 灰度百分比发布
* WAF、限流
* Redis、Prometheus
* 多租户

## 5. 第二版每阶段验收检查

### V2 Phase 1 检查项

* `enable_https=true` 的路由发布后生成 443 端口 server 块
* `redirect_http=true` 的路由生成 80 → 443 重定向块
* 支持手动导入证书与文件导入证书
* 未开启 HTTPS 的路由渲染结果不受影响
* Agent 拉取后 Nginx reload 成功

### V2 Phase 2 检查项

* 可通过管理界面维护域名并绑定证书
* 反代规则输入域名后可自动匹配证书
* 通配符证书可匹配子域名（`*.example.com`）

### V2 Phase 3 检查项

* 可通过管理界面创建节点并生成节点专属 auth token
* 可查看全局 discovery token，并允许多个节点共用该 token 自动接入
* Agent 可使用全局 discovery token 自动接入并完成 agent token 置换
* 用户可编辑节点名并删除节点
* 删除节点后 Agent 请求立即失败
* Agent 在未显式配置节点名/IP 时可自动探测

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
