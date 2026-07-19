import type { WAFRuleGraph, WAFRuleNode } from '@/lib/services/openflare';

const NODE_WIDTH = 220;
const NODE_HEIGHT = 72;
const GAP_X = 120;
const GAP_Y = 48;
const ORIGIN_X = 48;
const ORIGIN_Y = 48;

/** Left-to-right layered layout for the WAF rule DAG. Edges are unchanged. */
export function layoutRuleGraph(graph: WAFRuleGraph): WAFRuleGraph {
  if (graph.nodes.length === 0) return graph;

  const children = new Map<string, string[]>();
  const indegree = new Map<string, number>();
  for (const node of graph.nodes) {
    children.set(node.id, []);
    indegree.set(node.id, 0);
  }
  for (const edge of graph.edges) {
    if (!children.has(edge.source) || !indegree.has(edge.target)) continue;
    children.get(edge.source)!.push(edge.target);
    indegree.set(edge.target, (indegree.get(edge.target) ?? 0) + 1);
  }

  const start =
    graph.nodes.find((node) => node.type === 'start') ?? graph.nodes[0];
  const depth = new Map<string, number>();
  const queue: string[] = [start.id];
  depth.set(start.id, 0);

  while (queue.length > 0) {
    const id = queue.shift()!;
    const d = depth.get(id) ?? 0;
    for (const child of children.get(id) ?? []) {
      const next = d + 1;
      const prev = depth.get(child);
      if (prev === undefined || next > prev) {
        depth.set(child, next);
        queue.push(child);
      }
    }
  }

  // Unreachable nodes (no path from start) sit after the main layers.
  let maxDepth = 0;
  for (const value of depth.values()) maxDepth = Math.max(maxDepth, value);
  let orphanColumn = maxDepth + 1;
  for (const node of graph.nodes) {
    if (!depth.has(node.id)) {
      depth.set(node.id, orphanColumn);
      orphanColumn += 1;
    }
  }

  const columns = new Map<number, WAFRuleNode[]>();
  for (const node of graph.nodes) {
    const col = depth.get(node.id) ?? 0;
    const list = columns.get(col) ?? [];
    list.push(node);
    columns.set(col, list);
  }

  for (const [, list] of columns) {
    list.sort((a, b) => {
      const rank = (node: WAFRuleNode) => {
        if (node.type === 'start') return 0;
        if (node.type === 'allow') return 2;
        if (node.type === 'block') return 3;
        return 1;
      };
      const diff = rank(a) - rank(b);
      if (diff !== 0) return diff;
      return a.id.localeCompare(b.id);
    });
  }

  let maxRows = 1;
  for (const list of columns.values()) maxRows = Math.max(maxRows, list.length);

  const positions = new Map<string, { x: number; y: number }>();
  const sortedCols = [...columns.keys()].sort((a, b) => a - b);
  for (const col of sortedCols) {
    const list = columns.get(col) ?? [];
    const blockHeight =
      list.length * NODE_HEIGHT + Math.max(0, list.length - 1) * GAP_Y;
    const totalHeight =
      maxRows * NODE_HEIGHT + Math.max(0, maxRows - 1) * GAP_Y;
    const offsetY = ORIGIN_Y + (totalHeight - blockHeight) / 2;
    list.forEach((node, index) => {
      positions.set(node.id, {
        x: ORIGIN_X + col * (NODE_WIDTH + GAP_X),
        y: offsetY + index * (NODE_HEIGHT + GAP_Y),
      });
    });
  }

  return {
    ...graph,
    nodes: graph.nodes.map((node) => ({
      ...node,
      position: positions.get(node.id) ?? node.position,
    })),
  };
}
