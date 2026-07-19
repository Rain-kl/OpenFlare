import { describe, expect, it } from 'vitest';

import type { WAFRuleGraph } from '@/lib/services/openflare';

import { layoutRuleGraph } from './graph-layout';

const sampleGraph: WAFRuleGraph = {
  schema_version: 1,
  nodes: [
    {
      id: 'start',
      type: 'start',
      position: { x: 500, y: 400 },
      config: {},
    },
    {
      id: 'match',
      type: 'ip_match',
      position: { x: 10, y: 10 },
      config: { ips: [], cidrs: [], ip_group_ids: [] },
    },
    {
      id: 'allow',
      type: 'allow',
      position: { x: 0, y: 0 },
      config: {},
    },
    {
      id: 'block',
      type: 'block',
      position: { x: 99, y: 99 },
      config: { status_code: 403, response_body: '' },
    },
  ],
  edges: [
    {
      id: 'e1',
      source: 'start',
      source_handle: 'next',
      target: 'match',
    },
    {
      id: 'e2',
      source: 'match',
      source_handle: 'true',
      target: 'allow',
    },
    {
      id: 'e3',
      source: 'match',
      source_handle: 'false',
      target: 'block',
    },
  ],
};

describe('layoutRuleGraph', () => {
  it('places start left of match and match left of terminals', () => {
    const laid = layoutRuleGraph(sampleGraph);
    const byId = Object.fromEntries(laid.nodes.map((n) => [n.id, n]));
    expect(byId.start.position.x).toBeLessThan(byId.match.position.x);
    expect(byId.match.position.x).toBeLessThan(byId.allow.position.x);
    expect(byId.match.position.x).toBeLessThan(byId.block.position.x);
  });

  it('keeps edges unchanged', () => {
    const laid = layoutRuleGraph(sampleGraph);
    expect(laid.edges).toEqual(sampleGraph.edges);
  });

  it('separates sibling terminals on y axis', () => {
    const laid = layoutRuleGraph(sampleGraph);
    const byId = Object.fromEntries(laid.nodes.map((n) => [n.id, n]));
    expect(byId.allow.position.y).not.toBe(byId.block.position.y);
  });
});
