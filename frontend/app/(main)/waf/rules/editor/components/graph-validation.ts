import type { WAFRuleGraph, WAFRuleNode } from '@/lib/services/openflare';

import { UA_BROWSER_LABELS, UA_OS_LABELS } from './ua-options';

export type GraphIssueCode =
  | 'schema'
  | 'size_limit'
  | 'empty_id'
  | 'duplicate_node_id'
  | 'duplicate_edge_id'
  | 'start_count'
  | 'allow_count'
  | 'start_incoming'
  | 'missing_handle'
  | 'duplicate_handle'
  | 'invalid_edge'
  | 'invalid_config'
  | 'cycle'
  | 'unreachable'
  | 'non_terminating';

export interface GraphIssue {
  code: GraphIssueCode;
  message: string;
  nodeId?: string;
  edgeId?: string;
}

const handles: Partial<Record<WAFRuleNode['type'], string[]>> = {
  start: ['next'],
  ip_match: ['true', 'false'],
  geo_match: ['true', 'false'],
  ua_check: ['true', 'false'],
  pow: ['next'],
};

export function validateGraph(graph: WAFRuleGraph): GraphIssue[] {
  const issues: GraphIssue[] = [];
  const nodeMap = new Map(graph.nodes.map((node) => [node.id, node]));
  if (graph.schema_version !== 1)
    issues.push({ code: 'schema', message: '规则图 schema_version 必须为 1' });
  if (
    graph.nodes.length > 128 ||
    graph.edges.length > 256 ||
    new TextEncoder().encode(JSON.stringify(graph)).length > 256 * 1024
  )
    issues.push({ code: 'size_limit', message: '规则图超过大小限制' });
  const nodeIds = new Set<string>();
  for (const node of graph.nodes) {
    if (!node.id.trim())
      issues.push({
        code: 'empty_id',
        message: '节点 ID 不能为空',
        nodeId: node.id,
      });
    if (nodeIds.has(node.id))
      issues.push({
        code: 'duplicate_node_id',
        message: `节点 ID ${node.id} 重复`,
        nodeId: node.id,
      });
    nodeIds.add(node.id);
    const configIssue = validateNodeConfig(node);
    if (configIssue)
      issues.push({
        code: 'invalid_config',
        message: configIssue,
        nodeId: node.id,
      });
  }
  const edgeIds = new Set<string>();
  for (const edge of graph.edges) {
    if (!edge.id.trim())
      issues.push({
        code: 'empty_id',
        message: '连线 ID 不能为空',
        edgeId: edge.id,
      });
    if (edgeIds.has(edge.id))
      issues.push({
        code: 'duplicate_edge_id',
        message: `连线 ID ${edge.id} 重复`,
        edgeId: edge.id,
      });
    edgeIds.add(edge.id);
    if (nodeMap.get(edge.target)?.type === 'start')
      issues.push({
        code: 'start_incoming',
        message: '开始节点不能有入边',
        edgeId: edge.id,
        nodeId: edge.target,
      });
  }
  if (graph.nodes.filter((node) => node.type === 'start').length !== 1)
    issues.push({
      code: 'start_count',
      message: '规则图必须恰好有一个开始节点',
    });
  if (graph.nodes.filter((node) => node.type === 'allow').length !== 1)
    issues.push({
      code: 'allow_count',
      message: '规则图必须恰好有一个通过节点',
    });

  for (const edge of graph.edges) {
    const source = nodeMap.get(edge.source);
    if (
      !source ||
      !nodeMap.has(edge.target) ||
      !(handles[source.type] ?? []).includes(edge.source_handle)
    ) {
      issues.push({
        code: 'invalid_edge',
        message: `连线 ${edge.id} 的端点或出口无效`,
        edgeId: edge.id,
      });
    }
  }
  for (const node of graph.nodes) {
    for (const handle of handles[node.type] ?? []) {
      const outgoing = graph.edges.filter(
        (edge) => edge.source === node.id && edge.source_handle === handle,
      );
      if (outgoing.length === 0)
        issues.push({
          code: 'missing_handle',
          message: `节点 ${node.id} 的 ${handle} 出口未连接`,
          nodeId: node.id,
        });
      if (outgoing.length > 1)
        issues.push({
          code: 'duplicate_handle',
          message: `节点 ${node.id} 的 ${handle} 出口只能连接一次`,
          nodeId: node.id,
        });
    }
  }

  const adjacency = new Map(
    graph.nodes.map((node) => [node.id, [] as string[]]),
  );
  const reverse = new Map(graph.nodes.map((node) => [node.id, [] as string[]]));
  for (const edge of graph.edges) {
    adjacency.get(edge.source)?.push(edge.target);
    reverse.get(edge.target)?.push(edge.source);
  }
  const start = graph.nodes.find((node) => node.type === 'start');
  const reachable = walk(start ? [start.id] : [], adjacency);
  for (const node of graph.nodes)
    if (!reachable.has(node.id))
      issues.push({
        code: 'unreachable',
        message: `节点 ${node.id} 无法从开始节点到达`,
        nodeId: node.id,
      });
  const terminals = graph.nodes
    .filter((node) => node.type === 'allow' || node.type === 'block')
    .map((node) => node.id);
  const canTerminate = walk(terminals, reverse);
  for (const node of graph.nodes)
    if (!canTerminate.has(node.id))
      issues.push({
        code: 'non_terminating',
        message: `节点 ${node.id} 无法抵达终止节点`,
        nodeId: node.id,
      });
  if (hasCycle(graph))
    issues.push({ code: 'cycle', message: '规则图不能包含循环' });
  return issues;
}

