# WAF Editor Node Label + Drag-Add Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users rename WAF rule nodes via optional `label`, and add nodes by dragging from the library onto the canvas drop position (no click-to-fixed-offset).

**Architecture:** Frontend-only. Align TS `WAFRuleNode` with backend `label`. Pure helpers for display name and default node factory. Node library is drag source; React Flow pane handles drop with `screenToFlowPosition`. Properties panel edits `label` for non-system nodes.

**Tech Stack:** Next.js App Router, React, TypeScript, `@xyflow/react`, Vitest + Testing Library, shadcn/ui.

**Spec:** `docs/superpowers/specs/2026-07-19-waf-editor-node-label-drag-design.md`

## Global Constraints

- No backend / schema_version / note field changes.
- System nodes `start` / `allow`: no rename UI.
- New nodes: no default `label` (type name shown).
- Drag-only add; remove click-add.
- After code: relevant vitest pass; run `make prettier` / `make code-check` if touching repo gates.

## File Map

| File | Role |
|------|------|
| `frontend/lib/services/openflare/types.ts` | Add `label?: string` to all `WAFRuleNode` variants |
| `frontend/app/(main)/waf/rules/editor/components/node-factory.ts` | `NODE_TYPE_LABELS`, `displayNodeTitle`, `createRuleNode`, drag MIME constant |
| `frontend/app/(main)/waf/rules/editor/components/node-factory.test.ts` | Unit tests for title + factory |
| `frontend/app/(main)/waf/rules/editor/components/rule-node.tsx` | Use `displayNodeTitle` |
| `frontend/app/(main)/waf/rules/editor/components/node-properties.tsx` | 「显示名称」Input |
| `frontend/app/(main)/waf/rules/editor/components/node-properties.test.tsx` | Label edit + system node |
| `frontend/app/(main)/waf/rules/editor/components/node-library.tsx` | Draggable items, no onClick |
| `frontend/app/(main)/waf/rules/editor/components/rule-flow-canvas.tsx` | Drop handler + position-aware create |

---

### Task 1: Types + pure helpers

**Files:**
- Modify: `frontend/lib/services/openflare/types.ts`
- Create: `frontend/app/(main)/waf/rules/editor/components/node-factory.ts`
- Create: `frontend/app/(main)/waf/rules/editor/components/node-factory.test.ts`

**Interfaces:**
- Produces: `WAF_NODE_DRAG_MIME`, `AddableNodeType`, `NODE_TYPE_LABELS`, `displayNodeTitle(node)`, `createRuleNode(type, position)`

- [ ] **Step 1: Add `label?: string` to every `WAFRuleNode` union member** in `types.ts`.

- [ ] **Step 2: Write failing tests** in `node-factory.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import {
  createRuleNode,
  displayNodeTitle,
  NODE_TYPE_LABELS,
} from './node-factory';

describe('displayNodeTitle', () => {
  it('uses trimmed label when present', () => {
    expect(
      displayNodeTitle({
        id: 'x',
        type: 'ip_match',
        label: '  办公室  ',
        position: { x: 0, y: 0 },
        config: { ips: [], cidrs: [], ip_group_ids: [] },
      }),
    ).toBe('办公室');
  });

  it('falls back to type default when label empty', () => {
    expect(
      displayNodeTitle({
        id: 'x',
        type: 'block',
        label: '  ',
        position: { x: 0, y: 0 },
        config: { status_code: 403, response_body: '' },
      }),
    ).toBe(NODE_TYPE_LABELS.block);
  });
});

describe('createRuleNode', () => {
  it('creates typed node at position without label', () => {
    const node = createRuleNode('pow', { x: 12, y: 34 });
    expect(node.type).toBe('pow');
    expect(node.position).toEqual({ x: 12, y: 34 });
    expect(node.label).toBeUndefined();
    expect(node.id.startsWith('pow-')).toBe(true);
    if (node.type === 'pow') {
      expect(node.config).toEqual({
        algorithm: 'fast',
        difficulty: 4,
        session_ttl: 3600,
        challenge_ttl: 300,
      });
    }
  });
});
```

- [ ] **Step 3: Implement `node-factory.ts`**

