import type {
  ProxyRouteConfigSection,
  ProxyRouteCustomHeader,
  ProxyRouteItem,
  ProxyRouteMutationPayload,
  ZoneDomainItem,
} from '@/lib/services/openflare';
import { ZoneService } from '@/lib/services/openflare';

export type TranslateFn = (
  key: string,
  values?: Record<string, string | number>,
) => string;

const proxyRouteConfigSectionKeys = [
  'domains',
  'limits',
  'proxy',
  'cache',
  'waf',
  'auth',
] as const satisfies ReadonlyArray<ProxyRouteConfigSection>;

export function getProxyRouteConfigSections(t: TranslateFn) {
  return [
    {
      key: 'domains' as const,
      label: t('sections.domains'),
      description: t('sections.domainsDesc'),
    },
    {
      key: 'limits' as const,
      label: t('sections.limits'),
      description: t('sections.limitsDesc'),
    },
    {
      key: 'proxy' as const,
      label: t('sections.proxy'),
      description: t('sections.proxyDesc'),
    },
    {
      key: 'cache' as const,
      label: t('sections.cache'),
      description: t('sections.cacheDesc'),
    },
    {
      key: 'waf' as const,
      label: t('sections.waf'),
      description: t('sections.wafDesc'),
    },
    {
      key: 'auth' as const,
      label: t('sections.auth'),
      description: t('sections.authDesc'),
    },
  ];
}

const domainPattern =
  /^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/i;

export function getProxyRouteConfigSection(
  value: string | null | undefined,
): ProxyRouteConfigSection {
  return proxyRouteConfigSectionKeys.some((key) => key === value)
    ? (value as ProxyRouteConfigSection)
    : 'domains';
}

export function validateDomain(domain: string, t: TranslateFn): string | null {
  const normalized = domain.trim().toLowerCase();
  if (!normalized) {
    return t('validation.enterDomain');
  }
  if (!domainPattern.test(normalized)) {
    return t('validation.invalidDomain');
  }
  return null;
}

export function parseOriginUrl(originUrl: string) {
  const parsed = new URL(originUrl);
  const port = parsed.port || (parsed.protocol === 'http:' ? '80' : '443');
  const path = parsed.pathname === '/' ? '' : parsed.pathname;

  return {
    scheme: parsed.protocol.replace(':', '') as 'http' | 'https',
    address: parsed.hostname,
    port,
    uri: parsed.search ? `${path}${parsed.search}` || parsed.search : path,
  };
}

/** Domain FQDNs bound to a proxy route (from zone_domains). */
export function getRouteDomainNames(route: ProxyRouteItem): string[] {
  return (route.zone_domains ?? []).map((item) => item.domain).filter(Boolean);
}

export function getRoutePrimaryDomain(route: ProxyRouteItem): string {
  return getRouteDomainNames(route)[0] ?? '';
}

export function getRouteDomainsLabel(
  route: ProxyRouteItem,
  t: TranslateFn,
): string {
  const names = getRouteDomainNames(route);
  return names.length > 0 ? names.join(', ') : t('unboundDomain');
}

/** Upstream address labels for display (supports multi-upstream). */
export function getUpstreamLabels(
  route: ProxyRouteItem,
  t: TranslateFn,
): string[] {
  if (route.upstream_type === 'pages') {
    return [
      route.pages_project_id
        ? t('pagesProjectId', { id: route.pages_project_id })
        : t('pagesUnbound'),
    ];
  }
  if (route.upstream_type === 'tunnel') {
    const protocol = route.tunnel_target_protocol || 'http';
    const target = route.tunnel_target_addr || t('tunnelTargetUnset');
    return [`Tunnel → ${protocol}://${target}`];
  }
  const list = (route.upstream_list ?? []).filter(Boolean);
  if (list.length > 0) {
    return list;
  }
  return route.origin_url ? [route.origin_url] : [];
}

export function getUpstreamSummary(
  route: ProxyRouteItem,
  t: TranslateFn,
): string {
  const labels = getUpstreamLabels(route, t);
  if (labels.length === 0) {
    return t('upstreamUnset');
  }
  if (labels.length === 1) {
    return labels[0];
  }
  return labels.join(' · ');
}

