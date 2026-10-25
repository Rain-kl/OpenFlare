import { describe, expect, it } from 'vitest';

import { createGeoOptions } from './geo-options';

const { countryOptions, regionOptions } = createGeoOptions('zh-CN');

describe('geo options', () => {
  it('uses English country names when requested without changing country codes', () => {
    const english = createGeoOptions('en');
    expect(english.countryOptions).toContainEqual(
      expect.objectContaining({ value: 'CN', label: 'China' }),
    );
    expect(english.regionOptions).toContainEqual(
      expect.objectContaining({
        value: 'CN-BJ',
        searchText: expect.stringContaining('China'),
      }),
    );
  });
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
