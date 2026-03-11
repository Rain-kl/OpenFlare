# ATSFlare 开发计划（V3）

## 1. 当前状态

当前结论：

* 第一版已完成并稳定闭环
* 第二版已完成并补齐 HTTPS、证书、域名、节点管理与预览能力
* 第三版进入实施阶段，聚焦运维体验优化

本文件不再展开第一版、第二版的详细实施步骤，只保留第三版实施计划与验收标准。

---

## 2. 已完成能力归档

### 2.1 第一版归档

已完成：

* 规则管理
* 配置发布与激活
* Agent 心跳、同步、应用、回滚
* 节点状态与应用记录展示

### 2.2 第二版归档

已完成：

* HTTPS/TLS 路由支持
* 证书托管与导入
* 域名管理与证书自动匹配
* 节点管理、专属 `agent_token`、全局 `discovery_token`
* 路由自定义请求头
* 配置预览与变更摘要

归档原则：

* 已完成阶段的实现细节以代码和 Git 历史为准
* 后续计划文档只维护当前阶段与下一阶段

---

## 3. 第三版实施计划

### 3.1 阶段一：Server 运维设置热更新

目标：将可热更新的运维相关设置迁入 Option 表，前端提供设置面板。

实施步骤：

1. 在 `common/constants.go` 新增运维设置变量：
   * `AgentHeartbeatInterval`（默认 30000ms）
   * `AgentSyncInterval`（默认 30000ms）
   * `NodeOfflineThreshold`（默认 120000ms）
   * `AgentUpdateRepo`（默认 `Rain-kl/ATSFlare`）
   * `GlobalApiRateLimitNum` / `GlobalApiRateLimitDuration`
   * `GlobalWebRateLimitNum` / `GlobalWebRateLimitDuration`
   * `UploadRateLimitNum` / `UploadRateLimitDuration`
   * `DownloadRateLimitNum` / `DownloadRateLimitDuration`
   * `CriticalRateLimitNum` / `CriticalRateLimitDuration`
2. 在 `model/option.go` 的 `InitOptionMap()` 注册新选项
3. 在 `model/option.go` 的 `updateOptionMap()` 增加对新选项的同步
4. 修改 `service/agent.go` 中 `computeNodeStatus()` 使用动态 `NodeOfflineThreshold`
5. 前端设置页新增「运维设置」Tab

验收标准：

* 运维设置在设置页面可查看和修改
* 修改后立即生效，无需重启 Server
* `NodeOfflineThreshold` 变更后节点状态判定使用新阈值
* 限流阈值与时间窗口可在设置页调整，并即时影响对应中间件

### 3.2 阶段二：Server 下发 Agent 设置 + Agent 接收

目标：心跳响应携带 `agent_settings`，Agent 动态调整运行参数。

实施步骤：

1. Server 端：
   * 修改 `service/agent.go` 的 `HeartbeatNode()` 返回 `AgentSettings`
   * 新增 `AgentSettings` 结构体
   * 修改 `controller/agent.go` 心跳接口返回 `agent_settings`
2. Agent 端：
   * 修改 `protocol/agent_api.go` 新增 `HeartbeatResponse` 和 `AgentSettings`
   * 修改 `httpclient/client.go` 解析心跳响应
   * 修改 `heartbeat/service.go` 返回 `HeartbeatResponse`
   * 修改 `agent/runner.go` 根据响应动态调整 `heartbeatTicker` 和 `syncTicker`

验收标准：

* Server 心跳响应 JSON 中包含 `agent_settings`
* Agent 收到新间隔后在下一个周期生效
* Agent 重启后恢复 `agent.json` 配置，再由心跳覆盖
* Server 未配置时 Agent 保持本地值不变

### 3.3 阶段三：Agent 自我更新

目标：Agent 支持从 GitHub Releases 自动更新。

实施步骤：

1. Agent 新增 `internal/updater` 模块：
   * GitHub Releases API 查询最新版本
   * 版本比较（语义化版本）
   * 下载对应平台二进制
   * 替换自身并重启（exec syscall）
2. 在 `runner.go` 心跳循环中集成更新检查
3. 更新触发条件：节点 `auto_update=true` 或收到一次性 `update_now=true` 且存在新版本

验收标准：

* Agent 能正确检测新版本
* 能下载并替换自身二进制
* 更新后自动重启并恢复心跳
 * 默认不自动更新，需由控制面板逐节点开启或手动触发
* 更新失败不影响正常运行

### 3.4 阶段四：GitHub Actions 完善与 Agent 一键部署

目标：CI 支持 Agent 构建发布，提供 curl 一键安装。

实施步骤：

1. 新增 `.github/workflows/agent-release.yml`
2. 修改现有工作流支持 alpha/prerelease
3. 创建 `scripts/install-agent.sh` 安装脚本
4. 前端运维设置面板展示动态 curl 部署命令

验收标准：

* 推送 tag 后 Agent 二进制出现在 GitHub Release
* Alpha tag 标记为 prerelease
* curl 命令可在干净 Linux 机器上完成 Agent 部署
* 前端正确展示拼接后的 curl 命令

### 3.5 阶段五：前端体验优化

目标：优化管理端操作体验。

实施步骤：

1. 节点列表时间显示改为友好格式
2. 节点状态颜色标识
3. 节点专属 Agent 部署命令改为节点列表弹窗展示
4. 版本检查入口迁移到顶栏“版本”，支持可升级提示与 Server 自升级

验收标准：

* 时间显示为友好的相对时间（如「2 分钟前」）
* 节点状态有颜色区分：在线（绿色）、离线（红色）、待接入（黄色）
* 节点列表可弹窗查看并复制节点专属部署命令
* 顶栏版本入口可检查 GitHub 最新 Release，并在存在新版本时显示可升级提示
* Root 用户可从顶栏版本弹窗触发 Server 自升级

### 3.6 并行专项：管理端前端工程改造

目标：按 [docs/frontend-revamp-plan.md](./frontend-revamp-plan.md) 推进新版管理端重建，不阻塞 Server/Agent 主链路。

实施约束：

1. 前端专项按独立阶段推进，当前从“阶段 1：工程初始化”开始执行
2. 首期产物必须保持静态导出，并继续由 `atsf_server` 托管
3. 认证迁移、业务模块迁移与最终切换按 `docs/frontend-revamp-plan.md` 的阶段顺序执行

验收标准：

* 新前端可本地开发、可静态构建
* Go Server 可继续托管新版构建产物
* 前端专项与现有第三版主链路互不破坏

---

## 4. 阶段执行原则

* 每个阶段完成后验证验收标准，再进入下一阶段
* 阶段间的代码不相互依赖时可并行
* 每个阶段完成后运行全量测试
* 管理端前端改造按 [docs/frontend-revamp-plan.md](./frontend-revamp-plan.md) 单独跟踪阶段状态
