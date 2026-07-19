# WAF 规则编辑器：节点命名与拖放添加

日期：2026-07-19  
范围：`/waf/rules/editor` 前端交互与类型对齐  
状态：已确认，待实现

## 背景

当前 WAF 规则流图编辑器有两处体验问题：

1. 画布节点只显示类型固定名称（如「IP 匹配」），无法自定义命名，复杂规则难以区分。
2. 节点库通过点击添加，新节点落在固定偏移位置（`x: 240, y: 140 + n*24`），无法在目标位置放置。

后端 `RuleNode` 已具备 `label` 字段（`json:"label,omitempty"`），前端类型与 UI 尚未消费。

## 目标

1. 用户可为可编辑节点自定义**显示名称**（`label`），画布与属性栏一致展示。
2. 从节点库**拖放到画布**，在鼠标松手处生成节点；**取消点击固定位置添加**。
3. 不做备注字段、不做拖到连线中插入、不改后端 schema / `schema_version`。

## 非目标

- 节点备注 / note / remark
- 拖到边自动拆边插入
- 系统节点 `start` / `allow` 可改名
- 后端校验、编译或运行时语义变更
- 侧栏式节点库大改版

## 数据模型

### 后端（已有，不改）

```go
type RuleNode struct {
    ID       string          `json:"id"`
    Type     RuleNodeType    `json:"type"`
    Label    string          `json:"label,omitempty"`
    Position RulePosition    `json:"position"`
    Config   json.RawMessage `json:"config"`
}
```

`label` 为空则 omit；现有大小限制与图校验保持不变。

### 前端

`WAFRuleNode` 各变体增加可选字段：

```ts
label?: string;
```

- 保存时：空字符串不写入或写 `undefined`，与 `omitempty` 对齐。
- 显示时：`label?.trim() || typeDefaultLabel`。
- 新建节点：不设 `label`（显示类型默认名）。
- `start` / `allow`：属性栏仍为「系统节点无需配置」，不提供改名输入；若历史数据带 `label`，画布仍可按上述规则显示，但不提供编辑入口。

## UI 行为

### 画布节点（`rule-node.tsx`）

| 区域 | 行为 |
|------|------|
| 主标题 | `label` 去空白后非空则用 `label`，否则用类型默认中文名 |
| 副标题 | 仍显示 `rule.id`（mono 小字） |
| 图标 / handle | 不变 |

### 属性栏（`node-properties.tsx`）

对非系统节点（`ip_match` | `geo_match` | `pow` | `block`），在类型专属配置**之上**增加：

- 字段标签：`显示名称`
- 控件：`Input`，受控绑定 `node.label ?? ''`
- 变更：`onChange({ ...node, label: value })`；清空时写 `''` 或去掉字段（实现任选其一，保存序列化时不落空 label）

系统节点保持现有文案。

### 节点库与添加（`node-library.tsx` + `rule-flow-canvas.tsx`）

1. 节点库项设为 `draggable`，`dragstart` 写入节点类型（如 `application/openflare-waf-node` 或等价自定义 MIME + `text/plain` 回退）。
2. 移除 `onClick` → `onAdd(type)` 的点击添加路径。
3. React Flow 画布容器：
   - `onDragOver`：`preventDefault`，允许 drop
   - `onDrop`：读取类型 → `screenToFlowPosition({ x: clientX, y: clientY })` → 创建节点（默认 config 逻辑与现有 `addNode` 相同，但 `position` 为落点）
4. 落点后选中新节点，清除边选中（与现有一致）。
5. 工具栏仍在画布左上角浮动区域，仅改为拖源，不改为侧栏。

## 实现落点（文件）

| 文件 | 变更 |
|------|------|
| `frontend/lib/services/openflare/types.ts` | `WAFRuleNode` 增加 `label?` |
| `frontend/app/(main)/waf/rules/editor/components/rule-node.tsx` | 标题显示逻辑 |
| `frontend/app/(main)/waf/rules/editor/components/node-properties.tsx` | 「显示名称」字段 |
| `frontend/app/(main)/waf/rules/editor/components/node-library.tsx` | 拖放源，去掉点击添加 |
| `frontend/app/(main)/waf/rules/editor/components/rule-flow-canvas.tsx` | drop 落点创建；`addNode` 接受 position |
| 相关 `*.test.tsx` / `*.test.ts` | label 展示/编辑、拖放 payload、落点 |

可选：若序列化路径有显式字段白名单，确认 `label` 会进入保存 payload。

## 错误与边界

- 未知 / 非法 drag type：忽略 drop。
- 落在画布外：不创建。
- 超长 `label`：依赖后端既有图大小/字段限制；前端可不设硬上限，或与常见 Input 一致（如 64–128 字符）——实现阶段若后端有明确上限则对齐。
- Undo/脏检查：`label` 与 `position` 变更走现有 `onGraphChange` 路径，不新增独立历史机制。

## 测试要点

1. 有 `label` 的节点主标题为自定义名；无 `label` 为类型默认名。
2. 属性栏修改 `label` 后 graph 节点更新且画布同步。
3. 节点库项可拖；drop 后节点 `position` 接近 flow 坐标（允许测试中 mock `screenToFlowPosition`）。
4. 不再通过点击节点库按钮创建节点（无 click-add 行为）。
5. 系统节点属性栏仍无「显示名称」。

## 验收标准

- [ ] 可编辑节点可命名，保存再打开名称仍在。
- [ ] 画布显示自定义名（空则类型名）。
- [ ] 仅拖放添加，松手位置为节点位置。
- [ ] 无后端 API / schema 变更；`make code-check` 与相关 vitest 通过。
