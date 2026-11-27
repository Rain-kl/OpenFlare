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

it('merges country aliases, filters unrecorded TCP RTTs, and sorts by request count', () => {
  const sample = [
    {
      region: 'US',
      request_count: 100,
      avg_request_time_ms: 120,
      avg_tcp_rtt_ms: 0,
    },
    {
      region: 'United States',
      request_count: 300,
      avg_request_time_ms: 80,
      avg_tcp_rtt_ms: 40,
    },
    {
      region: 'JP',
      request_count: 500,
      avg_request_time_ms: 50,
      avg_tcp_rtt_ms: 8,
    },
    {
      region: 'HK',
      request_count: 50,
      avg_request_time_ms: 250,
      avg_tcp_rtt_ms: 0,
    },
    {
      region: 'PT',
      request_count: 80,
      avg_request_time_ms: 0,
      avg_tcp_rtt_ms: 0,
    },
  ];

  const tcpRttMap = buildCountryLatency(sample);
  expect([...tcpRttMap.keys()]).toEqual(['Japan', 'United States of America']);
  expect(tcpRttMap.get('Japan')).toMatchObject({
    name: 'Japan',
    requestCount: 500,
    avgTcpRttMs: 8,
    hasBoundary: true,
  });
  expect(tcpRttMap.get('United States of America')).toMatchObject({
    name: 'United States of America',
    requestCount: 300,
    avgTcpRttMs: 40,
    hasBoundary: true,
  });
  expect(tcpRttMap.has('Hong Kong')).toBe(false);
  expect(tcpRttMap.has('Portugal')).toBe(false);
});