```ts
import type { WAFRuleNode } from '@/lib/services/openflare';

export const WAF_NODE_DRAG_MIME = 'application/openflare-waf-node';

export type AddableNodeType = Extract<
  WAFRuleNode['type'],
  'ip_match' | 'geo_match' | 'pow' | 'block'
>;

export const NODE_TYPE_LABELS: Record<WAFRuleNode['type'], string> = {
  start: '开始',
  ip_match: 'IP 匹配',
  geo_match: '地域匹配',
  pow: 'PoW 挑战',
  allow: '通过',
  block: '阻止',
};

export function displayNodeTitle(
  node: Pick<WAFRuleNode, 'type' | 'label'>,
): string {
  const custom = node.label?.trim();
  return custom || NODE_TYPE_LABELS[node.type];
}

export function createRuleNode(
  type: AddableNodeType,
  position: { x: number; y: number },
): WAFRuleNode {
  const id = `${type}-${crypto.randomUUID().slice(0, 8)}`;
  if (type === 'ip_match')
    return {
      id,
      type,
      position,
      config: { ips: [], cidrs: [], ip_group_ids: [] },
    };
  if (type === 'geo_match')
    return { id, type, position, config: { countries: [], regions: [] } };
  if (type === 'pow')
    return {
      id,
      type,
      position,
      config: {
        algorithm: 'fast',
        difficulty: 4,
        session_ttl: 3600,
        challenge_ttl: 300,
      },
    };
  return {
    id,
    type: 'block',
    position,
    config: { status_code: 403, response_body: '' },
  };
}

export function parseAddableNodeType(value: string): AddableNodeType | null {
  if (
    value === 'ip_match' ||
    value === 'geo_match' ||
    value === 'pow' ||
    value === 'block'
  )
    return value;
  return null;
}
```

- [ ] **Step 4: Run tests**

```bash
cd frontend && pnpm vitest run 'app/(main)/waf/rules/editor/components/node-factory.test.ts'
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/lib/services/openflare/types.ts \
  frontend/app/(main)/waf/rules/editor/components/node-factory.ts \
  frontend/app/(main)/waf/rules/editor/components/node-factory.test.ts
git commit -m "feat(waf): add node label type and factory helpers"
```

---

### Task 2: Canvas title + properties label field

**Files:**
- Modify: `frontend/app/(main)/waf/rules/editor/components/rule-node.tsx`
- Modify: `frontend/app/(main)/waf/rules/editor/components/node-properties.tsx`
- Modify: `frontend/app/(main)/waf/rules/editor/components/node-properties.test.tsx`

- [ ] **Step 1: Tests for properties**

Add to `node-properties.test.tsx`:

```ts
it('edits display name for configurable nodes', () => {
  const node: WAFRuleNode = {
    id: 'match',
    type: 'ip_match',
    position: { x: 0, y: 0 },
    config: { ips: [], cidrs: [], ip_group_ids: [] },
  };
  const onChange = vi.fn();
  render(<NodeProperties node={node} ipGroups={[]} onChange={onChange} />);
  fireEvent.change(screen.getByLabelText('显示名称'), {
    target: { value: '内网放行' },
  });
  expect(onChange).toHaveBeenCalledWith(
    expect.objectContaining({ label: '内网放行' }),
  );
});

it('hides display name for system nodes', () => {
  const node: WAFRuleNode = {
    id: 'start',
    type: 'start',
    position: { x: 0, y: 0 },
    config: {},
  };
  render(<NodeProperties node={node} ipGroups={[]} onChange={vi.fn()} />);
  expect(screen.queryByLabelText('显示名称')).not.toBeInTheDocument();
  expect(screen.getByText('系统节点无需配置。')).toBeInTheDocument();
});
```

- [ ] **Step 2: Implement properties field** — at start of each configurable `FieldGroup` (or wrap once before type switch for non-system):

Prefer extract:

```tsx
function DisplayNameField({
  node,
  onChange,
}: {
  node: WAFRuleNode;
  onChange: (node: WAFRuleNode) => void;
}) {
  return (
    <Field>
      <FieldLabel htmlFor={`${node.id}-label`}>显示名称</FieldLabel>
      <Input
        id={`${node.id}-label`}
        value={node.label ?? ''}
        placeholder={/* type default from NODE_TYPE_LABELS */}
        onChange={(e) => onChange({ ...node, label: e.target.value })}
      />
    </Field>
  );
}
```