function validateNodeConfig(node: WAFRuleNode): string | undefined {
  if (node.type === 'ip_match') {
    if (node.config.ips.some((value) => !isIP(value)))
      return `节点 ${node.id} 包含无效 IP`;
    if (
      node.config.cidrs.some((value) => {
        const parts = value.split('/');
        if (parts.length !== 2) return true;
        const [ip, bits] = parts;
        return (
          !isIP(ip) ||
          !/^\d+$/.test(bits) ||
          Number(bits) > (ip.includes(':') ? 128 : 32)
        );
      })
    )
      return `节点 ${node.id} 包含无效 CIDR`;
    if (node.config.ip_group_ids.some((id) => !Number.isInteger(id) || id <= 0))
      return `节点 ${node.id} 包含无效 IP 组`;
  }
  if (
    node.type === 'geo_match' &&
    (node.config.countries.some((code) => !/^[A-Z]{2}$/.test(code)) ||
      node.config.regions.some(
        (code) => !/^[A-Z]{2}-[A-Z0-9]{1,3}$/.test(code),
      ))
  )
    return `节点 ${node.id} 包含无效地域代码`;
  if (
    node.type === 'pow' &&
    (!['fast', 'slow'].includes(node.config.algorithm) ||
      !isIntegerInRange(node.config.difficulty, 1, 16) ||
      !isIntegerInRange(node.config.session_ttl, 60) ||
      !isIntegerInRange(node.config.challenge_ttl, 30))
  )
    return `节点 ${node.id} 的 PoW 配置超出范围`;
  if (
    node.type === 'block' &&
    (!isIntegerInRange(node.config.status_code, 400, 599) ||
      new TextEncoder().encode(node.config.response_body).length > 16 * 1024)
  )
    return `节点 ${node.id} 的阻止响应配置无效`;
  if (node.type === 'ua_check') {
    if (!['and', 'or'].includes(node.config.match_mode))
      return `节点 ${node.id} 的匹配模式必须为 and 或 or`;
    if (node.config.browsers.some((label) => !UA_BROWSER_LABELS.has(label)))
      return `节点 ${node.id} 包含无效浏览器标签`;
    if (node.config.operating_systems.some((label) => !UA_OS_LABELS.has(label)))
      return `节点 ${node.id} 包含无效操作系统标签`;
  }
  return undefined;
}

function isIntegerInRange(
  value: number,
  min: number,
  max = Number.MAX_SAFE_INTEGER,
): boolean {
  return (
    Number.isFinite(value) &&
    Number.isInteger(value) &&
    value >= min &&
    value <= max
  );
}

function isIP(value: string): boolean {
  if (value.includes(':')) return isIPv6(value);
  const parts = value.split('.');
  return (
    parts.length === 4 &&
    parts.every(
      (part) => /^(0|[1-9]\d{0,2})$/.test(part) && Number(part) <= 255,
    )
  );
}

function isIPv6(value: string): boolean {
  if (
    !/^[0-9a-f:.]+$/i.test(value) ||
    value.includes(':::') ||
    value.split('::').length > 2
  )
    return false;
  const compressed = value.includes('::');
  const sections = value.split('::');
  const groups = sections.flatMap((section) =>
    section ? section.split(':') : [],
  );
  let units = 0;
  for (let index = 0; index < groups.length; index++) {
    const group = groups[index];
    if (group.includes('.')) {
      if (index !== groups.length - 1 || !isIP(group)) return false;
      units += 2;
    } else {
      if (!/^[0-9a-f]{1,4}$/i.test(group)) return false;
      units++;
    }
  }
  return compressed ? units < 8 : units === 8;
}

function walk(seeds: string[], links: Map<string, string[]>): Set<string> {
  const seen = new Set<string>();
  const stack = [...seeds];
  while (stack.length) {
    const id = stack.pop()!;
    if (seen.has(id)) continue;
    seen.add(id);
    stack.push(...(links.get(id) ?? []));
  }
  return seen;
}

function hasCycle(graph: WAFRuleGraph): boolean {
  const indegree = new Map(graph.nodes.map((node) => [node.id, 0]));
  for (const edge of graph.edges)
    if (indegree.has(edge.target) && indegree.has(edge.source))
      indegree.set(edge.target, (indegree.get(edge.target) ?? 0) + 1);
  const queue = [...indegree]
    .filter(([, degree]) => degree === 0)
    .map(([id]) => id);
  let visited = 0;
  while (queue.length) {
    const id = queue.shift()!;
    visited++;
    for (const edge of graph.edges.filter((item) => item.source === id)) {
      const next = (indegree.get(edge.target) ?? 0) - 1;
      indegree.set(edge.target, next);
      if (next === 0) queue.push(edge.target);
    }
  }
  return visited !== graph.nodes.length;
}

export function wouldCreateCycle(
  graph: WAFRuleGraph,
  source: string,
  target: string,
): boolean {
  if (source === target) return true;
  const adjacency = new Map(
    graph.nodes.map((node) => [node.id, [] as string[]]),
  );
  for (const edge of graph.edges) adjacency.get(edge.source)?.push(edge.target);
  return walk([target], adjacency).has(source);
}

export function removeNodeFromGraph(
  graph: WAFRuleGraph,
  nodeId: string,
): WAFRuleGraph {
  return {
    ...graph,
    nodes: graph.nodes.filter((node) => node.id !== nodeId),
    edges: graph.edges.filter(
      (edge) => edge.source !== nodeId && edge.target !== nodeId,
    ),
  };
}

export function removeEdgeFromGraph(
  graph: WAFRuleGraph,
  edgeId: string,
): WAFRuleGraph {
  return { ...graph, edges: graph.edges.filter((edge) => edge.id !== edgeId) };
}