const originHostPattern =
  /^(?:(?:[a-z0-9-]+\.)*[a-z0-9-]+|\[[0-9a-f:.]+\]|[0-9.]+)(?::\d{1,5})?$/i;
const headerKeyPattern = /^[A-Za-z0-9_-]+$/;
const limitRatePattern = /^\d+(?:[kKmM])?$/;

export function getErrorMessage(error: unknown, fallback?: string) {
  return error instanceof Error
    ? error.message
    : (fallback ?? '请求失败，请稍后重试。');
}

export function linesFromTextarea(value: string) {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function validateDomains(domains: string[], t: TranslateFn) {
  if (domains.length === 0) {
    return t('validation.enterAtLeastOneDomain');
  }

  const seen = new Set<string>();
  for (const domain of domains) {
    const normalized = domain.trim().toLowerCase();
    const error = validateDomain(normalized, t);
    if (error && normalized) {
      return t('validation.invalidDomainWithValue', { domain });
    }
    if (!normalized) {
      continue;
    }
    if (seen.has(normalized)) {
      return t('validation.duplicateDomain', { domain });
    }
    seen.add(normalized);
  }

  return null;
}

export function parseOriginUrls(value: string, t: TranslateFn) {
  const urls = linesFromTextarea(value);
  if (urls.length === 0) {
    return { urls: [], error: t('validation.enterAtLeastOneUpstream') };
  }

  let sharedScheme = '';
  for (const originUrl of urls) {
    let parsed: URL;
    try {
      parsed = new URL(originUrl);
    } catch {
      return {
        urls: [],
        error: t('validation.invalidUpstream', { url: originUrl }),
      };
    }

    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      return {
        urls: [],
        error: t('validation.upstreamMustBeHttp', { url: originUrl }),
      };
    }

    if (!parsed.hostname) {
      return {
        urls: [],
        error: t('validation.upstreamMissingHost', { url: originUrl }),
      };
    }

    if (urls.length > 1) {
      if ((parsed.pathname && parsed.pathname !== '/') || parsed.search) {
        return {
          urls: [],
          error: t('validation.multiUpstreamNoPath'),
        };
      }

      if (!sharedScheme) {
        sharedScheme = parsed.protocol;
      } else if (sharedScheme !== parsed.protocol) {
        return {
          urls: [],
          error: t('validation.multiUpstreamSameProtocol'),
        };
      }
    }
  }

  return { urls, error: null };
}

export function validateOriginHost(value: string, t: TranslateFn) {
  const normalized = value.trim();
  if (!normalized) {
    return null;
  }
  if (
    normalized.includes('://') ||
    /[/\\\s]/.test(normalized) ||
    !originHostPattern.test(normalized)
  ) {
    return t('validation.invalidOriginHost');
  }
  return null;
}

export function parseCustomHeadersText(value: string, t: TranslateFn) {
  const lines = linesFromTextarea(value);
  const headers: ProxyRouteCustomHeader[] = [];

  for (const line of lines) {
    const separatorIndex = line.indexOf(':');
    if (separatorIndex <= 0) {
      return {
        headers: [],
        error: t('validation.invalidHeader', { line }),
      };
    }

    const key = line.slice(0, separatorIndex).trim();
    const headerValue = line.slice(separatorIndex + 1).trim();

    if (!headerKeyPattern.test(key)) {
      return {
        headers: [],
        error: t('validation.invalidHeaderName', { key }),
      };
    }

    headers.push({ key, value: headerValue });
  }

  return { headers, error: null };
}

export function customHeadersToText(headers: ProxyRouteCustomHeader[]) {
  return headers.map((header) => `${header.key}: ${header.value}`).join('\n');
}

export function validateLimitRate(value: string, t: TranslateFn) {
  const normalized = value.trim();
  if (!normalized || normalized === '0' || normalized === '-1') {
    return null;
  }
  if (!limitRatePattern.test(normalized)) {
    return t('validation.invalidLimitRate');
  }
  return null;
}

