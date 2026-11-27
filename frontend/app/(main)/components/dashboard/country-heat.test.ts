import { expect, it } from 'vitest';
import { buildCountryHeat, buildCountryLatency } from './country-heat';

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

it('merges country aliases and resolves centroids for country latency', () => {
  const latency = buildCountryLatency([
    {
      region: 'US',
      request_count: 100,
      avg_request_time_ms: 120,
      avg_tcp_rtt_ms: 40,
    },
    {
      region: 'United States',
      request_count: 300,
      avg_request_time_ms: 80,
      avg_tcp_rtt_ms: 0,
    },
    {
      region: 'HK',
      request_count: 50,
      avg_request_time_ms: 25,
      avg_tcp_rtt_ms: 12,
    },
  ]);

  const us = latency.get('United States of America');
  expect(us).toMatchObject({
    name: 'United States of America',
    requestCount: 400,
    avgRequestTimeMs: 90,
    avgTcpRttMs: 40,
    hasBoundary: true,
  });
  expect(latency.get('Hong Kong')).toMatchObject({
    name: 'Hong Kong',
    requestCount: 50,
    avgRequestTimeMs: 25,
    avgTcpRttMs: 12,
    hasBoundary: false,
    coordinate: [114.1747, 22.2783],
  });
});
