import type {
  DashboardCountryLatency,
  DistributionItem,
} from '@/lib/services/openflare';
import centroids from './data/country-centroids.json';
import world from './data/world-geo.json';

export type Coordinate = [number, number];

const countryCentroids: Record<string, number[]> = centroids;
const boundaries = new Set(
  world.features.map((feature) => feature.properties.name),
);
const names = new Map(
  [...boundaries, ...Object.keys(centroids)].map((name) => [
    name.toLowerCase(),
    name,
  ]),
);
const regionNames = new Intl.DisplayNames('en', {
  type: 'region',
  fallback: 'none',
});

// Common source names that differ from the bundled map's names.
const aliases = new Map([
  ['bahamas', 'The Bahamas'],
  ['bosnia & herzegovina', 'Bosnia and Herzegovina'],
  ['congo - brazzaville', 'Republic of the Congo'],
  ['congo - kinshasa', 'Democratic Republic of the Congo'],
  ['côte d’ivoire', 'Ivory Coast'],
  ["côte d'ivoire", 'Ivory Coast'],
  ['czechia', 'Czech Republic'],
  ['eswatini', 'Swaziland'],
  ['guinea-bissau', 'Guinea Bissau'],
  ['hong kong sar china', 'Hong Kong'],
  ['macao sar china', 'Macao'],
  ['macau', 'Macao'],
  ['myanmar (burma)', 'Myanmar'],
  ['north macedonia', 'Macedonia'],
  ['serbia', 'Republic of Serbia'],
  ['tanzania', 'United Republic of Tanzania'],
  ['timor-leste', 'East Timor'],
  ['trinidad & tobago', 'Trinidad and Tobago'],
  ['türkiye', 'Turkey'],
  ['united states', 'United States of America'],
]);

function resolveCountryName(key: string) {
  const trimmed = key.trim();
  const name = /^[a-z]{2}$/i.test(trimmed)
    ? regionNames.of(trimmed.toUpperCase())
    : trimmed;
  if (!name) return undefined;
  const normalized = name.toLowerCase();
  return aliases.get(normalized) ?? names.get(normalized);
}

export type CountryHeat = {
  name: string;
  value: number;
  intensity: number;
  hasBoundary: boolean;
  coordinate?: Coordinate;
};

export function buildCountryHeat(
  countries: DistributionItem[],
): Map<string, CountryHeat> {
  const totals = new Map<string, number>();
  for (const { key, value } of countries) {
    if (value <= 0) continue;
    const name = resolveCountryName(key);
    if (name) totals.set(name, (totals.get(name) ?? 0) + value);
  }

  const ranked = [...totals].sort((a, b) => b[1] - a[1]);
  const maximum = ranked[0]?.[1] ?? 1;
  return new Map(
    ranked.map(([name, value]) => {
      const centroid = countryCentroids[name];
      return [
        name,
        {
          name,
          value,
          // Keep smaller sources visible while preserving their order.
          intensity: Math.sqrt(value / maximum),
          hasBoundary: boundaries.has(name),
          coordinate: centroid ? [centroid[0], centroid[1]] : undefined,
        },
      ];
    }),
  );
}

export type CountryLatencyDatum = {
  name: string;
  requestCount: number;
  avgTcpRttMs: number;
  hasBoundary: boolean;
  coordinate?: Coordinate;
};

export function buildCountryLatency(
  items: DashboardCountryLatency[],
): Map<string, CountryLatencyDatum> {
  const buckets = new Map<
    string,
    {
      requestCount: number;
      tcpWeight: number;
      weightedTcpRttMs: number;
    }
  >();

  for (const item of items) {
    if (!item.region || item.request_count <= 0) continue;
    const name = resolveCountryName(item.region);
    if (!name) continue;
    const avgTcpMs =
      Number.isFinite(item.avg_tcp_rtt_ms) && item.avg_tcp_rtt_ms > 0
        ? item.avg_tcp_rtt_ms
        : 0;
    if (avgTcpMs <= 0) continue;

    const current = buckets.get(name) ?? {
      requestCount: 0,
      tcpWeight: 0,
      weightedTcpRttMs: 0,
    };
    current.requestCount += item.request_count;
    current.tcpWeight += item.request_count;
    current.weightedTcpRttMs += avgTcpMs * item.request_count;
    buckets.set(name, current);
  }

  const entries: Array<[string, CountryLatencyDatum]> = [];
  for (const [name, bucket] of buckets) {
    const avgTcpRttMs =
      bucket.tcpWeight > 0 ? bucket.weightedTcpRttMs / bucket.tcpWeight : 0;
    if (avgTcpRttMs <= 0) continue;
    entries.push([
      name,
      {
        name,
        requestCount: bucket.requestCount,
        avgTcpRttMs,
        hasBoundary: boundaries.has(name),
        coordinate: countryCentroids[name]
          ? [countryCentroids[name][0], countryCentroids[name][1]]
          : undefined,
      },
    ]);
  }
  entries.sort((a, b) => {
    if (b[1].requestCount !== a[1].requestCount) {
      return b[1].requestCount - a[1].requestCount;
    }
    return b[1].avgTcpRttMs - a[1].avgTcpRttMs;
  });
  return new Map(entries);
}
