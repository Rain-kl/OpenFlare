export interface DashboardSummary {
  total_nodes: number;
  online_nodes: number;
  offline_nodes: number;
  pending_nodes: number;
  unhealthy_nodes: number;
}

export interface DashboardTraffic {
  request_count: number;
  unique_visitors: number;
  error_count: number;
  estimated_qps: number;
  reported_nodes: number;
}

export interface DashboardCapacity {
  average_cpu_usage_percent: number;
  average_memory_usage_percent: number;
  high_cpu_nodes: number;
  high_memory_nodes: number;
  high_storage_nodes: number;
}

export interface DistributionItem {
  key: string;
  value: number;
}

export type CompactDistributionItem = [string, number];

export interface TrafficTrendPoint {
  bucket_started_at: string;
  request_count: number;
  error_count: number;
  unique_visitor_count: number;
}

export interface CapacityTrendPoint {
  bucket_started_at: string;
  average_cpu_usage_percent: number;
  average_memory_usage_percent: number;
  reported_nodes: number;
}

export interface NetworkTrendPoint {
  bucket_started_at: string;
  network_rx_bytes: number;
  network_tx_bytes: number;
  openresty_rx_bytes: number;
  openresty_tx_bytes: number;
  reported_nodes: number;
}

export interface DiskIOTrendPoint {
  bucket_started_at: string;
  disk_read_bytes: number;
  disk_write_bytes: number;
  reported_nodes: number;
}

export interface TrafficDistributions {
  status_codes: DistributionItem[];
  top_domains: DistributionItem[];
  source_countries: DistributionItem[];
}

export interface DashboardTrends {
  traffic_24h: TrafficTrendPoint[];
  capacity_24h: CapacityTrendPoint[];
  network_24h: NetworkTrendPoint[];
  disk_io_24h: DiskIOTrendPoint[];
}

export interface DashboardNodeHealth {
  id: number;
  node_id: string;
  name: string;
  geo_name: string;
  geo_latitude?: number | null;
  geo_longitude?: number | null;
  status: 'online' | 'offline' | 'pending';
  openresty_status: 'healthy' | 'unhealthy' | 'unknown';
  current_version: string;
  last_seen_at: string;
  active_event_count: number;
  cpu_usage_percent: number;
  memory_usage_percent: number;
  storage_usage_percent: number;
  request_count: number;
  error_count: number;
  unique_visitor_count: number;
}

export interface DashboardOverview {
  generated_at: string;
  summary: DashboardSummary;
  traffic: DashboardTraffic;
  capacity: DashboardCapacity;
  distributions: TrafficDistributions;
  trends: DashboardTrends;
  nodes: DashboardNodeHealth[];
}

export type CompactTrafficTrendPoint = [string, number, number, number];
export type CompactCapacityTrendPoint = [string, number, number, number];
export type CompactNetworkTrendPoint = [
  string,
  number,
  number,
  number,
  number,
  number,
];
export type CompactDiskIOTrendPoint = [string, number, number, number];
export type CompactDashboardNodeHealth = [
  number,
  string,
  string,
  string,
  number | null,
  number | null,
  DashboardNodeHealth['status'],
  DashboardNodeHealth['openresty_status'],
  string,
  string,
  number,
  number,
  number,
  number,
  number,
  number,
  number,
];

export interface DashboardOverviewCompact {
  generated_at: string;
  summary: DashboardSummary;
  traffic: DashboardTraffic;
  capacity: DashboardCapacity;
  distributions: {
    status_codes: CompactDistributionItem[];
    top_domains: CompactDistributionItem[];
    source_countries: CompactDistributionItem[];
  };
  trends: {
    traffic_24h: CompactTrafficTrendPoint[];
    capacity_24h: CompactCapacityTrendPoint[];
    network_24h: CompactNetworkTrendPoint[];
    disk_io_24h: CompactDiskIOTrendPoint[];
  };
  nodes: CompactDashboardNodeHealth[];
}
