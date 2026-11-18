import { expect, it } from 'vitest';
import { buildCountryHeat } from './country-heat';

it('colors the bundled US region for source names and country codes', () => {
  const heat = buildCountryHeat([
    { key: 'Japan', value: 24_526 },
    { key: 'United States', value: 45_000 },
    { key: 'US', value: 100 },
    { key: 'United States of America', value: 23 },
    { key: 'Germany', value: 138 },
    { key: 'HK', value: 1_091 },
  ]);

  const [first] = heat.values();
  expect(first).toMatchObject({
    name: 'United States of America',
    value: 45_123,
    intensity: 1,
    hasBoundary: true,
  });
  expect(heat.get('Japan')!.intensity).toBeGreaterThan(
    heat.get('Germany')!.intensity,
  );
  expect(heat.get('Hong Kong')).toMatchObject({
    hasBoundary: false,
    coordinate: [114.1747, 22.2783],
  });
});
