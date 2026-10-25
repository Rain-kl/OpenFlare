import { runtimeText } from '@/i18n/runtime';
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
  security_check: ['true', 'false'],
  pow: ['next'],
};

export function validateGraph(graph: WAFRuleGraph): GraphIssue[] {
  const issues: GraphIssue[] = [];
  const nodeMap = new Map(graph.nodes.map((node) => [node.id, node]));
  if (graph.schema_version !== 1)
    issues.push({
      code: 'schema',
      message: runtimeText('rule_graph_schema_version_must_be_1_ea9a1161'),
    });
  if (
    graph.nodes.length > 128 ||
    graph.edges.length > 256 ||
    new TextEncoder().encode(JSON.stringify(graph)).length > 256 * 1024
  )
    issues.push({
      code: 'size_limit',
      message: runtimeText('rule_graph_exceeds_the_size_limit_337651b9'),
    });
  const nodeIds = new Set<string>();
  for (const node of graph.nodes) {
    if (!node.id.trim())
      issues.push({
        code: 'empty_id',
        message: runtimeText('node_id_must_not_be_empty_51a5de58'),
        nodeId: node.id,
      });
    if (nodeIds.has(node.id))
      issues.push({
        code: 'duplicate_node_id',
        message: runtimeText('duplicate_node_id_value0_621230af', {
          value0: node.id,
        }),
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
        message: runtimeText('edge_id_must_not_be_empty_bc8f69cb'),
        edgeId: edge.id,
      });
    if (edgeIds.has(edge.id))
      issues.push({
        code: 'duplicate_edge_id',
        message: runtimeText('duplicate_edge_id_value0_0a9d1c7c', {
          value0: edge.id,
        }),
        edgeId: edge.id,
      });
    edgeIds.add(edge.id);
    if (nodeMap.get(edge.target)?.type === 'start')
      issues.push({
        code: 'start_incoming',
        message: runtimeText('the_start_node_must_not_have_incoming_50a07b17'),
        edgeId: edge.id,
        nodeId: edge.target,
      });
  }
  if (graph.nodes.filter((node) => node.type === 'start').length !== 1)
    issues.push({
      code: 'start_count',
      message: runtimeText('the_rule_graph_must_contain_exactly_one_7dfbc027'),
    });
  if (graph.nodes.filter((node) => node.type === 'allow').length !== 1)
    issues.push({
      code: 'allow_count',
      message: runtimeText('the_rule_graph_must_contain_exactly_one_0036eb82'),
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
        message: runtimeText(
          'edge_value0_has_an_invalid_endpoint_or_9d84eb7e',
          { value0: edge.id },
        ),
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
          message: runtimeText(
            'node_value0_output_value1_is_not_connected_c3038e81',
            { value0: node.id, value1: handle },
          ),
          nodeId: node.id,
        });
      if (outgoing.length > 1)
        issues.push({
          code: 'duplicate_handle',
          message: runtimeText(
            'node_value0_output_value1_can_only_be_903e54ce',
            { value0: node.id, value1: handle },
          ),
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
        message: runtimeText(
          'node_value0_is_unreachable_from_the_start_086531dc',
          { value0: node.id },
        ),
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
        message: runtimeText(
          'node_value0_cannot_reach_a_terminal_node_8de2ff6e',
          { value0: node.id },
        ),
        nodeId: node.id,
      });
  if (hasCycle(graph))
    issues.push({
      code: 'cycle',
      message: runtimeText('the_rule_graph_must_not_contain_cycles_fd6fe797'),
    });
  return issues;
}

function validateNodeConfig(node: WAFRuleNode): string | undefined {
  if (node.type === 'ip_match') {
    if (node.config.ips.some((value) => !isIP(value)))
      return runtimeText(
        'node_value0_contains_an_invalid_ip_address_1873ab94',
        { value0: node.id },
      );
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
      return runtimeText('node_value0_contains_an_invalid_cidr_00c880d9', {
        value0: node.id,
      });
    if (node.config.ip_group_ids.some((id) => !Number.isInteger(id) || id <= 0))
      return runtimeText('node_value0_contains_an_invalid_ip_group_589219a5', {
        value0: node.id,
      });
  }
  if (
    node.type === 'geo_match' &&
    (node.config.countries.some((code) => !/^[A-Z]{2}$/.test(code)) ||
      node.config.regions.some(
        (code) => !/^[A-Z]{2}-[A-Z0-9]{1,3}$/.test(code),
      ))
  )
    return runtimeText(
      'node_value0_contains_an_invalid_geography_code_1a9f469b',
      { value0: node.id },
    );
  if (
    node.type === 'pow' &&
    (!['fast', 'slow'].includes(node.config.algorithm) ||
      !isIntegerInRange(node.config.difficulty, 1, 16) ||
      !isIntegerInRange(node.config.session_ttl, 60) ||
      !isIntegerInRange(node.config.challenge_ttl, 30))
  )
    return runtimeText('node_value0_has_pow_settings_outside_the_9fd2a4cb', {
      value0: node.id,
    });
  if (
    node.type === 'block' &&
    (!isIntegerInRange(node.config.status_code, 400, 599) ||
      new TextEncoder().encode(node.config.response_body).length > 16 * 1024)
  )
    return runtimeText(
      'node_value0_has_invalid_block_response_settings_ae4c35ca',
      { value0: node.id },
    );
  if (node.type === 'ua_check') {
    if (!['and', 'or'].includes(node.config.match_mode))
      return runtimeText('node_value0_match_mode_must_be_and_242c1ae7', {
        value0: node.id,
      });
    if (node.config.browsers.some((label) => !UA_BROWSER_LABELS.has(label)))
      return runtimeText(
        'node_value0_contains_an_invalid_browser_tag_4d0e6740',
        { value0: node.id },
      );
    if (node.config.operating_systems.some((label) => !UA_OS_LABELS.has(label)))
      return runtimeText(
        'node_value0_contains_an_invalid_operating_system_45bea1fb',
        { value0: node.id },
      );
    if (node.config.custom_ua_patterns.length > 32)
      return runtimeText('node_value0_allows_at_most_32_custom_497938c6', {
        value0: node.id,
      });
    for (const pattern of node.config.custom_ua_patterns) {
      if (!pattern.trim())
        return runtimeText('node_value0_custom_ua_pattern_must_not_a72718be', {
          value0: node.id,
        });
      if (new TextEncoder().encode(pattern).length > 256)
        return runtimeText('node_value0_custom_ua_pattern_is_too_c2c83c0e', {
          value0: node.id,
        });
      try {
        void new RegExp(pattern);
      } catch {
        return runtimeText('node_value0_has_an_invalid_custom_ua_c47ffd0d', {
          value0: node.id,
        });
      }
    }
    if (
      node.config.block_custom_ua &&
      node.config.custom_ua_patterns.length === 0
    )
      return runtimeText('node_value0_requires_at_least_one_pattern_6114bb93', {
        value0: node.id,
      });
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
