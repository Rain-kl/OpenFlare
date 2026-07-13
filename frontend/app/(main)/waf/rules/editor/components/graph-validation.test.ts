import {describe, expect, it} from 'vitest';

import type {WAFRuleGraph} from '@/lib/services/openflare';

import {removeEdgeFromGraph, removeNodeFromGraph, validateGraph, wouldCreateCycle} from './graph-validation';

const validGraph = (): WAFRuleGraph => ({
  schema_version: 1,
  nodes: [
    {id: 'start', type: 'start', position: {x: 0, y: 0}, config: {}},
    {id: 'match', type: 'ip_match', position: {x: 240, y: 0}, config: {ips: ['127.0.0.1'], cidrs: [], ip_group_ids: []}},
    {id: 'allow', type: 'allow', position: {x: 520, y: -80}, config: {}},
    {id: 'block', type: 'block', position: {x: 520, y: 100}, config: {status_code: 403, response_body: ''}},
  ],
  edges: [
    {id: 'start-match', source: 'start', source_handle: 'next', target: 'match'},
    {id: 'match-allow', source: 'match', source_handle: 'true', target: 'allow'},
    {id: 'match-block', source: 'match', source_handle: 'false', target: 'block'},
  ],
});

describe('validateGraph', () => {
  it('accepts a complete terminating graph', () => expect(validateGraph(validGraph())).toEqual([]));

  it('requires exactly one start and allow node', () => {
    const graph = validGraph();
    graph.nodes = graph.nodes.filter((node) => node.type !== 'allow');
    expect(validateGraph(graph).map((issue) => issue.code)).toContain('allow_count');
  });

  it('requires every source handle', () => {
    const graph = validGraph();
    graph.edges = graph.edges.filter((edge) => edge.source_handle !== 'false');
    expect(validateGraph(graph)).toContainEqual(expect.objectContaining({code: 'missing_handle', nodeId: 'match'}));
  });

  it('rejects cycles', () => {
    const graph = validGraph();
    graph.edges.push({id: 'cycle', source: 'block', source_handle: 'next', target: 'match'});
    expect(validateGraph(graph).map((issue) => issue.code)).toContain('cycle');
  });

  it('reports unreachable nodes and paths without a terminal', () => {
    const graph = validGraph();
    graph.nodes.push({id: 'orphan', type: 'pow', position: {x: 0, y: 200}, config: {algorithm: 'fast', difficulty: 4, session_ttl: 60, challenge_ttl: 30}});
    expect(validateGraph(graph)).toEqual(expect.arrayContaining([
      expect.objectContaining({code: 'unreachable', nodeId: 'orphan'}),
      expect.objectContaining({code: 'non_terminating', nodeId: 'orphan'}),
    ]));
  });

  it('rejects duplicate identifiers and start incoming edges', () => {
    const graph = validGraph();
    graph.nodes.push({...graph.nodes[1]});
    graph.edges.push({...graph.edges[0]}, {id: 'into-start', source: 'match', source_handle: 'true', target: 'start'});
    expect(validateGraph(graph)).toEqual(expect.arrayContaining([
      expect.objectContaining({code: 'duplicate_node_id', nodeId: 'match'}),
      expect.objectContaining({code: 'duplicate_edge_id', edgeId: 'start-match'}),
      expect.objectContaining({code: 'start_incoming', edgeId: 'into-start'}),
    ]));
  });

  it('validates typed node configuration locally', () => {
    const graph = validGraph();
    graph.nodes = graph.nodes.map((node) => node.type === 'ip_match' ? {...node, config: {ips: ['999.1.1.1'], cidrs: ['broken'], ip_group_ids: [-1]}} : node.type === 'block' ? {...node, config: {status_code: 200, response_body: 'x'.repeat(65_537)}} : node);
    expect(validateGraph(graph).filter((issue) => issue.code === 'invalid_config').map((issue) => issue.nodeId)).toEqual(expect.arrayContaining(['match', 'block']));
  });

  it('validates PoW bounds and geography codes', () => {
    const graph = validGraph();
    graph.nodes.push({id: 'pow', type: 'pow', position: {x: 0, y: 0}, config: {algorithm: 'fast', difficulty: 0, session_ttl: 0, challenge_ttl: 0}});
    graph.nodes.push({id: 'geo', type: 'geo_match', position: {x: 0, y: 0}, config: {countries: ['china'], regions: ['']}});
    expect(validateGraph(graph)).toEqual(expect.arrayContaining([
      expect.objectContaining({code: 'invalid_config', nodeId: 'pow'}),
      expect.objectContaining({code: 'invalid_config', nodeId: 'geo'}),
    ]));
  });

  it('rejects non-finite and fractional integer configuration', () => {
    const graph = validGraph();
    graph.nodes.push({id: 'pow', type: 'pow', position: {x: 0, y: 0}, config: {algorithm: 'fast', difficulty: 4.5, session_ttl: Number.NaN, challenge_ttl: 30}});
    graph.nodes.push({id: 'block-fraction', type: 'block', position: {x: 0, y: 0}, config: {status_code: 403.5, response_body: ''}});
    expect(validateGraph(graph)).toEqual(expect.arrayContaining([
      expect.objectContaining({code: 'invalid_config', nodeId: 'pow'}),
      expect.objectContaining({code: 'invalid_config', nodeId: 'block-fraction'}),
    ]));
  });

  it('matches server IP and prefix parsing semantics', () => {
    const invalid = validGraph();
    invalid.nodes = invalid.nodes.map((node) => node.type === 'ip_match' ? {...node, config: {ips: ['2001:db8::1', '::::'], cidrs: ['2001:db8::/32', '10.0.0.0/33'], ip_group_ids: []}} : node);
    expect(validateGraph(invalid)).toContainEqual(expect.objectContaining({code: 'invalid_config', nodeId: 'match'}));
    const valid = validGraph();
    valid.nodes = valid.nodes.map((node) => node.type === 'ip_match' ? {...node, config: {ips: ['2001:db8::1', '192.0.2.1'], cidrs: ['2001:db8::/32', '10.0.0.0/8'], ip_group_ids: []}} : node);
    expect(validateGraph(valid)).toEqual([]);
  });

  it('requires exactly one CIDR slash and rejects scoped IPv6 addresses', () => {
    for (const value of ['10.0.0.0/8/extra', 'fe80::1%en0', 'fe80::%en0/64']) {
      const graph = validGraph();
      graph.nodes = graph.nodes.map((node) => node.type === 'ip_match' ? {...node, config: value.includes('/') ? {ips: [], cidrs: [value], ip_group_ids: []} : {ips: [value], cidrs: [], ip_group_ids: []}} : node);
      expect(validateGraph(graph)).toContainEqual(expect.objectContaining({code: 'invalid_config', nodeId: 'match'}));
    }
  });
});

it('removes incident edges when deleting a node', () => {
  const next = removeNodeFromGraph(validGraph(), 'match');
  expect(next.nodes.some((node) => node.id === 'match')).toBe(false);
  expect(next.edges).toEqual([]);
});

it('removes a selected connection without changing nodes', () => {
  const graph = validGraph();
  const edgeId = graph.edges[0].id;
  const next = removeEdgeFromGraph(graph, edgeId);
  expect(next.nodes).toEqual(graph.nodes);
  expect(next.edges).toHaveLength(graph.edges.length - 1);
  expect(next.edges.some((edge) => edge.id === edgeId)).toBe(false);
});

it('detects whether a new connection creates a cycle', () => {
  expect(wouldCreateCycle(validGraph(), 'allow', 'start')).toBe(true);
  expect(wouldCreateCycle(validGraph(), 'start', 'block')).toBe(false);
});
