# 限流页请求压力分析设计

日期：2026-07-19  
状态：已评审待实现  
方案：Tabs（分析 / 配置）+ 专用 ECharts 双轴压力图（方案 A）

## 背景

`/rate-limits` 当前仅为管理员配置全局 OpenResty 默认限流（`limit_conn_*` / `limit_rate`），无请求压力可视化。

访问日志概览已提供：

- 过滤：时间预设 `24h | 7d | 15d | 30d` + 域名多选 `hosts[]`
- 数据：`GET /api/v1/d/access-logs/overview` → 小时桶 `trends.requests` / `trends.visits`，以及 `top_hosts` / `top_ips`（窗口总请求数）
- 图表：共享 `TrendChart` 为**单 Y 轴**；仓库内无 ECharts `dataZoom`、无双轴指标图

需求：在限流页展示当前请求压力（RPS），默认 24 小时，图表样式对齐「双轴时序面积折线 + 底部缩放条」描述，过滤复用访问日志概览组件，并增加域名/IP 平均 RPS 排行。

## 目标

1. `/rate-limits` 改为 Tabs：**分析**（默认）/ **配置**。
2. 分析 Tab：概览式过滤 + RPS/访客双轴主图 + 域名/IP 平均 RPS 排行。
3. 配置 Tab：迁入现有全局默认限流表单，行为不变。
4. 数据复用 `AccessLogService.getOverview`，不新增后端 API。
5. 主图为**专用** ECharts 组件，不扩展共享 `TrendChart`。

## 非目标

- 新 RPS 时序 API 或峰值桶 RPS 排行接口
- 给通用 `TrendChart` 增加双轴 / dataZoom
- 配置 Tab 限流语义变更
- 分析过滤支持 node_id / IP / path（概览亦无）
- 英文文档

## 页面信息架构

**路由：** `/rate-limits`（导航「安全性 → 限流」不变）

| Tab | 内容 |
|-----|------|
| **分析**（默认） | 过滤条 → `RatePressureChart` → 双排行榜 |
| **配置** | 现有三项全局默认限流表单 + 保存 + 链到版本发布 |

可选：`?tab=config` 直达配置；默认 `analysis`。

**权限：** 仅管理员（与现页一致）。

**分析 Tab 自上而下：**

1. **过滤条**（与访问日志概览一致）
   - 时间：`24 | 168 | 360 | 720` 小时，默认 **24**
   - 域名：Zone 树多选 → `hosts[]`
2. **主图卡片** `RatePressureChart`
3. **排行榜**（并排）
   - 平均 RPS 最高域名
   - 平均 RPS 最高 IP

## 数据与状态

### 查询

```ts
AccessLogService.getOverview({
  hours: overviewHours,
  hosts: overviewHosts.length > 0 ? overviewHosts : undefined,
})
// queryKey: ['openflare', 'rate-limits', 'overview', hours, hosts]
```

- 过滤变更 → 重新请求 overview
- 图表 `dataZoom` **仅**前端缩放已加载序列，**不**改 `hours`、**不**触发 refetch

### 指标定义

| 序列 | 源字段 | 换算 | 轴 |
|------|--------|------|-----|
| 请求速率 (RPS) | `trends.requests[].value` | `value / 3600`（概览固定 1h 桶） | 左 Y |
| 独立访客 | `trends.visits[].value` | 桶内 UV，不换算 | 右 Y |

- 时间点：`bucket_started_at`
- Tooltip：时间 + RPS（如 `12.3 req/s`）+ 访客数
- 空数据 / 加载 / 错误：对齐访问日志概览空态与 `ErrorInline` / loading

### 排行口径

窗口**平均** RPS（与 dashboard `estimated_qps` 一致）：

```
avgRps = total_requests / (hours * 3600)
```

- 域名：`top_hosts[]` 的 `value` 为窗口总请求数 → 换算后展示
- IP：`top_ips[]` 同理
- 标题：「平均 RPS 最高域名」「平均 RPS 最高 IP」
- 副文案标明窗口（如「近 24 小时平均」）
- UI 组件：现有 `RankCard` / `RankChart`

**不是**峰值小时桶 RPS；避免新 API。

## 主图组件 `RatePressureChart`

### 布局（对齐产品描述）

1. **外部卡片**：圆角、边框/轻阴影，扁平矩形
2. **顶部控制栏**
   - 左：主标题「请求压力」（字号加粗）
   - 右：时钟图标 + 当前查询窗口起止（由 `hours` 与「现在」推算本地时间，`YYYY-MM-DD HH:mm:ss`）
