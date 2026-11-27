import type {
  SmartDNSFlowGraph,
  SmartDNSFlowNode,
} from '@/lib/services/openflare';

export function createDefaultFlowGraph(strategy = 'smart'): SmartDNSFlowGraph {
  const isLB = strategy === 'load_balance';
  return {
    nodes: [
      {
        id: 'input',
        type: 'input',
        label: 'DNS Query',
        position: { x: 40, y: 120 },
        data: {},
      },
      {
        id: 'cond-1',
        type: 'condition',
        label: 'Geo / Network Match',
        position: { x: 260, y: 110 },
        data: {
          hit_policy: 'first',
          cases: [
            {
              id: 'case-cn',
              label: 'Mainland China',
              condition: "client.country == 'CN'",
            },
            {
              id: 'case-default',
              label: 'Default Fallback',
              condition: '',
            },
          ],
        },
      },
      {
        id: 'action-cn',
        type: 'action',
        label: 'CN Low Latency',
        position: { x: 540, y: 40 },
        data: {
          mode: isLB ? 'load_balance' : 'smart',
          weights: isLB
            ? { latency: 0, load: 1, geo: 0 }
            : { latency: 0.65, load: 0.25, geo: 0.1 },
          use_tag_weights: true,
          filters: {
            node_ids: [],
            countries: [],
            tags: [],
            network_groups: [],
          },
        },
      },
      {
        id: 'action-default',
        type: 'action',
        label: 'Global Selection',
        position: { x: 540, y: 200 },
        data: {
          mode: isLB ? 'load_balance' : 'smart',
          weights: isLB
            ? { latency: 0, load: 1, geo: 0 }
            : { latency: 0.55, load: 0.3, geo: 0.15 },
          use_tag_weights: true,
          filters: {
            node_ids: [],
            countries: [],
            tags: [],
            network_groups: [],
          },
        },
      },
      {
        id: 'output',
        type: 'output',
        label: 'DNS Answer',
        position: { x: 820, y: 120 },
        data: {},
      },
    ],
    edges: [
      { id: 'e-in', source: 'input', target: 'cond-1' },
      {
        id: 'e-cn',
        source: 'cond-1',
        source_handle: 'case-cn',
        target: 'action-cn',
      },
      {
        id: 'e-def',
        source: 'cond-1',
        source_handle: 'case-default',
        target: 'action-default',
      },
      { id: 'e-out-cn', source: 'action-cn', target: 'output' },
      { id: 'e-out-def', source: 'action-default', target: 'output' },
    ],
  };
}

export function flowId(prefix: string) {
  const suffix = Array.from(
    crypto.getRandomValues(new Uint32Array(2)),
    (value) => value.toString(36),
  ).join('-');
  return `${prefix}-${suffix}`;
}

export function flowNodeType(
  node: SmartDNSFlowNode,
): 'input' | 'condition' | 'action' | 'output' {
  switch (node.type.trim().toLowerCase()) {
    case 'input':
    case 'inputnode':
      return 'input';
    case 'condition':
    case 'switch':
    case 'switchnode':
      return 'condition';
    case 'output':
    case 'outputnode':
      return 'output';
    default:
      return 'action';
  }
}

export function canConnectFlow(
  graph: SmartDNSFlowGraph,
  source: string,
  target: string,
  handle?: string | null,
) {
  const from = graph.nodes.find((node) => node.id === source);
  const to = graph.nodes.find((node) => node.id === target);
  if (
    !from ||
    !to ||
    source === target ||
    flowNodeType(from) === 'output' ||
    flowNodeType(to) === 'input'
  )
    return false;
  if (
    flowNodeType(from) === 'condition' &&
    !from.data.cases?.some((branch) => branch.id === handle)
  )
    return false;
  if (
    graph.edges.some(
      (edge) =>
        edge.source === source &&
        edge.target === target &&
        (edge.source_handle || null) === (handle || null),
    )
  )
    return false;

  const seen = new Set<string>();
  const queue = [target];
  while (queue.length) {
    const id = queue.pop()!;
    if (id === source) return false;
    if (seen.has(id)) continue;
    seen.add(id);
    for (const edge of graph.edges) {
      if (edge.source === id) queue.push(edge.target);
    }
  }
  return true;
}

export function availableNodePosition(
  graph: SmartDNSFlowGraph,
  center: { x: number; y: number },
) {
  const position = {
    x: Math.round(center.x - 104),
    y: Math.round(center.y - 36),
  };
  while (true) {
    const overlaps = graph.nodes.filter((node) => {
      const height = 66 + (node.data.cases?.length ?? 0) * 30;
      return (
        Math.abs(position.x - node.position.x) < 230 &&
        position.y < node.position.y + height + 24 &&
        position.y + 130 > node.position.y
      );
    });
    if (!overlaps.length) return position;
    position.y = Math.max(
      ...overlaps.map(
        (node) => node.position.y + 90 + (node.data.cases?.length ?? 0) * 30,
      ),
    );
  }
}
