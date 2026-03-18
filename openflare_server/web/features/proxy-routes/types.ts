export interface ProxyRouteCustomHeader {
  key: string;
  value: string;
}

export interface ProxyRouteItem {
  id: number;
  domain: string;
  origin_url: string;
  origin_host: string;
  upstreams: string;
  enabled: boolean;
  enable_https: boolean;
  cert_id: number | null;
  redirect_http: boolean;
  cache_enabled: boolean;
  cache_policy: string;
  cache_rules: string;
  custom_headers: string;
  remark: string;
  created_at: string;
  updated_at: string;
}

export interface ProxyRouteMutationPayload {
  domain: string;
  origin_url: string;
  origin_host: string;
  upstreams: string[];
  enabled: boolean;
  enable_https: boolean;
  cert_id: number | null;
  redirect_http: boolean;
  cache_enabled: boolean;
  cache_policy: string;
  cache_rules: string[];
  custom_headers: ProxyRouteCustomHeader[];
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
