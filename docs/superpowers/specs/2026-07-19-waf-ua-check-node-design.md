# WAF 规则节点：UA 检查（ua_check）

日期：2026-07-19  
范围：WAF 编排图新节点 `ua_check`（控制面校验/编译 + 边缘 Lua 运行时 + 前端编辑器）  
状态：已确认，待实现

## 背景

访问日志概览已按 User-Agent 分类浏览器与操作系统（`internal/repository/analytics/browser.go`），但 WAF 规则图尚无基于 UA 的分支节点。运营需要在图中：

1. 要求请求必须携带 UA；
2. 按浏览器 / 操作系统做白名单匹配（and/or 可配）；
3. 优先屏蔽常见爬虫与非正常 UA。

## 目标

- 新增 match 型节点 **`ua_check`**，输出 `true` / `false` 句柄（与 `ip_match` / `geo_match` 一致）。
- 属性栏交互与产品草图对齐：开启 UA 检查、匹配多选、屏蔽开关。
- 边缘分类标签与访问日志概览一致（同一套 token 规则）。
- 屏蔽逻辑优先级高于白名单匹配。

## 非目标

- 设备类型（Mobile/Tablet）维度。
- 原始 UA 正则 / 自由子串列表（PoW 列表已有，不并入本节点）。
- 在 Server 请求路径上执行 WAF 图（仍仅 Agent OpenResty）。
- 将 analytics 包直接 import 到 Agent（边缘用 Lua 复刻规则；Go 侧用同一规则表做校验与单测对拍）。

## 节点模型

### 类型

| 字段 | 值 |
|------|-----|
| `type` | `ua_check` |
| 句柄 | `true`, `false` |
| 可删除 | 是 |
| 可命名 | 是（`label`） |
| 可拖放添加 | 是 |

### Config（JSON）

```json
{
  "require_ua": false,
  "browsers": [],
  "operating_systems": [],
  "match_mode": "or",
  "block_common_bots": false,
  "block_abnormal_ua": false
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `require_ua` | bool | 开启后：请求头无 UA（空 / 仅空白）→ **false** |
| `browsers` | string[] | 白名单浏览器标签；空表示不限制浏览器 |
| `operating_systems` | string[] | 白名单操作系统标签；空表示不限制 OS |
| `match_mode` | `"and"` \| `"or"` | **浏览器条件与 OS 条件**之间的组合；默认 `"or"` |
| `block_common_bots` | bool | 屏蔽常见爬虫：分类 browser 或 os 为 `Bot` → **false** |
| `block_abnormal_ua` | bool | 屏蔽非正常 UA：browser ∈ `{Bot, Other, Unknown}` → **false** |

默认值：开关全 `false`，列表空，`match_mode: "or"`。

### 允许的标签（封闭枚举）

与 `ParseBrowserName` / `ParseOSName` 输出对齐：

**browsers：**  
`Chrome`, `Safari`, `Firefox`, `Edge`, `Opera`, `Chromium`, `WeChat`, `Postman`, `CLI`, `Bot`, `Unknown`, `Other`

**operating_systems：**  
`Android`, `iOS`, `Windows`, `macOS`, `Chrome OS`, `Linux`, `Bot`, `Unknown`, `Other`

校验：列表元素必须属于上表；重复项编译时去重排序；未知字符串拒绝保存。

## 求值语义（边缘）

输入：`ua = http_user_agent`（trim 后判断空）。  
分类：`browser = ParseBrowserName(ua)`，`os = ParseOSName(ua)`（空 UA → 二者均为 `Unknown`，与 analytics 一致）。

**严格顺序：**

```
1) if require_ua and ua 为空 → false
2) browser, os := classify(ua)
3) if block_common_bots and (browser == "Bot" or os == "Bot") → false
4) if block_abnormal_ua and browser in {"Bot","Other","Unknown"} → false
5) has_browsers := browsers 非空; has_os := operating_systems 非空
6) if not has_browsers and not has_os → true
7) browser_hit := browser ∈ browsers; os_hit := os ∈ operating_systems
8) if has_browsers and not has_os → browser_hit
9) if has_os and not has_browsers → os_hit
10) if both lists set:
      match_mode == "and" → browser_hit and os_hit
      match_mode == "or"  → browser_hit or os_hit
