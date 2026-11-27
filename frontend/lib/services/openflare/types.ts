import type { XYPosition } from '@xyflow/react';

export type ReleaseChannel = 'stable' | 'preview';

export type NodeType = 'edge_node';

export type NodeStatus = 'online' | 'offline' | 'pending' | 'quarantined';

export type OpenrestyStatus = 'healthy' | 'unhealthy' | 'unknown';

export type ApplyResult = 'success' | 'warning' | 'failed' | '';

export interface NodeItem {
  id: number;
  node_id: string;
  node_type: NodeType;
  name: string;
  ip: string;
  ip_manual_override: boolean;
  asn?: number;
  asn_org?: string;
  geo_name: string;
  geo_latitude?: number | null;
  geo_longitude?: number | null;
  geo_manual_override: boolean;
  access_token: string;
  auto_update_enabled: boolean;
  edge_update_supported?: boolean;
  update_requested: boolean;
  update_channel: ReleaseChannel;
  update_tag: string;
  restart_openresty_requested: boolean;
  version: string;
  ext_version: string;
  openresty_status: OpenrestyStatus;
  openresty_message: string;
  status: NodeStatus;
  quarantined_at?: string | null;
  quarantine_reason?: string;
  current_version: string;
  last_seen_at: string;
  last_error: string;
  latest_apply_result: ApplyResult;
  latest_apply_message: string;
  latest_apply_checksum: string;
  latest_main_config_checksum: string;
  latest_route_config_checksum: string;
  latest_support_file_count: number;
  latest_apply_at?: string | null;
  created_at: string;
  updated_at: string;
  monitoring?: NodeMonitoring;
}

export interface NodeMonitoring {
  system?: string;
  metric?: Pick<
    NodeMetricSnapshot,
    | 'captured_at'
    | 'cpu_usage_percent'
    | 'memory_used_bytes'
    | 'memory_total_bytes'
    | 'storage_used_bytes'
    | 'storage_total_bytes'
    | 'net_in'
    | 'net_out'
  >;
  ping?: NodeMonitoringPing[];
}

export interface NodeMonitoringPing {
  task_id: number;
  name: string;
  samples: Pick<PingRecord, 'recorded_at' | 'value'>[];
}

export interface NodeCertificateDelivery {
  certificate_id: number;
  certificate_name: string;
  primary_domain: string;
  provider: string;
  apply_status: string;
  not_after?: string | null;
  cert_sha256: string;
  current_cert_sha256: string;
  matches_current: boolean;
  release_sequence: number;
  delivery_count: number;
  first_delivered_at: string;
  last_delivered_at: string;
}

export interface NodeQuarantineInput {
  reason: string;
  rotate_acme_certs: boolean;
}

export interface NodeQuarantineResult {
  node: NodeItem;
  exposed_certificates: NodeCertificateDelivery[];
  rotated_certificate_ids: number[];
  rotation_errors: Record<number, string>;
  manual_action_certificate_ids: number[];
  dns_switched_groups: string[];
  dns_unbacked_groups: string[];
}

export interface NodeListQuery {
  page?: number;
  page_size?: number;
  search?: string;
  status?: NodeStatus | '';
  node_type?: NodeType | '';
  include_monitoring?: boolean;
}

export interface NodeListResult {
  total: number;
  results: NodeItem[];
  page: number;
  page_size: number;
  monitoring_unavailable?: boolean;
}

export interface NodeBootstrapToken {
  discovery_token: string;
}

export interface NodeMutationPayload {
  node_type: NodeType;
  name: string;
  ip: string;
  ip_manual_override: boolean;
  auto_update_enabled: boolean;
  geo_name: string;
  geo_latitude?: number | null;
  geo_longitude?: number | null;
  geo_manual_override: boolean;
}

export interface NodeAgentReleaseInfo {
  tag_name: string;
  body: string;
  html_url: string;
  published_at: string;
  current_version: string;
  has_update: boolean;
  channel: ReleaseChannel;
  prerelease: boolean;
  update_requested: boolean;
  requested_channel: ReleaseChannel;
  requested_tag: string;
}

export interface NodeAgentUpdatePayload {
  channel?: ReleaseChannel;
  tag_name?: string;
}

export interface NodeSystemProfile {
  hostname: string;
  os_name: string;
  os_version: string;
  kernel_version: string;
  architecture: string;
  cpu_model: string;
  cpu_cores: number;
  cpu_physical_cores?: number;
  virtualization?: string;
  gpu_name?: string;
  total_memory_bytes: number;
  swap_total_bytes?: number;
  total_disk_bytes: number;
  ipv4?: string;
  ipv6?: string;
  probe_version?: string;
  uptime_seconds: number;
  reported_at: string;
}

export interface NodeMetricSnapshot {
  captured_at: string;
  cpu_usage_percent: number;
  gpu_usage_percent?: number;
  memory_used_bytes: number;
  memory_total_bytes: number;
  swap_used_bytes?: number;
  swap_total_bytes?: number;
  load1?: number;
  load5?: number;
  load15?: number;
  temp?: number;
  storage_used_bytes: number;
  storage_total_bytes: number;
  disk_read_bytes: number;
  disk_write_bytes: number;
  net_in?: number;
  net_out?: number;
  net_total_up?: number;
  net_total_down?: number;
  process_count?: number;
  tcp_connections?: number;
  udp_connections?: number;
  uptime_seconds?: number;
  openresty_connections: number;
}

export type PingTaskType = 'icmp' | 'tcp' | 'http';

export interface PingTask {
  id: number;
  weight: number;
  name: string;
  clients: string[];
  default_on: boolean;
  type: PingTaskType;
  target: string;
  interval: number;
  created_at: string;
  updated_at: string;
}

export interface PingTaskPayload {
  name: string;
  clients: string[];
  default_on: boolean;
  type: PingTaskType;
  target: string;
  interval: number;
}

export interface PingTaskBatchUpdateItem extends PingTaskPayload {
  id: number;
}

export interface PingTaskStats {
  id: number;
  name: string;
  type: PingTaskType;
  target: string;
  interval: number;
  min: number;
  max: number;
  avg: number;
  latest: number;
  p50: number;
  p99: number;
  p99_p50_ratio: number;
  loss: number;
  total: number;
}

export interface PingRecord {
  task_id: number;
  recorded_at: string;
  value: number;
}

export interface NodePingRecords {
  node_id: string;
  hours: number;
  tasks: PingTaskStats[];
  records: PingRecord[];
}

export interface ExecTaskResult {
  id?: number;
  task_id: string;
  client: string;
  node_name: string;
  result: string;
  exit_code: number | null;
  finished_at: string | null;
  created_at: string;
}