Insert `<DisplayNameField ... />` as first child inside each non-system `FieldGroup`.

- [ ] **Step 3: `rule-node.tsx`** — use `displayNodeTitle(rule)` for main title; keep icon from meta; keep id subtitle.

- [ ] **Step 4: Run tests**

```bash
cd frontend && pnpm vitest run 'app/(main)/waf/rules/editor/components/node-properties.test.tsx' 'app/(main)/waf/rules/editor/components/node-factory.test.ts'
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/app/(main)/waf/rules/editor/components/rule-node.tsx \
  frontend/app/(main)/waf/rules/editor/components/node-properties.tsx \
  frontend/app/(main)/waf/rules/editor/components/node-properties.test.tsx
git commit -m "feat(waf): show and edit rule node display names"
```

---

### Task 3: Drag library + canvas drop

**Files:**
- Modify: `frontend/app/(main)/waf/rules/editor/components/node-library.tsx`
- Modify: `frontend/app/(main)/waf/rules/editor/components/rule-flow-canvas.tsx`
- Create (optional pure tests): extend `node-factory.test.ts` for `parseAddableNodeType`

- [ ] **Step 1: Node library** — remove `onAdd` prop; make each button `draggable` with:

```tsx
onDragStart={(e) => {
  e.dataTransfer.setData(WAF_NODE_DRAG_MIME, type);
  e.dataTransfer.setData('text/plain', type);
  e.dataTransfer.effectAllowed = 'copy';
}}
```

Use `type='button'` + cursor `cursor-grab active:cursor-grabbing`. No `onClick` that adds nodes.

- [ ] **Step 2: Canvas** — replace `addNode(type)` fixed position with:

```ts
const addNodeAt = useCallback(
  (type: AddableNodeType, position: { x: number; y: number }) => {
    const node = createRuleNode(type, position);
    onGraphChange({ ...graph, nodes: [...graph.nodes, node] });
    onSelectEdge(undefined);
    onSelect(node.id);
  },
  [graph, onGraphChange, onSelect, onSelectEdge],
);

const onDragOver = useCallback((e: React.DragEvent) => {
  e.preventDefault();
  e.dataTransfer.dropEffect = 'copy';
}, []);

const onDrop = useCallback(
  (e: React.DragEvent) => {
    e.preventDefault();
    const raw =
      e.dataTransfer.getData(WAF_NODE_DRAG_MIME) ||
      e.dataTransfer.getData('text/plain');
    const type = parseAddableNodeType(raw);
    if (!type || !instance.current) return;
    const position = instance.current.screenToFlowPosition({
      x: e.clientX,
      y: e.clientY,
    });
    addNodeAt(type, position);
  },
  [addNodeAt],
);
```

Pass `onDragOver` / `onDrop` to `<ReactFlow ...>` (xyflow supports these on the component).

Update `<NodeLibrary />` — no `onAdd`.

- [ ] **Step 3: Run editor-related tests**

```bash
cd frontend && pnpm vitest run 'app/(main)/waf/rules/editor'
```

Expected: PASS (update any tests that assumed click-add)

- [ ] **Step 4: Format + commit**

```bash
make prettier
git add frontend/app/(main)/waf/rules/editor
git commit -m "feat(waf): drag-drop nodes onto rule canvas at cursor"
```

- [ ] **Step 5: Changelog** — under `docs/changelog/index.md` `[Unreleased]`:

```md
### 改进
- WAF 规则编辑器支持为节点自定义显示名称，并从节点库拖放到画布指定位置添加节点。
```

```bash
git add docs/changelog/index.md
git commit -m "docs(changelog): WAF 编辑器节点命名与拖放添加"
```

---

## Spec coverage

| Spec item | Task |
|-----------|------|
| `label?` on TS types | 1 |
| Display title fallback | 1–2 |
| Properties 显示名称 | 2 |
| System nodes no rename | 2 |
| Drag-only library | 3 |
| Drop at cursor | 3 |
| No note / backend | N/A (omitted) |
| Tests | 1–3 |
| Changelog | 3 |
