import { apiRequest } from '@/lib/api/client';

import type { DashboardOverview } from '@/features/dashboard/types';

function arrayOrEmpty<T>(value: T[] | null | undefined) {
  return Array.isArray(value) ? value : [];
}

function normalizeDashboardOverview(
  overview: DashboardOverview | null | undefined,
): DashboardOverview | null {
  if (!overview) {
    return null;
  }

  return {
    ...overview,
    nodes: arrayOrEmpty(overview.nodes),
    active_alerts: arrayOrEmpty(overview.active_alerts),
    distributions: {
      ...overview.distributions,
      source_countries: arrayOrEmpty(overview.distributions?.source_countries),
      status_codes: arrayOrEmpty(overview.distributions?.status_codes),
      top_domains: arrayOrEmpty(overview.distributions?.top_domains),
    },
    trends: {
      ...overview.trends,
      traffic_24h: arrayOrEmpty(overview.trends?.traffic_24h),
      capacity_24h: arrayOrEmpty(overview.trends?.capacity_24h),
      network_24h: arrayOrEmpty(overview.trends?.network_24h),
      disk_io_24h: arrayOrEmpty(overview.trends?.disk_io_24h),
    },
  };
}

export async function getDashboardOverview() {
  const overview = await apiRequest<DashboardOverview>('/dashboard/overview');
  return normalizeDashboardOverview(overview);
}