export function normalizeLimitRate(value: string) {
  const normalized = value.trim().toLowerCase();
  if (normalized === '0') return '';
  return normalized;
}

const limitReqPattern = /^\d+r\/[sm]$/i;

export function validateLimitReqPerIP(value: string, t: TranslateFn) {
  const normalized = value.trim();
  if (!normalized || normalized === '0' || normalized === '-1') {
    return null;
  }
  if (!limitReqPattern.test(normalized)) {
    return t('validation.invalidLimitReq');
  }
  return null;
}

export function normalizeLimitReqPerIP(value: string) {
  const normalized = value.trim().toLowerCase();
  if (!normalized || normalized === '0') return '';
  return normalized;
}

export function validateCacheRules(
  policy: 'static' | 'all' | 'url' | 'suffix' | 'path_prefix' | 'path_exact',
  rules: string[],
  t: TranslateFn,
) {
  if (policy === 'static' || policy === 'all' || policy === 'url') {
    return null;
  }

  if (rules.length === 0) {
    return t('validation.cacheRuleRequired');
  }

  if (policy === 'suffix') {
    for (const rule of rules) {
      const normalized = rule.replace(/^\./, '');
      if (!normalized || /[/\\\s]/.test(normalized)) {
        return t('validation.invalidCacheSuffix', { rule });
      }
    }
    return null;
  }

  for (const rule of rules) {
    if (!rule.startsWith('/') || rule.includes('://') || /[\s]/.test(rule)) {
      return t('validation.invalidCachePath', { rule });
    }
  }

  return null;
}

export function buildPayloadFromRoute(
  route: ProxyRouteItem,
  overrides: Partial<ProxyRouteMutationPayload>,
): ProxyRouteMutationPayload {
  const primaryOrigin =
    route.upstream_type === 'pages'
      ? parseOriginUrl('http://127.0.0.1')
      : parseOriginUrl(route.origin_url || 'http://127.0.0.1');

  return {
    site_name: route.site_name,
    zone_domain_ids: route.zone_domain_ids ?? [],
    origin_id: null,
    origin_url: route.origin_url,
    origin_scheme: primaryOrigin.scheme,
    origin_address: primaryOrigin.address,
    origin_port: primaryOrigin.port,
    origin_uri: primaryOrigin.uri,
    origin_host: route.origin_host || '',
    upstreams: (route.upstream_list ?? []).slice(1),
    enabled: route.enabled,
    enable_https: route.enable_https,
    redirect_http: route.redirect_http,
    limit_conn_per_server: route.limit_conn_per_server,
    limit_conn_per_ip: route.limit_conn_per_ip,
    limit_rate: route.limit_rate,
    limit_req_per_ip: route.limit_req_per_ip,
    cache_enabled: route.cache_enabled,
    cache_policy: (() => {
      if (!route.cache_enabled) {
        return 'static';
      }
      const policy = (route.cache_policy || '').trim();
      // Legacy empty/url → all (same as backend displayCachePolicy).
      if (!policy || policy === 'url' || policy === 'all') {
        return 'all';
      }
      return policy;
    })(),
    cache_rules: route.cache_rule_list ?? [],
    custom_headers: route.custom_header_list ?? [],
    basic_auth_enabled: route.basic_auth_enabled,
    basic_auth_username: route.basic_auth_username,
    basic_auth_password: route.basic_auth_password,
    upstream_type: route.upstream_type,
    tunnel_node_id: route.tunnel_node_id ?? route.tunnel_id ?? null,
    tunnel_target_addr: route.tunnel_target_addr || '',
    tunnel_target_protocol: route.tunnel_target_protocol || '',
    pages_project_id: route.pages_project_id ?? null,
    ...overrides,
  };
}

/** Load all Zone domains across registered Zones for route binding selectors. */
export async function listAllZoneDomains(): Promise<ZoneDomainItem[]> {
  const zones = await ZoneService.list();
  const overviews = await Promise.all(
    zones.map((zone) => ZoneService.getOverview(zone.id)),
  );
  return overviews.flatMap((overview) => overview.domains ?? []);
}