3. **图例与轴标识**
   - 左上：左轴属性「RPS」
   - 右上：图例圆点 +「请求速率」「独立访客」
   - 最右：右轴单位「访客 / 桶」
4. **主绘制区**
   - 双 Y 轴：左 RPS 从 0 递增；右访客从 0 递增
   - X 轴：时间，标签两行（月-日 / 时:分），可复用 `formatOverviewTrendLabel` 思路
   - 水平等距虚线网格
   - 面积 + 折线，半透明填充，两序列可重叠
5. **底部 dataZoom slider**
   - ECharts `dataZoom: [{ type: 'slider', ... }]`
   - 宽度对齐绘图区；左右手柄；内嵌缩略波动线
   - 仅影响可见区间

### 实现约束

- 新建专用组件，**不要**给 `TrendChart` 加 dualY/dataZoom
- 库：`echarts` + `echarts-for-react`（与看板一致）
- 颜色使用主题/CSS 变量或与访问日志趋势相近的语义色，避免硬编码与 shadcn 变体冲突时可参考现有 `TrendChart` 系列色

## 过滤组件复用

优先从 `frontend/app/(main)/access-logs/components/overview-tab.tsx` **抽出**：

- `OverviewToolbar`（或等价）
- `OverviewHostFilter`
- 依赖的 `OVERVIEW_RANGE_OPTIONS` / `OverviewRangeHours` 已在 `access-log-utils.ts`

落点建议：

- 仍放在 `access-logs/components/` 并 export，限流分析 import；或
- 若跨模块更清晰，迁到 `frontend/components/common/`（仅当确实跨页面复用且避免循环依赖时）

**验收：** 访问日志概览过滤行为与抽出前一致。

## 文件结构

```
frontend/app/(main)/rate-limits/
  page.tsx                      # Tabs、权限、分析/配置挂载
  components/
    analysis-tab.tsx            # 过滤 + 图 + 排行 + overview query
    rate-pressure-chart.tsx     # 双轴 + dataZoom
    config-tab.tsx              # 现有 Option 表单逻辑迁入
```

可选抽出：

```
frontend/app/(main)/access-logs/components/
  overview-toolbar.tsx          # 从 overview-tab 抽出
  overview-host-filter.tsx
```

后端：无变更。

## 边界与兼容

| 场景 | 行为 |
|------|------|
| 无日志 / ClickHouse 空 | 图与排行空态 |
| 仅选域名 | overview 带 `hosts[]` |
| dataZoom 拖动 | 不请求后端 |
| 非管理员 | 空态「权限不足」 |
| 书签 `/rate-limits` | 默认分析 Tab |
| 配置保存 | 仍 invalidate options / config-preview / config-versions |

## 测试与验收

### 自动化（按项目习惯）

- 若有 vitest：过滤 props 透传、`avgRps` 换算纯函数单测
- 图表以手工/视觉验收为主（ECharts 难做快照）

### 验收标准

1. 默认进入分析 Tab，24h，主图展示 RPS + 访客
2. 切换 7d / 域名后图与排行刷新
3. dataZoom 仅改变可见时间范围
4. 排行展示平均 RPS，不是原始请求总数（文案明确「平均」）
5. 配置 Tab 可读写三项默认限流并保存
6. 访问日志概览过滤不回归
7. `make prettier`；相关 typecheck/lint 通过

## 文档

- 本设计：`docs/superpowers/specs/2026-07-19-rate-limit-analytics-design.md`
- 实现时：`docs/changelog/index.md` `[Unreleased]` 补充用户可见条目
- 纯 UI/分析展示，无新 system config 键

## 实现落点索引

| 区域 | 路径 |
|------|------|
| 限流页 | `frontend/app/(main)/rate-limits/` |
| 概览过滤复用 | `access-logs/components/overview-tab.tsx` 等 |
| Overview API | `AccessLogService.getOverview` |
| 排行 UI | `components/data/rank-card.tsx` |
| 趋势参考 | `components/data/trend-chart.tsx`（只参考样式，不扩展） |

## 决策摘要

| 决策 | 选择 |
|------|------|
| 页面结构 | Tabs：分析 / 配置 |
| 双轴 | 左 RPS，右 独立访客/桶 |
| 过滤 | 概览过滤 + 默认 24h 预设 |
| 排行 | 窗口平均 RPS = 总请求 / 窗口秒数 |
| 图表实现 | 专用 ECharts 组件（方案 A） |
| 后端 | 无新 API |
