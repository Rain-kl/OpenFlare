import { apiRequest } from '@/lib/api/client';

import type {
  CompactCapacityTrendPoint,
  CompactDashboardNodeHealth,
  CompactDiskIOTrendPoint,
  CompactDistributionItem,
  CompactNetworkTrendPoint,
  CompactTrafficTrendPoint,
  DashboardCapacity,
  DashboardNodeHealth,
  DashboardOverview,
  DashboardOverviewCompact,
  DashboardSummary,
  DashboardTraffic,
  DistributionItem,
} from '@/features/dashboard/types';

function arrayOrEmpty<T>(value: T[] | null | undefined) {
  return Array.isArray(value) ? value : [];
}

function isCompactDistributionItem(
  value: DistributionItem | CompactDistributionItem,
): value is CompactDistributionItem {
  return Array.isArray(value);
}

function isCompactTrafficTrendPoint(
  value: DashboardOverview['trends']['traffic_24h'][number] | CompactTrafficTrendPoint,
): value is CompactTrafficTrendPoint {
  return Array.isArray(value);
}

function isCompactCapacityTrendPoint(
  value:
    | DashboardOverview['trends']['capacity_24h'][number]
    | CompactCapacityTrendPoint,
): value is CompactCapacityTrendPoint {
  return Array.isArray(value);
}

function isCompactNetworkTrendPoint(
  value:
    | DashboardOverview['trends']['network_24h'][number]
    | CompactNetworkTrendPoint,
): value is CompactNetworkTrendPoint {
  return Array.isArray(value);
}

function isCompactDiskIOTrendPoint(
  value:
    | DashboardOverview['trends']['disk_io_24h'][number]
    | CompactDiskIOTrendPoint,
): value is CompactDiskIOTrendPoint {
  return Array.isArray(value);
}

function isCompactDashboardNode(
  value: DashboardNodeHealth | CompactDashboardNodeHealth,
): value is CompactDashboardNodeHealth {
  return Array.isArray(value);
}

function normalizeDistributionItems(
  items: Array<DistributionItem | CompactDistributionItem> | null | undefined,
): DistributionItem[] {
  return arrayOrEmpty(items).map((item) =>
    isCompactDistributionItem(item)
      ? { key: String(item[0] ?? ''), value: Number(item[1] ?? 0) }
      : item,
  );
}

function normalizeTrafficTrendPoints(
  items:
    | Array<DashboardOverview['trends']['traffic_24h'][number] | CompactTrafficTrendPoint>
    | null
    | undefined,
) {
  return arrayOrEmpty(items).map((item) =>
    isCompactTrafficTrendPoint(item)
      ? {
          bucket_started_at: String(item[0] ?? ''),
          request_count: Number(item[1] ?? 0),
          error_count: Number(item[2] ?? 0),
          unique_visitor_count: Number(item[3] ?? 0),
        }
      : item,
  );
}

function normalizeCapacityTrendPoints(
  items:
    | Array<
        | DashboardOverview['trends']['capacity_24h'][number]
        | CompactCapacityTrendPoint
      >
    | null
    | undefined,
) {
  return arrayOrEmpty(items).map((item) =>
    isCompactCapacityTrendPoint(item)
      ? {
          bucket_started_at: String(item[0] ?? ''),
          average_cpu_usage_percent: Number(item[1] ?? 0),
          average_memory_usage_percent: Number(item[2] ?? 0),
          reported_nodes: Number(item[3] ?? 0),
        }
      : item,
  );
}

function normalizeNetworkTrendPoints(
  items:
    | Array<
        DashboardOverview['trends']['network_24h'][number] | CompactNetworkTrendPoint
      >
    | null
    | undefined,
) {
  return arrayOrEmpty(items).map((item) =>
    isCompactNetworkTrendPoint(item)
      ? {
          bucket_started_at: String(item[0] ?? ''),
          network_rx_bytes: Number(item[1] ?? 0),
          network_tx_bytes: Number(item[2] ?? 0),
          openresty_rx_bytes: Number(item[3] ?? 0),
          openresty_tx_bytes: Number(item[4] ?? 0),
          reported_nodes: Number(item[5] ?? 0),
        }
      : item,
  );
}

function normalizeDiskIOTrendPoints(
  items:
    | Array<
        DashboardOverview['trends']['disk_io_24h'][number] | CompactDiskIOTrendPoint
      >
    | null
    | undefined,
) {
  return arrayOrEmpty(items).map((item) =>
    isCompactDiskIOTrendPoint(item)
      ? {
          bucket_started_at: String(item[0] ?? ''),
          disk_read_bytes: Number(item[1] ?? 0),
          disk_write_bytes: Number(item[2] ?? 0),
          reported_nodes: Number(item[3] ?? 0),
        }
      : item,
  );
}

function normalizeDashboardNodes(
  items: Array<DashboardNodeHealth | CompactDashboardNodeHealth> | null | undefined,
): DashboardNodeHealth[] {
  return arrayOrEmpty(items).map((item) =>
    isCompactDashboardNode(item)
      ? {
          id: Number(item[0] ?? 0),
          node_id: String(item[1] ?? ''),
          name: String(item[2] ?? ''),
          geo_name: String(item[3] ?? ''),
          geo_latitude:
            item[4] === null || item[4] === undefined ? null : Number(item[4]),
          geo_longitude:
            item[5] === null || item[5] === undefined ? null : Number(item[5]),
          status: (item[6] ?? 'pending') as DashboardNodeHealth['status'],
          openresty_status: (item[7] ??
            'unknown') as DashboardNodeHealth['openresty_status'],
          current_version: String(item[8] ?? ''),
          last_seen_at: String(item[9] ?? ''),
          active_event_count: Number(item[10] ?? 0),
          cpu_usage_percent: Number(item[11] ?? 0),
          memory_usage_percent: Number(item[12] ?? 0),
          storage_usage_percent: Number(item[13] ?? 0),
          request_count: Number(item[14] ?? 0),
          error_count: Number(item[15] ?? 0),
          unique_visitor_count: Number(item[16] ?? 0),
        }
      : item,
  );
}

function normalizeDashboardOverview(
  overview:
    | DashboardOverview
    | DashboardOverviewCompact
    | null
    | undefined,
): DashboardOverview | null {
  if (!overview) {
    return null;
  }

  const summary = (overview.summary ?? {}) as DashboardSummary;
  const traffic = (overview.traffic ?? {}) as DashboardTraffic;
  const capacity = (overview.capacity ?? {}) as DashboardCapacity;

  return {
    generated_at: String(overview.generated_at ?? ''),
    summary,
    traffic,
    capacity,
    nodes: normalizeDashboardNodes(overview.nodes),
    distributions: {
      source_countries: normalizeDistributionItems(
        overview.distributions?.source_countries,
      ),
      status_codes: normalizeDistributionItems(overview.distributions?.status_codes),
      top_domains: normalizeDistributionItems(overview.distributions?.top_domains),
    },
    trends: {
      traffic_24h: normalizeTrafficTrendPoints(overview.trends?.traffic_24h),
      capacity_24h: normalizeCapacityTrendPoints(overview.trends?.capacity_24h),
      network_24h: normalizeNetworkTrendPoints(overview.trends?.network_24h),
      disk_io_24h: normalizeDiskIOTrendPoints(overview.trends?.disk_io_24h),
    },
  };
}

export async function getDashboardOverview() {
  const overview = await apiRequest<DashboardOverview | DashboardOverviewCompact>(
    '/dashboard/overview',
  );
  return normalizeDashboardOverview(overview);
}
