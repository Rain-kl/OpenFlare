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
  remark: string;
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
  remark: string;
}

export interface TlsCertificateItem {
  id: number;
  name: string;
  not_after?: string | null;
}

export interface ManagedDomainMatchCandidate {
  managed_domain_id: number;
  domain: string;
  match_type: 'exact' | 'wildcard';
  certificate_id: number;
  certificate_name: string;
}

export interface ManagedDomainMatchResult {
  domain: string;
  matched: boolean;
  candidate?: ManagedDomainMatchCandidate;
  candidates: ManagedDomainMatchCandidate[];
}