export interface ExecTask {
  task_id: string;
  command: string;
  clients: string[];
  created_by: number;
  created_at: string;
  results: ExecTaskResult[];
}

export interface ExecRunPayload {
  command: string;
  clients: string[];
  timeout_seconds?: number;
}

export interface ExecRunResponse {
  task_id: string;
  command: string;
  clients: string[];
  online_clients: string[];
  offline_clients: string[];
  created_at: string;
  results: ExecTaskResult[];
}

export interface NodeHealthEvent {
  event_type: string;
  severity: string;
  status: string;
  message: string;
  metadata_json?: string;
  first_triggered_at: string;
  last_triggered_at: string;
  reported_at: string;
  resolved_at?: string | null;
}

export interface NodeTrafficSummary {
  window_started_at: string;
  window_ended_at: string;
  request_count: number;
  unique_visitor_count: number;
  error_count: number;
  estimated_qps: number;
  error_rate_percent: number;
}

export interface NodeHealthSummary {
  active_alerts: number;
  critical_alerts: number;
  warning_alerts: number;
  info_alerts: number;
  resolved_alerts: number;
  has_capacity_risk: boolean;
  has_traffic_risk: boolean;
  has_runtime_risk: boolean;
}

export interface NodeTrafficDistributions {
  status_codes: DistributionItem[];
  top_domains: DistributionItem[];
  source_countries: DistributionItem[];
}

export interface NodeObservabilityAnalytics {
  traffic: NodeTrafficSummary | null;
  distributions: NodeTrafficDistributions;
  health: NodeHealthSummary | null;
}

export interface NodeObservabilityTrends {
  traffic_24h: TrafficTrendPoint[];
  capacity_24h: CapacityTrendPoint[];
  network_24h: NetworkTrendPoint[];
  disk_io_24h: DiskIOTrendPoint[];
}

export interface NodeObservability {
  node_id: string;
  profile: NodeSystemProfile | null;
  metric_snapshots: NodeMetricSnapshot[];
  health_events: NodeHealthEvent[];
  analytics?: NodeObservabilityAnalytics;
  trends?: NodeObservabilityTrends;
}

export type ProxyRouteConfigSection =
  | 'domains'
  | 'limits'
  | 'proxy'
  | 'cache'
  | 'waf'
  | 'pow'
  | 'auth';

export interface ProxyRouteCustomHeader {
  key: string;
  value: string;
}

export interface ProxyRoutePoWListConfig {
  ips: string[];
  ip_cidrs: string[];
  paths: string[];
  path_regexes: string[];
  user_agents: string[];
}

export interface ProxyRoutePoWConfig {
  difficulty: number;
  algorithm: 'fast' | 'slow';
  session_ttl: number;
  challenge_ttl: number;
  whitelist: ProxyRoutePoWListConfig;
  blacklist: ProxyRoutePoWListConfig;
}

/** Route-bound Zone domain as returned by proxy-route APIs. */
export interface ProxyRouteZoneDomain {
  id: number;
  zone_id: number;
  domain: string;
  cert_id: number | null;
}

export interface ProxyRouteItem {
  id: number;
  site_name: string;
  zone_domain_ids: number[];
  zone_domains: ProxyRouteZoneDomain[];
  origin_id: number | null;
  origin_url: string;
  origin_host: string;
  upstreams: string;
  upstream_list: string[];
  enabled: boolean;
  enable_https: boolean;
  enable_http2: boolean;
  redirect_http: boolean;
  limit_conn_per_server: number;
  limit_conn_per_ip: number;
  limit_rate: string;
  limit_req_per_ip: string;
  cache_enabled: boolean;
  cache_policy: string;
  cache_rules: string;
  cache_rule_list: string[];
  custom_headers: string;
  custom_header_list: ProxyRouteCustomHeader[];
  basic_auth_enabled: boolean;
  site_auth_ttl_hours?: number;
  basic_auth_username: string;
  basic_auth_password: string;
  upstream_type: 'direct' | 'tunnel' | 'pages';
  tunnel_id?: number | null;
  tunnel_target_addr?: string;
  tunnel_target_protocol?: string;
  tunnel_options_json?: string;
  pages_project_id?: number | null;
  created_at: string;
  updated_at: string;
}

export interface ProxyRouteMutationPayload {
  site_name?: string;
  zone_domain_ids: number[];
  origin_id: number | null;
  origin_url: string;
  origin_scheme: 'http' | 'https';
  origin_address: string;
  origin_port: string;
  origin_uri: string;
  origin_host: string;
  upstreams: string[];
  enabled: boolean;
  enable_https: boolean;
  enable_http2?: boolean;
  redirect_http: boolean;
  limit_conn_per_server?: number;
  limit_conn_per_ip?: number;
  limit_rate?: string;
  limit_req_per_ip?: string;
  cache_enabled: boolean;
  cache_policy: string;
  cache_rules: string[];
  custom_headers: ProxyRouteCustomHeader[];
  basic_auth_enabled: boolean;
  site_auth_ttl_hours?: number;
  basic_auth_username?: string;
  basic_auth_password?: string;
  upstream_type?: 'direct' | 'tunnel' | 'pages';
  tunnel_id?: number | null;
  tunnel_target_addr?: string;
  tunnel_target_protocol?: string;
  tunnel_options_json?: string;
  pages_project_id?: number | null;
}

export interface ApplyLogItem {
  id: number;
  node_id: string;
  version: string;
  result: 'success' | 'failed' | string;
  message: string;
  checksum: string;
  main_config_checksum: string;
  route_config_checksum: string;
  support_file_count: number;
  created_at: string;
}

export interface ApplyLogList {
  rows: ApplyLogItem[];
  current: number;
  total: number;
  totalPage: number;
}

export interface ApplyLogListQuery {
  node_id?: string;
  pageNo?: number;
  pageSize?: number;
}

export interface ApplyLogCleanupPayload {
  delete_all?: boolean;
  retention_days?: number;
}

export interface ApplyLogCleanupResult {
  delete_all: boolean;
  retention_days: number;
  deleted_count: number;
  cutoff?: string;
}

// ==================== Pages ====================

export interface PagesDeployment {
  id: number;
  project_id: number;
  deployment_number: number;
  checksum: string;
  status: 'uploaded' | 'active';
  file_count: number;
  total_size: number;
  created_by: string;
  source_type: 'manual_upload' | 'manual_url' | 'remote_url' | 'github_release';
  source_label: string;
  trigger_type:
    | 'manual_upload'
    | 'manual_url'
    | 'manual_sync'
    | 'scheduled_auto_update';
  created_at: string;
  activated_at?: string | null;
}

