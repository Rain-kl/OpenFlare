import { describe, expect, it } from 'vitest';

import { serializeSearchParams } from '@/lib/services/core/search-params';

describe('serializeSearchParams', () => {
  it('serializes hosts arrays as repeated keys for Gin QueryArray', () => {
    const query = serializeSearchParams({
      hours: 168,
      hosts: ['gist.arctel.de'],
    });
    expect(query).toBe('hours=168&hosts=gist.arctel.de');
  });

  it('serializes multiple hosts without bracket notation', () => {
    const query = serializeSearchParams({
      hosts: ['a.example', 'b.example'],
    });
    expect(query).toBe('hosts=a.example&hosts=b.example');
    expect(query).not.toContain('hosts[]');
    expect(query).not.toContain('hosts%5B%5D');
  });
});
