import { describe, expect, it } from 'vitest';

import {
  createRuleNode,
  displayNodeTitle,
  NODE_TYPE_LABELS,
  parseAddableNodeType,
} from './node-factory';

describe('displayNodeTitle', () => {
  it('uses trimmed label when present', () => {
    expect(
      displayNodeTitle({
        type: 'ip_match',
        label: '  办公室  ',
      }),
    ).toBe('办公室');
  });

  it('falls back to type default when label empty', () => {
    expect(
      displayNodeTitle({
        type: 'block',
        label: '  ',
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

describe('parseAddableNodeType', () => {
  it('accepts addable types and rejects others', () => {
    expect(parseAddableNodeType('ip_match')).toBe('ip_match');
    expect(parseAddableNodeType('ua_check')).toBe('ua_check');
    expect(parseAddableNodeType('start')).toBeNull();
    expect(parseAddableNodeType('')).toBeNull();
  });
});

describe('createRuleNode ua_check', () => {
  it('creates default UA check config', () => {
    const node = createRuleNode('ua_check', { x: 1, y: 2 });
    expect(node.type).toBe('ua_check');
    if (node.type === 'ua_check') {
      expect(node.config).toEqual({
        require_ua: false,
        browsers: [],
        operating_systems: [],
        match_mode: 'or',
        block_common_bots: false,
        block_abnormal_ua: false,
        block_custom_ua: false,
        custom_ua_patterns: [],
      });
    }
  });
});
