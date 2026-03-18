export interface AccessLogFilters {
  node_id?: string;
  remote_addr?: string;
  host?: string;
  path?: string;
  p?: number;
  page_size?: number;
  sort_by?: string;
  sort_order?: 'asc' | 'desc';
}

export interface AccessLogItem {
  id: number;
  node_id: string;
  node_name: string;
  logged_at: string;
  remote_addr: string;
  region: string;
  host: string;
  path: string;
  status_code: number;
}

export interface AccessLogList {
  items: AccessLogItem[];
  page: number;
  page_size: number;
  has_more: boolean;
  total_record: number;
  total_ip: number;
}

export interface FoldedAccessLogFilters extends AccessLogFilters {
  fold_minutes: 3 | 5;
}

export interface FoldedAccessLogItem {
  bucket_started_at: string;
  request_count: number;
  unique_ip_count: number;
  unique_host_count: number;
  success_count: number;
  client_error_count: number;
  server_error_count: number;
}

export interface FoldedAccessLogList {
  items: FoldedAccessLogItem[];
  page: number;
  page_size: number;
  has_more: boolean;
  total_bucket: number;
  total_record: number;
  total_ip: number;
  fold_minutes: number;
}

export interface AccessLogIPSummaryFilters {
  node_id?: string;
  remote_addr?: string;
  host?: string;
  p?: number;
  page_size?: number;
  sort_by?: string;
  sort_order?: 'asc' | 'desc';
}

export interface AccessLogIPSummaryItem {
  remote_addr: string;
  total_requests: number;
  recent_requests: number;
  last_seen_at: string;
}

export interface AccessLogIPSummaryList {
  items: AccessLogIPSummaryItem[];
  page: number;
  page_size: number;
  has_more: boolean;
  total_ip: number;
  sort_by: string;
  sort_order: 'asc' | 'desc';
}

export interface AccessLogIPTrendFilters {
  node_id?: string;
  remote_addr: string;
  host?: string;
  hours?: number;
  bucket_minutes?: number;
}

export interface AccessLogIPTrendPoint {
  bucket_started_at: string;
  request_count: number;
}

export interface AccessLogIPTrend {
  remote_addr: string;
  hours: number;
  bucket_minutes: number;
  points: AccessLogIPTrendPoint[];
}

export interface AccessLogCleanupPayload {
  retention_days: number;
}

export interface AccessLogCleanupResult {
  retention_days: number;
  deleted_count: number;
  cutoff: string;
}