export interface PagesDeploymentFile {
  id: number;
  deployment_id: number;
  path: string;
  size: number;
  checksum: string;
  created_at: string;
}

export interface PagesProject {
  id: number;
  name: string;
  slug: string;
  description: string;
  enabled: boolean;
  spa_fallback_enabled: boolean;
  spa_fallback_path: string;
  api_proxy_enabled: boolean;
  api_proxy_path: string;
  api_proxy_pass: string;
  api_proxy_rewrite: string;
  root_dir?: string;
  entry_file: string;
  active_deployment_id?: number | null;
  active_deployment?: PagesDeployment | null;
  deployment_count: number;
  created_at: string;
  updated_at: string;
}

export interface PagesProjectPayload {
  name: string;
  slug: string;
  description: string;
  enabled: boolean;
  spa_fallback_enabled: boolean;
  spa_fallback_path: string;
  api_proxy_enabled: boolean;
  api_proxy_path: string;
  api_proxy_pass: string;
  api_proxy_rewrite: string;
  root_dir?: string;
  entry_file: string;
}

export interface PagesDeploymentUploadPayload {
  file: File;
  onProgress?: (percent: number) => void;
}

export interface PagesDeploymentUploadFromURLPayload {
  url: string;
}

export type PagesSourceStatus =
  | 'idle'
  | 'checking'
  | 'update_available'
  | 'syncing'
  | 'failed'
  | 'attention';

export type PagesGitHubReleaseSelector = 'latest' | 'tag';

export interface PagesSourceRevision {
  revision: string;
  label: string;
  asset_name?: string;
}

interface PagesSourceRuntimeView {
  sync_status?: PagesSourceStatus;
  update_available?: boolean;
  last_seen?: PagesSourceRevision;
  last_applied?: PagesSourceRevision;
  last_checked_at?: string | null;
  last_synced_at?: string | null;
  next_check_at?: string | null;
  last_error?: string;
}

export interface PagesManualSource {
  source_type: 'manual';
}

export interface PagesRemoteURLSource extends PagesSourceRuntimeView {
  source_type: 'remote_url';
  remote_url: string;
  allow_insecure?: boolean;
}

interface PagesGitHubReleaseSourceBase extends PagesSourceRuntimeView {
  source_type: 'github_release';
  github_repository: string;
  asset_name: string;
}

interface PagesGitHubLatestReleaseSource extends PagesGitHubReleaseSourceBase {
  release_selector: 'latest';
  release_tag?: '';
  auto_update_enabled: boolean;
  check_interval_minutes: number;
}

interface PagesGitHubTagReleaseSource extends PagesGitHubReleaseSourceBase {
  release_selector: 'tag';
  release_tag: string;
  auto_update_enabled: false;
  check_interval_minutes?: 0;
}

export type PagesGitHubReleaseSource =
  | PagesGitHubLatestReleaseSource
  | PagesGitHubTagReleaseSource;

/**
 * 部署源使用判别联合，后续仓库构建来源只需增加独立 git_repository variant，
 * 不需要向 Remote 或 GitHub Release 填入构建字段。
 */
export type PagesSource =
  | PagesManualSource
  | PagesRemoteURLSource
  | PagesGitHubReleaseSource;

export interface PagesRemoteSourceUpdatePayload {
  source_type: 'remote_url';
  remote_url: string;
  allow_insecure?: boolean;
}

interface PagesGitHubSourceUpdateBase {
  source_type: 'github_release';
  repository_url: string;
  asset_name: string;
}

export interface PagesGitHubLatestSourceUpdatePayload
  extends PagesGitHubSourceUpdateBase {
  release_selector: 'latest';
  release_tag: '';
  auto_update_enabled: boolean;
  check_interval_minutes: number;
}

export interface PagesGitHubTagSourceUpdatePayload
  extends PagesGitHubSourceUpdateBase {
  release_selector: 'tag';
  release_tag: string;
  auto_update_enabled: false;
  check_interval_minutes: 0;
}

export type PagesGitHubSourceUpdatePayload =
  | PagesGitHubLatestSourceUpdatePayload
  | PagesGitHubTagSourceUpdatePayload;

/**
 * Source 更新请求保持 Provider 判别联合；未来仓库拉取构建使用独立 git_repository variant，
 * 不向 Remote URL 或 GitHub Release payload 混入构建字段。
 */
export type PagesSourceUpdatePayload =
  | PagesRemoteSourceUpdatePayload
  | PagesGitHubSourceUpdatePayload;

export interface PagesSourceActionPayload {
  confirmed_revision?: string;
}

export interface PagesSourceActionReceipt {
  task_id: string;
  execution_id: string;
  action: 'check' | 'sync';
}

export interface PagesSourceUpdateResult {
  source: PagesSource;
  check_task: PagesSourceActionReceipt | null;
  warning: string;
}

// ==================== Origins ====================

export interface OriginItem {
  id: number;
  name: string;
  address: string;
  remark: string;
  route_count: number;
  created_at: string;
  updated_at: string;
}

export interface OriginRouteSummary {
  id: number;
  domain: string;
  origin_url: string;
  enabled: boolean;
  updated_at: string;
}

export interface OriginDetail extends OriginItem {
  routes: OriginRouteSummary[];
}

export interface OriginMutationPayload {
  name: string;
  address: string;
  remark: string;
}

// ==================== Access Logs ====================

export interface AccessLogFilters {
  node_id?: string;
  remote_addr?: string;
  host?: string;
  path?: string;
  status_code?: number;
  since?: string;
  until?: string;
  p?: number;
  page_size?: number;
  sort_by?: string;
  sort_order?: 'asc' | 'desc';
}

export interface AccessLogOverviewFilters {
  node_id?: string;
  host?: string;
  hosts?: string[];
  hours?: number;
  bucket_minutes?: number;
}

export interface AccessLogOverviewMetricPoint {
  bucket_started_at: string;
  value: number;
}

export interface AccessLogOverview {
  generated_at: string;
  hours: number;
  bucket_minutes?: number;
  summary: {
    total_requests: number;
    total_visits: number;
    bandwidth_served: number;
  };
  trends: {
    requests: AccessLogOverviewMetricPoint[];
    visits: AccessLogOverviewMetricPoint[];
    bandwidth: AccessLogOverviewMetricPoint[];
  };
  top_paths: DistributionItem[];
  top_hosts: DistributionItem[];
  top_ips: DistributionItem[];
  device_types: DistributionItem[];
  top_browsers: DistributionItem[];
  top_operating_systems: DistributionItem[];
  top_user_agents: DistributionItem[];
  status_codes: DistributionItem[];
}

