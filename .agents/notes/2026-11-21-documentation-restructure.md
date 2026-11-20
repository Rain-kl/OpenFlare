# 2026-11-21 文档体系重构与板块化重排

## 背景与目标
原有文档体系存在以下问题：
1. 导航结构扁平化（单个板块下堆叠 17~21 篇文档，无视觉分区）。
2. 技术陈旧信息残留（部分文档残留 OpenResty / Nginx reload / Lua 共享内存等已迁移历史描述）。
3. 英文文档缺失多篇核心指南与设计文档（如 Pingora Edge 部署、架构设计、开发指引、性能策略、态势地图等）。
4. 开源致谢与依赖列表过于简略，未系统反映项目数十项核心底层与业务依赖。

## 重构方案与子代理分工
依据 `writing-technical-documentation` 与 Diataxis 规范，采用 4 个并行子代理按业务板块划分责任：

### 1. 指南板块 (Guide Section)
- `meta.json` 引入 Fumadocs `---分组名---` 分隔器：划分入门指引、流量代理与加速、边缘安全防护、应用与集成、运维支持五大体系。
- 补齐英文对应文档：`dashboard-map.md`, `performance-policies.md`, `pow-usage.md`。
- 清除陈旧 OpenResty/Nginx 诊断命令与叙述，更新为 Pingora 边缘网关事实机制。
- 深度扩充中英文开源致谢页面（`credits.md`）：系统梳理核心系统（Pingora, Wavelet, FRP, Gin）、后端库（GORM, Goose, Asynq, Lego, OTel, Cobra, Viper, MaxMind, Expr, Zap, ClickHouse, Swaggo）、前端交互栈（Next.js, React 19, Tailwind, shadcn/ui, Radix UI, XYFlow, TanStack, ECharts, Lucide, next-intl）及合规版权声明。

### 2. 部署板块 (Deployment Section)
- 导航重排为架构与概览、组件部署、隧道基础设施、运维维护四大分类。
- 补全英文 Pingora Edge 部署文档（`docs/en/deployment/openflare-edge.md`）。
- 修正流量拓扑图：严格区分控制流（Server ↔ 节点）与数据流（Client ↔ Edge ↔ Origin / Relay ↔ OpenFlared）。
- 移除陈旧的 `quick-start` 死链与误导性描述。

### 3. 设计与架构板块 (Design Section)
- 21 篇设计文档重新聚合为五大架构域：核心架构与拓扑、边缘引擎与节点、安全与防护机制、分发缓存与隧道、可观测性与数据存储。
- 修复 `index.md` 引用不存在规范的死链。
- 清除 OpenResty 路径与代码映射，更新为 Rust Pingora 边缘网关源码结构与快照热同步原子替换模型。
- 补齐英文设计文档：`pingora-edge-design.md`, `pingora-edge-development.md`, `pow-design.md`, `log-language.md`。

### 4. 参考与规格板块 (Reference Section)
- 分类编排：配置与参数、命令行与脚本、接口与协议、性能基准。
- 新建 OpenAPI / Swagger 接口与协议参考指南（`docs/reference/api.md` & `docs/en/reference/api.md`）。
- 清除 `configuration.md` 中已废弃的 30 余项 Nginx 参数，修正 ClickHouse 配置文件挂载路径。
- 更新 CLI 指令参考，覆盖 `openflare-server`、`openflare-edge`、`openflare-relay`、`flared` 最新参数。

## 验证
- 双语相对链接与锚点全面校验，0 死链。
- `docs/` 执行 `pnpm typecheck` 与 `pnpm run build`，109 个静态页面全部成功编译导出（SSG）。
