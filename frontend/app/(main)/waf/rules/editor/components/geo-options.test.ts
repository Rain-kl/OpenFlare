import { describe, expect, it } from 'vitest';

import { countryOptions, regionOptions } from './geo-options';

describe('geo options', () => {
  it('contains the complete country list with localized names', () => {
    expect(countryOptions).toHaveLength(249);
    expect(countryOptions).toContainEqual(
      expect.objectContaining({ value: 'CN', label: '中国' }),
    );
  });

  it('contains searchable ISO subdivision options', () => {
    expect(regionOptions.length).toBeGreaterThan(4000);
    expect(regionOptions).toContainEqual(
      expect.objectContaining({
        value: 'CN-BJ',
        label: expect.stringContaining('Beijing'),
        searchText: expect.stringContaining('中国'),
      }),
    );
  });
});
