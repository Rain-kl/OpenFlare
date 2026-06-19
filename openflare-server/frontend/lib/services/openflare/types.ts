export type ReleaseChannel = 'stable' | 'preview';

export type NodeType = 'edge_node' | 'tunnel_relay' | 'tunnel_client';

export type NodeStatus = 'online' | 'offline' | 'pending';

export type OpenrestyStatus = 'healthy' | 'unhealthy' | 'unknown';

export type ApplyResult = 'success' | 'warning' | 'failed' | '';

export interface NodeItem {
  id: number;
  node_id: string;
  node_type: NodeType;
  name: string;
  ip: string;
  ip_manual_override: boolean;
  relay_bind_port: number;
  relay_vhost_http_port: number;
  relay_client_access_addr: string;
  relay_agent_access_addr: string;
  relay_client_proxy_url: string;
  relay_auth_token: string;
  relay_status: string;
  relay_web_server_enabled: boolean;
  relay_frps_connections: number;
  relay_frps_proxy_count: number;
  geo_name: string;
  geo_latitude?: number | null;
  geo_longitude?: number | null;
  geo_manual_override: boolean;
  access_token: string;
  auto_update_enabled: boolean;
  update_requested: boolean;
  update_channel: ReleaseChannel;
  update_tag: string;
  restart_openresty_requested: boolean;
  version: string;
  ext_version: string;
  openresty_status: OpenrestyStatus;
  openresty_message: string;
  status: NodeStatus;
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
}

export interface NodeBootstrapToken {
  discovery_token: string;
}

export interface NodeMutationPayload {
  node_type: NodeType;
  name: string;
  ip: string;
  ip_manual_override: boolean;
  relay_bind_port?: number;
  relay_vhost_http_port?: number;
  relay_client_access_addr?: string;
  relay_agent_access_addr?: string;
  relay_client_proxy_url?: string;
  relay_web_server_enabled?: boolean;
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
  total_memory_bytes: number;
  total_disk_bytes: number;
  uptime_seconds: number;
  reported_at: string;
}