```

说明：

- **屏蔽优先于匹配**：步骤 3–4 在白名单之前。
- **未配置匹配列表**：步骤 6 直接 true（仅受 require / block 约束）。
- **仅一侧列表有值**：只校验该侧是否命中；`match_mode` 仅在两侧都有值时生效。
- 节点本身不 allow/block，仅选句柄；下游连线决定动作。

### 示例

| 配置摘要 | 请求 | 结果 |
|----------|------|------|
| 仅 `require_ua` | 无 UA | false |
| 仅 `require_ua` | 正常 Chrome | true |
| `block_common_bots` | Googlebot | false |
| `block_abnormal_ua` | 无法识别 UA | false |
| browsers=`[Chrome]`, mode=or | Safari | false |
| browsers=`[Chrome]`, os=`[iOS]`, mode=and | Chrome Desktop | false（os 未命中） |
| browsers=`[Chrome]`, os=`[iOS]`, mode=or | Chrome Desktop | true |
| 列表皆空，无 block | 任意有 UA | true |

## 分类规则来源

权威实现（analytics）：`internal/repository/analytics/browser.go` 中 `browserRules` / `osRules`。

实现要求：

1. **Lua 运行时**复刻相同 token 顺序与 `contains` / `noneOf` 语义（lower-case 子串）。
2. **Go 单测**用同一批样例 UA 对拍 `ParseBrowserName` / `ParseOSName` 与 Lua 或共享测试表，防止漂移。
3. 不强制本迭代抽取共享包；若抽取，须保持 analytics 与 WAF 行为不变。

## 控制面

### `graph_types.go`

- `RuleNodeUACheck RuleNodeType = "ua_check"`
- `UACheckConfig` 结构体对应上表 JSON 字段

### `graph_validate.go`

- `requiredHandles`: `true`, `false`
- `validateUACheckNodeConfig`：
  - `match_mode` 仅 `and`/`or`（缺省按 `or` 或拒绝非法值）
  - browsers / OS 标签 ∈ 封闭枚举
  - 布尔字段默认 false
  - `DisallowUnknownFields`

### `graph_compile.go`

- 编译进 `RuntimeRuleNode`，列表 `sortedUniqueStrings`
- 规范化 `match_mode`（非法不得编译成功）

### 测试

- validate：合法配置、非法标签、非法 mode、缺句柄
- compile：列表排序去重、默认值

## 数据面（Agent）

### `waf_runtime.lua`

在 `execute_graph` 增加：

```lua
elseif node.type == "ua_check" then
  handle = matches_ua_check(node.config) and "true" or "false"
```

实现 `matches_ua_check` + 本地 classify 函数；读取 `ngx.var.http_user_agent`。

### `waf_runtime_spec.lua`

覆盖：空 UA + require；bot 屏蔽；abnormal；whitelist and/or；列表空；损坏边 fail-closed。

## 前端编辑器

| 文件 | 变更 |
|------|------|
| `types.ts` | `ua_check` 变体 + `UACheckConfig` |
| `node-factory.ts` | 标签「UA 检查」、默认 config、`AddableNodeType` |
| `node-library.tsx` | 拖放项 |
| `rule-node.tsx` | 图标 + `true`/`false` handles |
| `node-properties.tsx` | 属性 UI（见下） |
| `graph-validation.ts` | handles + 标签/mode 校验 |
| `editor-behavior.ts` | connection handles |

### 属性栏布局

```
显示名称
── UA 检查 ──
[Switch] 开启 UA 检查
  说明：开启后如果请求头不携带 UA 返回 False
── UA 匹配 ──
匹配模式  Select: 或(or) / 且(and)
浏览器    MultiSelect（封闭枚举）
操作系统  MultiSelect（封闭枚举）
── 屏蔽 ──
说明：命中返回 false，优先级高于匹配
[Switch] 屏蔽常见爬虫 UA
[Switch] 屏蔽非正常 UA
```

前端选项列表写死与封闭枚举一致；展示可用中文副标题，**写入 config 的值必须是英文标签**（与 analytics / 边缘一致）。

## 文档

- 更新 `docs/design/waf-orchestration-design.md` 节点表（中文）。
- `docs/changelog/index.md` `[Unreleased]` 增加用户向说明。
- 纯设计文档不写 changelog 以外的英文同步。

## 验收标准

- [ ] 编辑器可拖入 `ua_check`，配置保存再打开一致。
- [ ] 图校验拒绝非法标签与非法 `match_mode`。
- [ ] 发布后 Agent Lua 按求值顺序分支；spec 全绿。
- [ ] 样例 UA 分类与访问日志 `ParseBrowserName`/`ParseOSName` 一致。
- [ ] `make code-check` 与相关 Go/前端/Lua 测试通过。

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| Go/Lua 分类漂移 | 共享样例表单测对拍 |
| 「非正常」过严误伤 | 产品定义为 Bot/Other/Unknown；可关 switch |
| 白名单 + or 过宽 | UI 说明 and/or；默认 or 且列表空不限制 |
