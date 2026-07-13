import type { EdgeChange, Node, NodeChange } from '@xyflow/react';
import type { WAFRuleGraph, WAFRuleNode } from '@/lib/services/openflare';

import { wouldCreateCycle } from './graph-validation';

export type GraphErrorTarget = { kind: 'node' | 'edge'; id: string };

export function isPersistentNodeChange<NodeType extends Node = Node>(
  change: NodeChange<NodeType>,
): boolean {
  return (
    change.type === 'remove' ||
    change.type === 'add' ||
    change.type === 'replace' ||
    (change.type === 'position' &&
      change.dragging === false &&
      Boolean(change.position))
  );
}

export function isPersistentEdgeChange(change: EdgeChange): boolean {
  return (
    change.type === 'remove' ||
    change.type === 'add' ||
    change.type === 'replace'
  );
}

export function filterRemovableNodeIds(
  nodes: WAFRuleNode[],
  ids: string[],
): string[] {
  return ids.filter(
    (id) =>
      !['start', 'allow'].includes(
        nodes.find((node) => node.id === id)?.type ?? '',
      ),
  );
}

export function acceptedNodeChanges<NodeType extends Node = Node>(
  nodes: WAFRuleNode[],
  changes: NodeChange<NodeType>[],
): { changes: NodeChange<NodeType>[]; persistent: boolean } {
  const accepted = changes.filter(
    (change) =>
      change.type !== 'remove' ||
      filterRemovableNodeIds(nodes, [change.id]).length === 1,
  );
  return {
    changes: accepted,
    persistent: accepted.some(isPersistentNodeChange),
  };
}

export function getHistoryTransition(
  current: number,
  target: number,
): { direction: 'back' | 'forward'; restoreDelta: number } {
  return {
    direction: target < current ? 'back' : 'forward',
    restoreDelta: current - target,
  };
}

export function isConnectionAllowed(
  graph: WAFRuleGraph,
  connection: {
    source?: string | null;
    sourceHandle?: string | null;
    target?: string | null;
  },
): boolean {
  if (!connection.source || !connection.target || !connection.sourceHandle)
    return false;
  const source = graph.nodes.find((node) => node.id === connection.source);
  const handles: Partial<Record<WAFRuleNode['type'], string[]>> = {
    start: ['next'],
    ip_match: ['true', 'false'],
    geo_match: ['true', 'false'],
    pow: ['next'],
  };
  return (
    Boolean(
      source && (handles[source.type] ?? []).includes(connection.sourceHandle),
    ) &&
    !graph.edges.some(
      (edge) =>
        edge.source === connection.source &&
        edge.source_handle === connection.sourceHandle,
    ) &&
    !wouldCreateCycle(graph, connection.source, connection.target)
  );
}

export function findGraphErrorTarget(
  payload: unknown,
  nodeIds: string[],
  edgeIds: string[],
): GraphErrorTarget | undefined {
  const found = collectIdFields(payload);
  for (const { key, value } of found) {
    if ((key === 'node_id' || key === 'nodeId') && nodeIds.includes(value))
      return { kind: 'node', id: value };
    if ((key === 'edge_id' || key === 'edgeId') && edgeIds.includes(value))
      return { kind: 'edge', id: value };
  }
  const messages = collectMessages(payload);
  for (const message of messages) {
    const nodeId = findMessageId(message, '节点', nodeIds);
    if (nodeId) return { kind: 'node', id: nodeId };
    const edgeId = findMessageId(message, '边', edgeIds);
    if (edgeId) return { kind: 'edge', id: edgeId };
  }
  return undefined;
}

function collectMessages(value: unknown): string[] {
  if (value instanceof Error)
    return [value.message, ...collectMessages(value.cause)];
  if (!value || typeof value !== 'object') return [];
  return Object.entries(value).flatMap(([key, child]) =>
    key === 'error_msg' || key === 'message'
      ? typeof child === 'string'
        ? [child]
        : []
      : collectMessages(child),
  );
}

function findMessageId(
  message: string,
  prefix: '节点' | '边',
  ids: string[],
): string | undefined {
  const token = prefix === '边' ? '边(?:\\s+ID)?' : '节点';
  return [...ids]
    .sort((a, b) => b.length - a.length)
    .find((id) =>
      new RegExp(`${token}\\s+${escapeRegExp(id)}(?=$|[\\s，。,:：的])`).test(
        message,
      ),
    );
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function collectIdFields(value: unknown): { key: string; value: string }[] {
  if (!value || typeof value !== 'object') return [];
  const result: { key: string; value: string }[] = [];
  for (const [key, child] of Object.entries(value)) {
    if (typeof child === 'string') result.push({ key, value: child });
    else result.push(...collectIdFields(child));
  }
  return result;
}