export interface NodeMetricSnapshot {
  captured_at: string;
  cpu_usage_percent: number;
  memory_used_bytes: number;
  memory_total_bytes: number;
  storage_used_bytes: number;
  storage_total_bytes: number;
  disk_read_bytes: number;
  disk_write_bytes: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
  openresty_rx_bytes: number;
  openresty_tx_bytes: number;
  openresty_connections: number;
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

export interface NodeTrafficReport {
  window_started_at: string;
  window_ended_at: string;
  request_count: number;
  error_count: number;
  unique_visitor_count: number;
  status_codes_json: string;
  top_domains_json: string;
  source_countries_json: string;
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
  traffic_reports?: NodeTrafficReport[];
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

export interface ProxyRouteItem {
  id: number;
  site_name: string;
  domain: string;
  domains: string[];
  primary_domain: string;
  domain_count: number;
  origin_id: number | null;
  origin_url: string;
  origin_host: string;
  upstreams: string;
  upstream_list: string[];
  enabled: boolean;
  enable_https: boolean;
  cert_id: number | null;
  cert_ids: number[];
  domain_cert_ids: number[];
  redirect_http: boolean;
  limit_conn_per_server: number;
  limit_conn_per_ip: number;
  limit_rate: string;
  cache_enabled: boolean;
  cache_policy: string;
  cache_rules: string;
  cache_rule_list: string[];
  custom_headers: string;
  custom_header_list: ProxyRouteCustomHeader[];
  pow_enabled: boolean;
  pow_config: ProxyRoutePoWConfig;
  basic_auth_enabled: boolean;
  basic_auth_username: string;
  basic_auth_password: string;
  remark: string;
  upstream_type: 'direct' | 'tunnel' | 'pages';
  tunnel_node_id?: number | null;
  tunnel_id?: number | null;
  tunnel_target_addr?: string;
  tunnel_target_protocol?: string;
  pages_project_id?: number | null;
  created_at: string;
  updated_at: string;
}

export interface ProxyRouteMutationPayload {
  site_name?: string;
  domain: string;
  domains?: string[];
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
  cert_id: number | null;
  cert_ids?: number[];
  domain_cert_ids?: number[];
  redirect_http: boolean;
  limit_conn_per_server?: number;
  limit_conn_per_ip?: number;
  limit_rate?: string;
  cache_enabled: boolean;
  cache_policy: string;
  cache_rules: string[];
  custom_headers: ProxyRouteCustomHeader[];
  pow_enabled: boolean;
  pow_config: string;
  basic_auth_enabled: boolean;
  basic_auth_username?: string;
  basic_auth_password?: string;
  remark: string;
  upstream_type?: 'direct' | 'tunnel' | 'pages';
  tunnel_node_id?: number | null;
  tunnel_id?: number | null;
  tunnel_target_addr?: string;
  tunnel_target_protocol?: string;
  pages_project_id?: number | null;
}

export interface ConfigVersionSummary {
  id: number;
  version: string;
  checksum: string;
  is_active: boolean;
  created_by: string;
  created_at: string;
}

export interface ConfigVersionDetail extends ConfigVersionSummary {
  snapshot_json: string;
  main_config: string;
  rendered_config: string;
  support_files_json: string;
}

export interface SupportFile {
  path: string;
  content: string;
}

export interface ConfigPreviewResult {
  snapshot_json: string;
  main_config: string;
  route_config: string;
  rendered_config: string;
  support_files: SupportFile[];
  checksum: string;
  route_count: number;
  website_count: number;
}

export interface ConfigOptionDiffItem {
  key: string;
  previous_value: string;
  current_value: string;
}

export interface ConfigDiffResult {
  active_version?: string;
  added_sites: string[];
  removed_sites: string[];
  modified_sites: string[];
  added_domains: string[];
  removed_domains: string[];
  modified_domains: string[];
  main_config_changed: boolean;
  waf_config_changed: boolean;
  changed_option_keys: string[];
  changed_option_details: ConfigOptionDiffItem[];
  current_website_count: number;
  active_website_count: number;
}

export interface ConfigVersionCleanupPayload {
  keep_count: number;
}

export interface ConfigVersionCleanupResult {
  deleted_count: number;
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
  root_dir?: string;
  entry_file: string;
  created_by: string;
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
  rootDir?: string;
  entryFile?: string;
  onProgress?: (percent: number) => void;
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

export type DatabaseCleanupTarget =
  | 'node_access_logs'
  | 'node_metric_snapshots'
  | 'node_request_reports';

export interface DatabaseCleanupPayload {
  target: DatabaseCleanupTarget;
  retention_days?: number;
}

export interface DatabaseCleanupResult {
  target: DatabaseCleanupTarget;
  target_label: string;
  deleted_count: number;
  delete_all: boolean;
  retention_days?: number;
  cutoff?: string;
}

export interface OpenFlarePublicStatus {
  version: string;
  start_time: number;
  server_address: string;
  system_name: string;
}

export interface WAFRuleGroup {
  id: number;
  name: string;
  enabled: boolean;
  is_global: boolean;
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
  remark: string;
  applied_site_ids: number[];
  applied_site_count: number;
  created_at: string;
  updated_at: string;
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
  remark: string;
}

export interface WAFSiteRuleGroups {
  route_id: number;
  global_rule_group: WAFRuleGroup | null;
  rule_groups: WAFRuleGroup[];
  applied_rule_groups: WAFRuleGroup[];
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
  remark: string;
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
  remark: string;
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
  lookback_minutes: number;
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

// ==================== Websites / TLS / DNS ====================

export interface ManagedDomainItem {
  id: number;
  domain: string;
  cert_id: number | null;
  enabled: boolean;
  remark: string;
  created_at: string;
  updated_at: string;
}

export interface ManagedDomainMutationPayload {
  domain: string;
  cert_id: number | null;
  enabled: boolean;
  remark: string;
}

export interface ManagedDomainMatchCandidate {
  managed_domain_id: number;
  domain: string;
  match_type: 'exact' | 'wildcard' | string;
  certificate_id: number;
  certificate_name: string;
}

export interface ManagedDomainMatchResult {
  domain: string;
  matched: boolean;
  candidate?: ManagedDomainMatchCandidate;
  candidates: ManagedDomainMatchCandidate[];
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