export interface AccessLogItem {
  id: string;
  node_id: string;
  node_name: string;
  logged_at: string;
  remote_addr: string;
  region: string;
  host: string;
  path: string;
  user_agent: string;
  cache_status: string;
  status_code: number;
  bytes_sent: number;
  request_length: number;
  request_time_ms: number;
  created_at: string;
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

export interface FoldedAccessLogIPFilters extends FoldedAccessLogFilters {
  bucket_started_at: string;
}

export interface FoldedAccessLogIPItem {
  remote_addr: string;
  request_count: number;
  success_count: number;
  client_error_count: number;
  server_error_count: number;
  last_seen_at: string;
}

export interface FoldedAccessLogIPList {
  items: FoldedAccessLogIPItem[];
  page: number;
  page_size: number;
  has_more: boolean;
  total_ip: number;
  bucket_started_at: string;
  fold_minutes: number;
  sort_by: string;
  sort_order: 'asc' | 'desc';
}

export interface AccessLogIPSummaryFilters {
  node_id?: string;
  remote_addr?: string;
  host?: string;
  hours?: number;
  since?: string;
  until?: string;
  p?: number;
  page_size?: number;
  sort_by?: string;
  sort_order?: 'asc' | 'desc';
}

export interface AccessLogIPSummaryItem {
  remote_addr: string;
  region?: string;
  total_requests: number;
  success_2xx_count: number;
  success_ratio: number;
  bytes_received: number;
  bytes_sent: number;
  /** @deprecated always 0 */
  recent_requests?: number;
  last_seen_at: string;
}

export interface AccessLogIPSummaryList {
  items: AccessLogIPSummaryItem[];
  page: number;
  page_size: number;
  has_more: boolean;
  total_ip: number;
  hours: number;
  since: string;
  until?: string;
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

export interface AccessLogIPAnalysisFilters {
  node_id?: string;
  remote_addr: string;
  host?: string;
  hours?: number;
}

export interface AccessLogIPAnalysis {
  remote_addr: string;
  hours: number;
  generated_at: string;
  summary: {
    total_requests: number;
    error_count: number;
    bandwidth_served: number;
    bytes_received: number;
    unique_hosts: number;
    unique_paths: number;
  };
  top_paths: DistributionItem[];
  top_hosts: DistributionItem[];
  status_codes: DistributionItem[];
  top_user_agents: DistributionItem[];
  device_types: DistributionItem[];
  top_browsers: DistributionItem[];
}

export interface AccessLogCleanupPayload {
  retention_days: number;
}

export interface AccessLogCleanupResult {
  retention_days: number;
  deleted_count: number;
  cutoff: string;
}

// ==================== Options (Performance) ====================

export interface OptionItem {
  key: string;
  value: string;
}

export interface OptionBatchPayload {
  options: OptionItem[];
}

export interface GeoIPLookupResult {
  provider: string;
  ip: string;
  iso_code: string;
  name: string;
  latitude?: number | null;
  longitude?: number | null;
}

export interface OpenFlarePublicStatus {
  version: string;
  start_time: number;
  server_address: string;
  system_name: string;
}

export interface IPMatchConfig {
  ips: string[];
  cidrs: string[];
  ip_group_ids: number[];
}

export interface GeoMatchConfig {
  countries: string[];
  regions: string[];
}

export interface PoWNodeConfig {
  algorithm: 'fast' | 'slow';
  difficulty: number;
  session_ttl: number;
  challenge_ttl: number;
}

export interface BlockNodeConfig {
  status_code: number;
  response_body: string;
}

export interface UACheckConfig {
  require_ua: boolean;
  browsers: string[];
  operating_systems: string[];
  match_mode: 'and' | 'or';
  block_common_bots: boolean;
  block_abnormal_ua: boolean;
  block_custom_ua: boolean;
  custom_ua_patterns: string[];
}

export interface SecurityCheckConfig {
  attack_group_id?: number;
  sql_injection?: boolean;
  path_traversal?: boolean;
  command_injection?: boolean;
  xss?: boolean;
  ssrf?: boolean;
  file_inclusion?: boolean;
  malicious_upload?: boolean;
  xxe?: boolean;
  crlf_injection?: boolean;
}

export type WAFRuleNode =
  | {
      id: string;
      type: 'start';
      label?: string;
      position: XYPosition;
      config: Record<string, never>;
    }
  | {
      id: string;
      type: 'ip_match';
      label?: string;
      position: XYPosition;
      config: IPMatchConfig;
    }
  | {
      id: string;
      type: 'geo_match';
      label?: string;
      position: XYPosition;
      config: GeoMatchConfig;
    }
  | {
      id: string;
      type: 'ua_check';
      label?: string;
      position: XYPosition;
      config: UACheckConfig;
    }
  | {
      id: string;
      type: 'security_check';
      label?: string;
      position: XYPosition;
      config: SecurityCheckConfig;
    }
  | {
      id: string;
      type: 'pow';
      label?: string;
      position: XYPosition;
      config: PoWNodeConfig;
    }
  | {
      id: string;
      type: 'allow';
      label?: string;
      position: XYPosition;
      config: Record<string, never>;
    }
  | {
      id: string;
      type: 'block';
      label?: string;
      position: XYPosition;
      config: BlockNodeConfig;
    };

export interface WAFRuleEdge {
  id: string;
  source: string;
  source_handle: string;
  target: string;
}

export interface WAFRuleGraph {
  schema_version: number;
  nodes: WAFRuleNode[];
  edges: WAFRuleEdge[];
}

export interface WAFRule {
  id: number;
  name: string;
  enabled: boolean;
  is_global: boolean;
  graph: WAFRuleGraph;
  revision: number;
  applied_site_ids: number[];
  applied_site_count: number;
  created_at: string;
  updated_at: string;
}

export interface WAFRuleGroupSite {
  id: number;
  site_name: string;
  domains: string[];
}

/** @deprecated Legacy fixed-chain rule shape retained for untouched binding consumers. */
export interface WAFRuleGroup extends WAFRule {
  block_status_code: number;
  block_response_body: string;
  ip_whitelist: string[];
  ip_blacklist: string[];
  ip_whitelist_group_ids: number[];
  ip_blacklist_group_ids: number[];
  country_whitelist: string[];
  country_blacklist: string[];
  region_whitelist: string[];
  region_blacklist: string[];
  pow_enabled: boolean;
  pow_config: ProxyRoutePoWConfig;
}

export interface WAFCreateRulePayload {
  name: string;
}

export interface WAFSaveRuleGraphPayload {
  revision: number;
  graph: WAFRuleGraph;
}

export interface WAFUpdateRuleMetaPayload {
  name: string;
  enabled: boolean;
}

export interface WAFRuleGroupPayload {
  name: string;
  enabled: boolean;
  block_status_code: number;
  block_response_body: string;
  ip_whitelist: string[];
  ip_blacklist: string[];
  ip_whitelist_group_ids: number[];
  ip_blacklist_group_ids: number[];
  country_whitelist: string[];
  country_blacklist: string[];
  region_whitelist: string[];
  region_blacklist: string[];
  pow_enabled: boolean;
  pow_config: ProxyRoutePoWConfig;
}

export interface WAFSiteRuleGroups {
  route_id: number;
  global_rule_group: WAFRule | null;
  rule_groups: WAFRule[];
  applied_rule_groups: WAFRule[];
  applied_ids: number[];
}

export type WAFIPGroupType = 'manual' | 'automatic' | 'subscription';
export type WAFIPGroupSubscriptionFormat = 'text' | 'json';

export interface WAFIPGroupExtIP {
  ip: string;
  captured_at: string;
}

export interface WAFIPGroup {
  id: number;
  name: string;
  type: WAFIPGroupType;
  enabled: boolean;
  ip_list: string[];
  auto_config: Record<string, unknown>;
  ext_ips?: WAFIPGroupExtIP[];
  subscription_url: string;
  subscription_format: WAFIPGroupSubscriptionFormat;
  subscription_mapping_rule: string;
  sync_interval_minutes: number;
  last_synced_at?: string;
  next_sync_at?: string;
  last_sync_status: string;
  last_sync_message: string;
  referenced_by_rule_count: number;
  created_at: string;
  updated_at: string;
}

export interface WAFIPGroupPayload {
  name: string;
  type: WAFIPGroupType;
  enabled: boolean;
  ip_list: string[];
  auto_config: Record<string, unknown>;
  subscription_url: string;
  subscription_format: WAFIPGroupSubscriptionFormat;
  subscription_mapping_rule: string;
  sync_interval_minutes: number;
}

export interface WAFIPGroupSyncResult {
  group: WAFIPGroup;
  ip_count: number;
  synced_at: string;
  next_sync_at: string;
  status: string;
  message: string;
}

export interface WAFIPGroupAutoTestPayload {
  auto_config: Record<string, unknown>;
}

export interface WAFIPGroupAutoTestResult {
  matched_ips: string[];
  matched_count: number;
  lookback: string;
  rule_count: number;
  tested_at: string;
}

export interface DashboardSummary {
  total_nodes: number;
  online_nodes: number;
  offline_nodes: number;
  pending_nodes: number;
  unhealthy_nodes: number;
}

export interface DashboardTraffic {
  request_count: number;
  status_4xx_count?: number;
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
  status_2xx_count: number;
  status_4xx_count: number;
  status_5xx_count: number;
}

export interface CapacityTrendPoint {
  bucket_started_at: string;
  average_cpu_usage_percent: number;
  average_memory_usage_percent: number;
  reported_nodes: number;
}

export interface NetworkTrendPoint {
  bucket_started_at: string;
  /** L1 接收数据 sum(request_length) */
  bytes_received: number;
  /** L1 已提供数据 sum(bytes_sent) */
  bytes_provided: number;
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
  status: NodeStatus;
  openresty_status: OpenrestyStatus;
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

export type CompactTrafficTrendPoint = [
  string,
  number,
  number,
  number,
  number,
  number,
  number,
];
export type CompactCapacityTrendPoint = [string, number, number, number];
/** [bucket, bytes_received, bytes_provided, reported_nodes] */
export type CompactNetworkTrendPoint = [string, number, number, number];
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

// ==================== Websites / TLS / DNS ====================

export interface ZoneItem {
  id: number;
  domain: string;
  /** Present on list API; may be omitted on nested zone objects. */
  domain_count?: number;
  created_at: string;
  updated_at: string;
}

export interface ZoneMutationPayload {
  domain: string;
}

export interface ZoneDomainItem {
  id: number;
  zone_id: number;
  proxy_route_id: number | null;
  domain: string;
  cert_id: number | null;
  created_at: string;
  updated_at: string;
}

export interface ZoneDomainMutationPayload {
  domain: string;
  cert_id: number | null;
}

export interface ZoneDomainCleanupResult {
  deleted_count: number;
}

export interface ZoneOverview {
  zone: ZoneItem;
  domains: ZoneDomainItem[];
}

export type ZoneStatsRange = '24h' | '7d' | '30d';

export interface ZoneStatsPoint {
  bucket_started_at: string;
  request_count: number;
  unique_visitors: number;
  bytes_sent: number;
}

export interface ZoneStats {
  range: ZoneStatsRange;
  range_hours: number;
  window_started_at: string;
  window_ended_at: string;
  bucket_minutes: number;
  unique_visitors: number;
  request_count: number;
  bytes_sent: number;
  domain_count: number;
  available: boolean;
  series: ZoneStatsPoint[];
}

export type CloudflareConnectionSource = 'dns_account' | 'standalone';
export type CloudflareSyncStatus = 'pending' | 'syncing' | 'ok' | 'error';

export interface CloudflareConnection {
  configured: boolean;
  ready: boolean;
  source: CloudflareConnectionSource | '';
  dns_account_id: number | null;
  status: string;
  verified_at: string | null;
}

export interface CloudflareConnectionPayload {
  source: CloudflareConnectionSource;
  dns_account_id: number;
  api_token: string;
}

export interface CloudflareNodeOption {
  id: number;
  name: string;
  ip: string;
}

export interface CloudflareGroup {
  id: number;
  name: string;
  target_mode: 'node' | 'custom';
  record_type: 'CNAME' | 'A' | 'AAAA';
  record_content: string;
  primary_node: CloudflareNodeOption;
  backup_node: CloudflareNodeOption | null;
  active_node: CloudflareNodeOption;
  /** 1 表示自动，手动 TTL 为 60..86400 秒。 */
  ttl: number;
  default_proxied: boolean;
  enabled: boolean;
  member_count: number;
  created_at: string;
  updated_at: string;
}

export interface CloudflareGroupPayload {
  name: string;
  target_mode: 'node' | 'custom';
  record_type: 'CNAME' | 'A' | 'AAAA';
  record_content: string;
  primary_node_id: number;
  backup_node_id: number | null;
  /** 省略时保留旧调用方行为；1 表示自动，手动为 60..86400 秒。 */
  ttl?: number;
  default_proxied: boolean;
  enabled: boolean;
}

export interface CloudflareMember {
  id: number;
  group_id: number;
  zone_domain_id: number;
  domain: string;
  zone_id: number;
  proxied: boolean;
  desired_ip: string;
  sync_status: CloudflareSyncStatus;
  last_error: string;
  synced_at: string | null;
}

export interface CloudflareMemberCreatePayload {
  zone_domain_id: number;
  proxied?: boolean;
}

export interface CloudflareGroupDetail {
  group: CloudflareGroup;
  members: CloudflareMember[];
}

export interface CloudflareAvailableDomain {
  id: number;
  zone_id: number;
  domain: string;
  /** Zone apex / root domain used for hierarchical grouping. */
  zone_domain: string;
}

export interface CloudflareOverview {
  connection: CloudflareConnection;
  group_count: number;
  member_count: number;
  ok_count: number;
  pending_count: number;
  error_count: number;
}

export interface CloudflareSyncReceipt {
  task_id: string;
}

export interface TlsCertificateItem {
  id: number;
  name: string;
  cert_pem?: string;
  key_pem?: string;
  provider: string;
  acme_account_id: number;
  dns_account_id: number;
  key_algorithm: string;
  auto_renew: boolean;
  primary_domain: string;
  other_domains: string;
  disable_cname: boolean;
  skip_dns: boolean;
  dns1: string;
  dns2: string;
  apply_status: string;
  apply_message: string;
  not_before: string;
  not_after: string;
  remark: string;
  created_at: string;
  updated_at: string;
}

export interface TlsCertificateListItem extends TlsCertificateItem {
  dns_names: string[];
}

export interface TlsCertificateDetailItem extends TlsCertificateItem {
  cert_pem?: never;
  key_pem?: never;
}

export interface TlsCertificateContentItem extends TlsCertificateItem {
  cert_pem: string;
  key_pem: string;
}

export interface TlsCertificateMutationPayload {
  name: string;
  cert_pem: string;
  key_pem: string;
  remark: string;
}

export interface TlsCertificateApplyPayload {
  name: string;
  remark: string;
  acme_account_id: number;
  dns_account_id: number;
  key_algorithm: string;
  auto_renew: boolean;
  primary_domain: string;
  other_domains: string;
  disable_cname: boolean;
  skip_dns: boolean;
  dns1: string;
  dns2: string;
}

export interface TlsCertificateFileImportPayload {
  name: string;
  remark: string;
  certFile: File;
  keyFile: File;
}

export interface AcmeAccountItem {
  id: number;
  email: string;
  url: string;
  created_at: string;
  updated_at: string;
}

export interface DnsAccountItem {
  id: number;
  name: string;
  type: string;
  created_at: string;
  updated_at: string;
}

export interface DnsAccountMutationPayload {
  name: string;
  type: string;
  authorization: string;
}

// ==================== Tunnel / Rathole ====================

export type TunnelTransport = 'auto' | 'noise' | 'websocket' | 'tls' | 'tcp';

export interface TunnelPoolInput {
  pool_key: string;
  name: string;
  description?: string;
  region?: string;
  is_default?: boolean;
  routing_policy?: Record<string, unknown>;
}

export interface TunnelPoolView {
  id: number;
  pool_key: string;
  name: string;
  description: string;
  region: string;
  is_default: boolean;
  routing_policy: Record<string, unknown>;
  relay_count: number;
  tunnel_count: number;
  created_at: string;
  updated_at: string;
}

export interface TunnelRelayInput {
  pool_id?: number;
  pool_key?: string;
  name: string;
  public_addr: string;
  ip_manual_override?: boolean;
  transport_port?: number;
  ws_port?: number;
  gateway_addr?: string;
  gateway_port?: number;
  gateway_tls?: boolean;
  supported_transports?: string[];
  geo_name?: string;
  geo_latitude?: number | null;
  geo_longitude?: number | null;
  geo_manual_override?: boolean;
  zone_tag?: string;
  colocated_edge_node_id?: string;
  weight?: number;
  max_tunnels?: number;
  auto_update_enabled?: boolean;
  update_channel?: ReleaseChannel | '';
  update_tag?: string;
  metadata?: Record<string, unknown>;
}

export interface TunnelRelayDrainInput {
  draining: boolean;
}

export interface TunnelBinaryUpdateInput {
  channel?: ReleaseChannel | '';
  tag_name?: string;
}

export interface TunnelRelayView {
  id: number;
  relay_id: string;
  pool_id: number;
  pool_key: string;
  name: string;
  public_addr: string;
  ip_manual_override: boolean;
  transport_port: number;
  ws_port: number;
  gateway_addr: string;
  gateway_port: number;
  gateway_tls: boolean;
  supported_transports: string[];
  noise_public_key: string;
  geo_name: string;
  geo_latitude?: number | null;
  geo_longitude?: number | null;
  geo_manual_override: boolean;
  zone_tag: string;
  colocated_edge_node_id: string;
  weight: number;
  max_tunnels: number;
  draining: boolean;
  status: string;
  effective_status: string;
  ws_connected: boolean;
  runtime_status: string;
  active_connectors: number;
  active_streams: number;
  cpu_usage_percent: number;
  memory_usage_percent: number;
  version: string;
  engine_version: string;
  contract_version: number;
  capabilities: string[];
  auto_update_enabled: boolean;
  update_requested: boolean;
  update_channel: string;
  update_tag: string;
  last_seen_at?: string | null;
  last_error: string;
  quarantined_at?: string | null;
  quarantine_reason: string;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface TunnelRelayDeployView {
  relay: TunnelRelayView;
  access_token: string;
  bootstrap_token: string;
  server_url: string;
  deploy_command: string;
}

export interface TunnelRelayBootstrapView {
  discovery_token: string;
  bootstrap_token: string;
  server_url: string;
  deploy_command: string;
}

export interface TunnelInput {
  name: string;
  description?: string;
  pool_id?: number;
  pool_key?: string;
  ha_relay_count?: number;
  preferred_transport?: string;
  fallback_transports?: string[];
  client_proxy_url?: string;
  allowed_target_cidrs?: string[];
  blocked_target_cidrs?: string[];
  max_connectors?: number;
  enabled?: boolean;
  auto_update_enabled?: boolean;
  update_channel?: ReleaseChannel | '';
  update_tag?: string;
  metadata?: Record<string, unknown>;
}

export interface TunnelServiceInput {
  name: string;
  service_type: 'tcp' | 'udp';
  relay_id: number;
  listen_port: number;
  target_addr: string;
  enabled?: boolean;
  options?: Record<string, unknown>;
}

export interface TunnelConnectedRelayReport {
  relay_id: number;
  relay_public_id?: string;
  service_name: string;
  transport: string;
  state: string;
  rtt_ms: number;
  active_streams: number;
}

export interface TunnelConnectorView {
  id: number;
  connector_id: string;
  tunnel_id: number;
  hostname: string;
  os_arch: string;
  public_ip: string;
  version: string;
  engine_version: string;
  contract_version: number;
  capabilities: string[];
  applied_revision: number;
  status: string;
  effective_status: string;
  ws_connected: boolean;
  last_error: string;
  connected_relays: TunnelConnectedRelayReport[];
  draining: boolean;
  revoked_at?: string | null;
  last_seen_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface TunnelAttachmentView {
  id: number;
  tunnel_id: number;
  connector_id: string;
  relay_id: number;
  relay_name: string;
  relay_public_id: string;
  service_name: string;
  transport: string;
  state: string;
  rtt_ms: number;
  active_streams: number;
  relay_verified_at?: string | null;
  lease_expires_at?: string | null;
  updated_at: string;
}

export interface TunnelServiceView {
  id: number;
  tunnel_id: number;
  name: string;
  service_type: 'tcp' | 'udp';
  relay_id: number;
  listen_port: number;
  target_addr: string;
  enabled: boolean;
  options: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface TunnelBoundRouteView {
  route_id: number;
  site_name: string;
  domains: string[];
  enabled: boolean;
  tunnel_target_addr: string;
  tunnel_target_protocol: string;
}

export interface TunnelView {
  id: number;
  tunnel_id: string;
  name: string;
  description: string;
  pool_id: number;
  pool_key: string;
  ha_relay_count: number;
  preferred_transport: string;
  fallback_transports: string[];
  client_proxy_url: string;
  allowed_target_cidrs: string[];
  blocked_target_cidrs: string[];
  max_connectors: number;
  enabled: boolean;
  config_revision: number;
  status: string;
  online_connectors: number;
  active_attachments: number;
  bound_routes_count: number;
  auto_update_enabled: boolean;
  update_requested: boolean;
  update_channel: string;
  update_tag: string;
  quarantined_at?: string | null;
  quarantine_reason: string;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface TunnelDeployView {
  tunnel: TunnelView;
  access_token: string;
  bootstrap_token: string;
  server_url: string;
  deploy_command: string;
}

export interface TunnelDetailView {
  tunnel: TunnelView;
  connectors: TunnelConnectorView[];
  attachments: TunnelAttachmentView[];
  services: TunnelServiceView[];
  bound_routes: TunnelBoundRouteView[];
}

export interface TunnelTopologyView {
  pools: TunnelPoolView[];
  relays: TunnelRelayView[];
  tunnels: TunnelView[];
  connectors: TunnelConnectorView[];
  attachments: TunnelAttachmentView[];
}

export interface TunnelReleaseInfo {
  tag_name: string;
  body: string;
  html_url: string;
  published_at: string;
  channel: ReleaseChannel;
  prerelease: boolean;
}

export type NetToolsPingProtocol = 'icmp' | 'tcp' | 'http';
export type NetToolsTracerouteProtocol = 'icmp' | 'udp' | 'tcp';
export type NetToolsTaskStatus = 'running' | 'completed' | 'timeout';
export type NetToolsNodeResultStatus =
  | 'pending'
  | 'completed'
  | 'failed'
  | 'offline'
  | 'timeout';

export interface NetToolsPingPayload {
  target: string;
  protocol: NetToolsPingProtocol;
  port?: number;
  count?: number;
  interval_ms?: number;
  timeout_ms?: number;
  node_ids?: string[];
}

export interface NetToolsPingSample {
  seq: number;
  rtt_ms: number;
  success: boolean;
}

export interface NetToolsPingBatchResult {
  protocol: string;
  target: string;
  resolved_ip?: string;
  sent: number;
  received: number;
  loss_rate: number;
  min_ms: number;
  avg_ms: number;
  max_ms: number;
  p95_ms: number;
  jitter_ms: number;
  samples: NetToolsPingSample[];
  error?: string;
}

export interface NetToolsTraceroutePayload {
  target: string;
  protocol: NetToolsTracerouteProtocol;
  port?: number;
  max_hops?: number;
  probes_per_hop?: number;
  timeout_ms?: number;
  node_ids?: string[];
}

export interface NetToolsTracerouteProbe {
  seq: number;
  ip?: string;
  rtt_ms?: number;
}

export interface NetToolsTracerouteHop {
  ttl: number;
  ip?: string;
  alt_ips?: string[];
  sent: number;
  recv: number;
  loss_pct: number;
  min_ms?: number;
  avg_ms?: number;
  max_ms?: number;
  jitter_ms?: number;
  probes: NetToolsTracerouteProbe[];
  country_iso?: string;
  city_name?: string;
  asn?: number;
  asn_name?: string;
  asn_tags?: string[];
}

export interface NetToolsTracerouteResult {
  target: string;
  resolved_ip?: string;
  protocol: string;
  hop_count: number;
  reached: boolean;
  total_rtt_ms: number;
  hops: NetToolsTracerouteHop[];
  error?: string;
}

export interface NetToolsDNSQueryPayload {
  domain: string;
  record_type: string;
  resolver?: string;
  ecs_subnet?: string;
  timeout_ms?: number;
  node_ids?: string[];
}

export interface NetToolsDNSRecord {
  name: string;
  record_type: string;
  ttl: number;
  value: string;
  country_iso?: string;
  city_name?: string;
  asn?: number;
  asn_name?: string;
  asn_tags?: string[];
}

export interface NetToolsDirectDNSResult {
  domain: string;
  record_type: string;
  resolver: string;
  ecs_subnet?: string;
  rcode: string;
  authoritative: boolean;
  truncated: boolean;
  used_tcp: boolean;
  latency_ms: number;
  records: NetToolsDNSRecord[];
}

export interface NetToolsEdgeDNSResult {
  domain: string;
  record_type: string;
  resolver: string;
  rcode: string;
  authoritative: boolean;
  truncated: boolean;
  used_tcp: boolean;
  latency_ms: number;
  records: NetToolsDNSRecord[];
  error?: string;
}

export interface NetToolsProbeNodeResult {
  node_id: string;
  node_name: string;
  node_geo: string;
  node_asn: number;
  status: NetToolsNodeResultStatus;
  ping?: NetToolsPingBatchResult;
  traceroute?: NetToolsTracerouteResult;
  dns?: NetToolsEdgeDNSResult;
  error?: string;
  finished_at?: string;
}

export interface NetToolsProbeTaskState {
  task_id: string;
  kind: 'ping_batch' | 'traceroute' | 'dns_query';
  target: string;
  created_at: string;
  expires_at: string;
  total_nodes: number;
  completed_nodes: number;
  status: NetToolsTaskStatus;
  nodes: NetToolsProbeNodeResult[];
}

export interface NetToolsDNSQueryResponse {
  mode: 'direct' | 'nodes';
  direct_result?: NetToolsDirectDNSResult;
  task?: NetToolsProbeTaskState;
}

export interface NetToolsTracerouteHistoryItem {
  id: number;
  task_id: string;
  trigger_type: 'manual' | 'auto';
  node_id: string;
  target: string;
  resolved_ip: string;
  target_asn: number;
  target_country: string;
  protocol: string;
  hop_count: number;
  reached: boolean;
  total_rtt_ms: number;
  hops: NetToolsTracerouteHop[];
  as_path: number[];
  error_message?: string;
  created_at: string;
}

export interface NetToolsTracerouteHistoryQuery {
  p?: number;
  page_size?: number;
  node_id?: string;
  target?: string;
  trigger_type?: string;
  target_asn?: number;
}

export interface NetToolsTracerouteHistoryPage {
  total: number;
  results: NetToolsTracerouteHistoryItem[];
}

export interface NetToolsIPLookupResult {
  ip: string;
  country_iso: string;
  country_name: string;
  city_name: string;
  latitude?: number | null;
  longitude?: number | null;
  asn: number;
  asn_name: string;
  asn_class: string;
  asn_country: string;
  asn_tags: string[];
  prefix: string;
  prefix_hits: number;
  data_sources: string[];
}

export interface NetToolsASNInfo {
  asn: number;
  name: string;
  class: string;
  country: string;
  tags: string[];
  ipv4_prefixes: number;
  ipv6_prefixes: number;
  neighbor_count: number;
}

export interface NetToolsASNeighbor {
  peer_asn: number;
  peer_name: string;
  peer_country: string;
  peer_class: string;
  relation: string;
  tags?: string[];
  observed_rtt_ms?: number;
  sample_count?: number;
}

export interface NetToolsIPLookupResponse {
  query: string;
  resolved_ips: string[];
  primary?: NetToolsIPLookupResult;
  all_results?: NetToolsIPLookupResult[];
  asn_info?: NetToolsASNInfo;
  neighbors: NetToolsASNeighbor[];
  attribution: string;
}

export interface NetToolsASNDetailResponse {
  asn_info: NetToolsASNInfo;
  neighbors: NetToolsASNeighbor[];
  attribution: string;
}

export interface NetToolsPrefixRecord {
  cidr: string;
  asn: number;
  hits: number;
  family: 'ipv4' | 'ipv6';
}

export interface NetToolsASNPrefixesResponse {
  asn: number;
  total: number;
  results: NetToolsPrefixRecord[];
  attribution: string;
}

export interface NetToolsASPathInput {
  target_asn?: number;
  target_ip?: string;
  node_ids?: string[];
}

export interface NetToolsASPathHopNode {
  asn: number;
  name: string;
  country: string;
  tags?: string[];
}

export interface NetToolsNodeASPathComparison {
  node_id: string;
  node_name: string;
  node_ip: string;
  node_geo: string;
  node_asn: number;
  node_asn_name: string;
  as_hops: number;
  path: NetToolsASPathHopNode[];
  effective_rtt_ms: number;
  quality_score: number;
  confidence: number;
  data_source: string;
}

export interface NetToolsASPathResponse {
  target_asn: number;
  target_asn_info?: NetToolsASNInfo;
  nodes: NetToolsNodeASPathComparison[];
  attribution: string;
}

export interface NetToolsQualityMatrixItem {
  id: number;
  node_id: string;
  node_asn: number;
  node_name: string;
  node_geo: string;
  node_status: string;
  client_asn: number;
  client_country: string;
  client_asn_name: string;
  tcp_rtt_p50_ms: number;
  tcp_rtt_p95_ms: number;
  tcp_min_rtt_ms: number;
  tcp_retrans_rate: number;
  tcp_sample_count: number;
  tcp_last_seen_at?: string | null;
  trace_rtt_p50_ms: number;
  trace_loss_rate: number;
  trace_hop_count: number;
  trace_as_path: string;
  trace_sample_count: number;
  trace_last_seen_at?: string | null;
  effective_rtt_ms: number;
  predicted_rtt_ms: number;
  quality_score: number;
  confidence: number;
  data_source: 'measured' | 'predicted' | string;
  updated_at: string;
}

export interface NetToolsQualityMatrixQuery {
  p?: number;
  page_size?: number;
  node_id?: string;
  client_asn?: number;
  client_country?: string;
  data_source?: string;
  search?: string;
  sort_by?: string;
  sort_order?: 'asc' | 'desc';
}

export interface NetToolsQualityMatrixPage {
  total: number;
  results: NetToolsQualityMatrixItem[];
}

export interface NetToolsDatasetStatus {
  key: string;
  filename: string;
  loaded: boolean;
  path: string;
  size_bytes: number;
  record_count: number;
  updated_at?: string | null;
  source_url: string;
  license: string;
  error?: string;
}

export interface NetToolsNetIntelStatusResponse {
  data_dir: string;
  datasets: NetToolsDatasetStatus[];
  caida_enabled: boolean;
  auto_probe_enabled: boolean;
  attribution: string;
}

export interface NetToolsTriggerSyncResponse {
  task_id?: string;
  mode: 'queued' | 'inline';
  summary?: {
    data_dir: string;
    geoip_source: string;
    caida_enabled: boolean;
    updated_files: string[];
    skipped_files?: string[];
    errors?: Record<string, string>;
    backfilled_nodes: number;
    duration_ms: number;
  };
}

export interface NetToolsASUpstreamNode {
  asn: number;
  name: string;
  short_name: string;
  country: string;
  class: string;
  tags?: string[];
  role: 'origin' | 'transit' | 'tier1_upstream' | 'tier1';
  layer: number;
  is_tier1: boolean;
  path_hits: number;
}

export interface NetToolsASUpstreamEdge {
  from: number;
  to: number;
  kind: 'upstream' | 'tier1_interconnect' | 'observed';
  path_hits: number;
  observed_rtt_ms?: number;
  sample_count?: number;
}

export interface NetToolsASNUpstreamGraphResponse {
  origin_asn: number;
  sampled_prefix?: string;
  data_source:
    | 'caida'
    | 'ripestat_lg'
    | 'ripestat_neighbours'
    | 'observed'
    | 'none';
  attribution: string;
  total_paths_parsed: number;
  nodes: NetToolsASUpstreamNode[];
  edges: NetToolsASUpstreamEdge[];
  cached: boolean;
  generated_at: string;
}
