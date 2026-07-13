import {describe, expect, it} from 'vitest';

import type {WAFRuleGraph} from '@/lib/services/openflare';

import {acceptedNodeChanges, filterRemovableNodeIds, findGraphErrorTarget, getHistoryTransition, isConnectionAllowed, isPersistentEdgeChange, isPersistentNodeChange} from './editor-behavior';

describe('React Flow persistence filtering', () => {
  it('ignores dimensions and selection changes', () => {
    expect(isPersistentNodeChange({type: 'dimensions', id: 'n', dimensions: {width: 10, height: 10}})).toBe(false);
    expect(isPersistentNodeChange({type: 'select', id: 'n', selected: true})).toBe(false);
    expect(isPersistentEdgeChange({type: 'select', id: 'e', selected: true})).toBe(false);
  });

  it('persists completed position changes and removals', () => {
    expect(isPersistentNodeChange({type: 'position', id: 'n', position: {x: 1, y: 2}, dragging: false})).toBe(true);
    expect(isPersistentNodeChange({type: 'remove', id: 'n'})).toBe(true);
    expect(isPersistentEdgeChange({type: 'remove', id: 'e'})).toBe(true);
  });

  it('forwards transient node changes to React Flow without persisting them', () => {
    const changes = [
      {type: 'dimensions' as const, id: 'match', dimensions: {width: 176, height: 64}},
      {type: 'select' as const, id: 'match', selected: true},
      {type: 'position' as const, id: 'match', position: {x: 24, y: 48}, dragging: true},
    ];

    const nodes: WAFRuleGraph['nodes'] = [
      {id: 'match', type: 'ip_match', position: {x: 0, y: 0}, config: {ips: [], cidrs: [], ip_group_ids: []}},
    ];
    expect(acceptedNodeChanges(nodes, changes)).toEqual({changes, persistent: false});
  });
});

describe('editor safety constraints', () => {
  const graph: WAFRuleGraph = {schema_version: 1, nodes: [
    {id: 'start', type: 'start', position: {x: 0, y: 0}, config: {}},
    {id: 'match', type: 'ip_match', position: {x: 0, y: 0}, config: {ips: [], cidrs: [], ip_group_ids: []}},
    {id: 'allow', type: 'allow', position: {x: 0, y: 0}, config: {}},
  ], edges: [{id: 'start-match', source: 'start', source_handle: 'next', target: 'match'}]};

  it('protects start and allow from deletion', () => expect(filterRemovableNodeIds(graph.nodes, ['start', 'match', 'allow'])).toEqual(['match']));

  it('does not persist a removal containing only protected nodes', () => {
    expect(acceptedNodeChanges(graph.nodes, [{type: 'remove', id: 'start'}, {type: 'remove', id: 'allow'}])).toEqual({changes: [], persistent: false});
  });

  it('rejects invalid and already-used source ports', () => {
    expect(isConnectionAllowed(graph, {source: 'match', sourceHandle: 'next', target: 'allow'})).toBe(false);
    expect(isConnectionAllowed(graph, {source: 'start', sourceHandle: 'next', target: 'allow'})).toBe(false);
    expect(isConnectionAllowed(graph, {source: 'match', sourceHandle: 'true', target: 'allow'})).toBe(true);
  });
});

describe('server graph error targeting', () => {
  const nodes = ['start', 'match-1'];
  const edges = ['start-match'];

  it('uses explicit node and edge ids from nested payloads', () => {
    expect(findGraphErrorTarget({details: {node_id: 'match-1'}}, nodes, edges)).toEqual({kind: 'node', id: 'match-1'});
    expect(findGraphErrorTarget({details: {edgeId: 'start-match'}}, nodes, edges)).toEqual({kind: 'edge', id: 'start-match'});
  });

  it('does not substring match unrelated error text', () => {
    expect(findGraphErrorTarget({message: 'restart operation failed'}, nodes, edges)).toBeUndefined();
  });

  it('parses the real API envelope with strict ID boundaries', () => {
    expect(findGraphErrorTarget({error_msg: '规则图无效: 节点 match-1 的 true 出口未连接', data: null}, nodes, edges)).toEqual({kind: 'node', id: 'match-1'});
    expect(findGraphErrorTarget(new Error('规则图无效: 边 start-match 的目标节点不存在'), nodes, edges)).toEqual({kind: 'edge', id: 'start-match'});
    expect(findGraphErrorTarget({error_msg: '节点 match-10 无效'}, nodes, edges)).toBeUndefined();
    expect(findGraphErrorTarget({error_msg: '规则图无效: 边 ID start-match 重复'}, nodes, edges)).toEqual({kind: 'edge', id: 'start-match'});
    expect(findGraphErrorTarget({error_msg: '边 ID start-matcher 重复'}, nodes, edges)).toBeUndefined();
  });
});

it('calculates deterministic Back and Forward restoration deltas', () => {
  expect(getHistoryTransition(4, 3)).toEqual({direction: 'back', restoreDelta: 1});
  expect(getHistoryTransition(4, 6)).toEqual({direction: 'forward', restoreDelta: -2});
});
