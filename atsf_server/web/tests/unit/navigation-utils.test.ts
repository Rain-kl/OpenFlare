import { describe, expect, it } from 'vitest';

import { getCurrentNavigationItem, isPathActive } from '@/lib/utils/navigation';

describe('navigation utils', () => {
  it('marks root path as active only for home', () => {
    expect(isPathActive('/', '/')).toBe(true);
    expect(isPathActive('/node', '/')).toBe(false);
  });

  it('resolves current navigation item for nested paths', () => {
    expect(getCurrentNavigationItem('/node/abc')?.label).toBe('节点');
    expect(getCurrentNavigationItem('/website')?.label).toBe('网站');
    expect(getCurrentNavigationItem('/performance')?.label).toBe('性能');
    expect(getCurrentNavigationItem('/setting')?.label).toBe('设置');
  });
});
