# WAF 规则节点：安全防护（security_check）

日期：2026-07-19  
范围：WAF 编排图新节点 `security_check`（控制面校验/编译 + 边缘 Lua 特征检测 + 前端编辑器）  
状态：已确认，待实现

## 背景

现有节点覆盖 IP / 地域 / UA / PoW，缺少请求载荷侧的基础攻击特征检测。产品需要在图中提供可编排的「安全防护」单元：多项基础规则可开关，**命中任意已启用规则返回 false**。

检测深度采用 **Lua 内置特征规则**（非 ModSecurity/CRS），能拦截常见扫描与明显 payload，允许有限误报/漏报。

## 目标

1. 新增 match 型节点 **`security_check`**，句柄 `true` / `false`。
2. 属性栏分组：**安全防护**说明 + **基础防护** 9 项 Switch。
3. 语义：**任一已启用规则命中 → false**；全部未命中 → true。
4. 默认仅开启误报较低的两项：**路径穿越**、**文件包含**；其余默认关闭。

## 非目标（v1）

- ModSecurity / OWASP CRS / libinjection 完整引擎
- 响应侧 XSS 检测、机器学习
- 自定义规则上传 / 严重级别评分 / 命中日志字段（可后续加）
- 无限制大 Body 全量扫描

## 节点模型

### 类型

| 字段 | 值 |
|------|-----|
| `type` | `security_check` |
| 句柄 | `true`, `false` |
| 可删除 / 可命名 / 可拖放 | 是 |

### Config

```json
{
  "sql_injection": false,
  "path_traversal": true,
  "command_injection": false,
  "xss": false,
  "ssrf": false,
  "file_inclusion": true,
  "malicious_upload": false,
  "xxe": false,
  "crlf_injection": false
}
```

| 字段 | 默认 | UI 文案 | 检测面（v1） |
|------|------|---------|--------------|
| `sql_injection` | false | SQL 注入 | Query、Body、Header、Cookie |
| `path_traversal` | **true** | 路径穿越防护 | Path、Query、Body |
| `command_injection` | false | 命令注入 | Query、Body、Header |
| `xss` | false | XSS | Query、Body、Header |
| `ssrf` | false | SSRF | Query、Body 中 URL 形态参数 |
| `file_inclusion` | **true** | 文件包含（LFI/RFI） | Path、Query、Body |
| `malicious_upload` | false | 恶意文件上传 | Multipart Body |
| `xxe` | false | XXE | Body（Content-Type 含 xml 时） |
| `crlf_injection` | false | CRLF 注入 | Header、Query、Body |

全部关闭时：节点恒 **true**（空操作），合法。

## 求值语义

```
inputs := collect_inspection_strings(request)  // 见下
for each enabled rule:
  if rule_matches(rule, inputs) → return false
return true
```

- **false** = 命中攻击特征（接阻止）  
- **true** = 未命中（接通过或其它节点）

### 采集与限制

| 来源 | 方式 |
|------|------|
| Path | `ngx.var.uri`，URL 解码（含常见双重编码路径变体） |
| Query | `ngx.var.args` / `get_uri_args` 键与值 |
| Header | 请求头名+值（可排除 `Host` 等噪声，实现时固定白名单或全量） |
| Cookie | `Cookie` 头或 cookie 表 |
| Body | 仅当某已启用规则需要 Body，且 `Content-Length` 存在且 ≤ **65536**；否则跳过 Body 相关规则 |

Body 读取失败：跳过 Body 类检测并限频 warn（可用性优先，不 fail-closed 整图）。

### 规则特征方向（v1 模式包）

实现以可维护的模式表为准，下表为方向约束：

1. **SQL 注入**：`union select`、`or 1=1`、`sleep(`、`benchmark(`、注释符、十六进制/char 拼接等  
2. **路径穿越**：`../`、`..\\`、`%2e%2e`、`%252e`、绝对路径探测  
3. **命令注入**：`;` `|` `` ` `` `$()` 结合 shell 关键字、换行拼接  
4. **XSS**：`<script`、`javascript:`、事件处理器 `onerror=` 等  
5. **SSRF**：内网 IP、`localhost`、`169.254.`、`file://`、`gopher://`、`dict://`  
6. **文件包含**：`php://`、`file://`、`/etc/passwd`、`%00` 等（可与路径穿越重叠）  
7. **恶意上传**：multipart 文件名双扩展、危险扩展、可疑 Content-Type  
8. **XXE**：`<!ENTITY`、`SYSTEM`、外部实体（仅 XML 类 Content-Type）  
9. **CRLF**：`%0d%0a`、裸 `\r\n` 注入特征  

模式在 worker 内缓存；大小写不敏感（除明确大小写敏感的协议串）。

## 控制面

### `graph_types.go`

- `RuleNodeSecurityCheck = "security_check"`
- `SecurityCheckConfig` 九个 `bool` 字段（JSON snake_case 如上）

### `graph_validate.go`

- `requiredHandles`: `true`, `false`
- 严格 JSON；仅允许已知布尔字段

### `graph_compile.go`

- 原样编译布尔字段进运行时配置

### 测试

- 合法全关 / 默认子集 / 全开  
- 未知字段拒绝  
- 编译保留默认

## 数据面

### `waf_runtime.lua`

```lua
elseif node.type == "security_check" then
  handle = matches_security_check(node.config or {}) and "true" or "false"
```

`matches_security_check` 返回 **true 表示安全通过**（未命中），与 `ip_match` 的「条件成立」命名不同，但句柄语义与产品一致：命中攻击 → 走 `false` 边。

建议将模式表与匹配函数放在同文件或 `waf/security.lua`（若体积过大再拆，并在 `waf_assets.go` 嵌入）。

### `waf_runtime_spec.lua`

覆盖：默认配置拦路径穿越；全关放行；SQL/XSS 样例；Body 超限不炸；multipart 文件名危险扩展（若开启）。

## 前端

| 文件 | 变更 |
|------|------|
| `types.ts` | `security_check` + `SecurityCheckConfig` |
| `node-factory.ts` | 默认：path_traversal+file_inclusion true，其余 false |
| `node-library.tsx` | 「安全防护」+ 图标 |
| `rule-node.tsx` | `true`/`false` handles |
| `node-properties.tsx` | 显示名称；分组说明 + 9 Switch（问号 Tooltip） |
| `graph-validation.ts` / `editor-behavior.ts` | handles |

### 属性栏草图

```
显示名称
── 安全防护 ──
  命中任意已启用规则返回 False  [?]
── 基础防护 ──
  [Switch] 路径穿越防护  [?]
  [Switch] 文件包含（LFI/RFI）  [?]
  [Switch] SQL 注入  [?]
  ...
```

Tooltip 文案包含检测面与简要说明（与产品表一致）。

## 文档

- 更新 `docs/design/waf-orchestration-design.md` 节点表  
- `docs/changelog/index.md` `[Unreleased]`

## 验收标准

- [ ] 可拖入并配置 9 开关，默认仅路径穿越+文件包含  
- [ ] 保存/发布后 Agent 执行；命中 → false 边；未命中 → true  
- [ ] 全关恒 true  
- [ ] Lua/Go/前端相关测试与 `make code-check` 通过  

## 风险

| 风险 | 缓解 |
|------|------|
| 误报 | 默认仅开低误报两项；模式偏保守 |
| 漏报 | 文档标明特征检测边界；后续可加强模式 |
| Body 性能 | 64KiB 上限；未启用 Body 规则不读 Body |
| 与路径/包含重叠 | 允许重叠；任一命中即 false |
