import type {
  ProxyRouteConfigSection,
  ProxyRouteCustomHeader,
  ProxyRouteItem,
  ProxyRouteMutationPayload,
} from '@/lib/services/openflare';

export const proxyRouteConfigSections = [
  {
    key: 'domains' as const,
    label: '域名设置',
    description: '维护站点标识、域名列表和证书绑定。',
  },
  {
    key: 'limits' as const,
    label: '流量限制',
    description: '设置连接数和限速。',
  },
  {
    key: 'proxy' as const,
    label: '反向代理',
    description: '配置主回源和上游地址。',
  },
  {
    key: 'cache' as const,
    label: '缓存',
    description: '配置站点缓存策略。',
  },
  {
    key: 'waf' as const,
    label: 'WAF',
    description: '绑定 WAF 规则组，并查看当前站点生效策略。',
  },
  {
    key: 'auth' as const,
    label: '认证配置',
    description: '配置基础鉴权访问，需要输入账号密码才能访问网站。',
  },
] satisfies ReadonlyArray<{
  key: ProxyRouteConfigSection;
  label: string;
  description: string;
}>;

const domainPattern =
  /^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/i;

export function getProxyRouteConfigSection(
  value: string | null | undefined,
): ProxyRouteConfigSection {
  return proxyRouteConfigSections.some((section) => section.key === value)
    ? (value as ProxyRouteConfigSection)
    : 'domains';
}

export function validateDomain(domain: string): string | null {
  const normalized = domain.trim().toLowerCase();
  if (!normalized) {
    return '请输入域名';
  }
  if (!domainPattern.test(normalized)) {
    return '域名格式不合法';
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

export function getUpstreamSummary(route: ProxyRouteItem): string {
  if (route.upstream_type === 'pages') {
    return route.pages_project_id
      ? `Pages 项目 #${route.pages_project_id}`
      : 'Pages 项目未绑定';
  }
  if (route.upstream_type === 'tunnel') {
    const protocol = route.tunnel_target_protocol || 'http';
    const target = route.tunnel_target_addr || '未配置目标';
    return `Tunnel → ${protocol}://${target}`;
  }
  if (route.upstream_list.length <= 1) {
    return route.origin_url;
  }
  return `${route.upstream_list.length} 个上游，主上游 ${route.origin_url}`;
}

const originHostPattern =
  /^(?:(?:[a-z0-9-]+\.)*[a-z0-9-]+|\[[0-9a-f:.]+\]|[0-9.]+)(?::\d{1,5})?$/i;
const headerKeyPattern = /^[A-Za-z0-9_-]+$/;
const limitRatePattern = /^\d+(?:[kKmM])?$/;

export function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

export function linesFromTextarea(value: string) {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function validateDomains(domains: string[]) {
  if (domains.length === 0) {
    return '请至少填写一个域名';
  }

  const seen = new Set<string>();
  for (const domain of domains) {
    const normalized = domain.trim().toLowerCase();
    const error = validateDomain(normalized);
    if (error && normalized) {
      return `域名格式不合法：${domain}`;
    }
    if (!normalized) {
      continue;
    }
    if (seen.has(normalized)) {
      return `域名重复：${domain}`;
    }
    seen.add(normalized);
  }

  return null;
}

export function parseOriginUrls(value: string) {
  const urls = linesFromTextarea(value);
  if (urls.length === 0) {
    return { urls: [], error: '请至少填写一个上游地址' };
  }

  let sharedScheme = '';
  for (const originUrl of urls) {
    let parsed: URL;
    try {
      parsed = new URL(originUrl);
    } catch {
      return { urls: [], error: `上游地址格式不合法：${originUrl}` };
    }

    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      return {
        urls: [],
        error: `上游地址必须以 http:// 或 https:// 开头：${originUrl}`,
      };
    }

    if (!parsed.hostname) {
      return { urls: [], error: `上游地址缺少主机名：${originUrl}` };
    }

    if (urls.length > 1) {
      if ((parsed.pathname && parsed.pathname !== '/') || parsed.search) {
        return {
          urls: [],
          error: '多上游模式暂不支持带路径或查询参数的地址',
        };
      }

      if (!sharedScheme) {
        sharedScheme = parsed.protocol;
      } else if (sharedScheme !== parsed.protocol) {
        return {
          urls: [],
          error: '同一站点的多个上游必须使用相同协议',
        };
      }
    }
  }

  return { urls, error: null };
}

export function validateOriginHost(value: string) {
  const normalized = value.trim();
  if (!normalized) {
    return null;
  }
  if (
    normalized.includes('://') ||
    /[/\\\s]/.test(normalized) ||
    !originHostPattern.test(normalized)
  ) {
    return '回源 Host 格式不合法';
  }
  return null;
}

export function parseCustomHeadersText(value: string) {
  const lines = linesFromTextarea(value);
  const headers: ProxyRouteCustomHeader[] = [];

  for (const line of lines) {
    const separatorIndex = line.indexOf(':');
    if (separatorIndex <= 0) {
      return {
        headers: [],
        error: `自定义请求头格式不合法：${line}`,
      };
    }

    const key = line.slice(0, separatorIndex).trim();
    const headerValue = line.slice(separatorIndex + 1).trim();

    if (!headerKeyPattern.test(key)) {
      return {
        headers: [],
        error: `自定义请求头名称不合法：${key}`,
      };
    }

    headers.push({ key, value: headerValue });
  }

  return { headers, error: null };
}

export function customHeadersToText(headers: ProxyRouteCustomHeader[]) {
  return headers.map((header) => `${header.key}: ${header.value}`).join('\n');
}

export function validateLimitRate(value: string) {
  const normalized = value.trim();
  if (!normalized || normalized === '0') {
    return null;
  }
  if (!limitRatePattern.test(normalized)) {
    return '限速格式不合法，请使用 512k、1m 或纯数字';
  }
  return null;
}

export function normalizeLimitRate(value: string) {
  const normalized = value.trim().toLowerCase();
  return normalized === '0' ? '' : normalized;
}

export function validateCacheRules(
  policy: 'url' | 'suffix' | 'path_prefix' | 'path_exact',
  rules: string[],
) {
  if (policy === 'url') {
    return null;
  }

  if (rules.length === 0) {
    return '当前缓存策略至少需要一条规则';
  }

  if (policy === 'suffix') {
    for (const rule of rules) {
      const normalized = rule.replace(/^\./, '');
      if (!normalized || /[/\\\s]/.test(normalized)) {
        return `缓存后缀格式不合法：${rule}`;
      }
    }
    return null;
  }

  for (const rule of rules) {
    if (!rule.startsWith('/') || rule.includes('://') || /[\s]/.test(rule)) {
      return `缓存路径规则格式不合法：${rule}`;
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
      : parseOriginUrl(route.origin_url);

  return {
    site_name: route.site_name,
    domain: route.primary_domain,
    domains: route.domains,
    origin_id: null,
    origin_url: route.origin_url,
    origin_scheme: primaryOrigin.scheme,
    origin_address: primaryOrigin.address,
    origin_port: primaryOrigin.port,
    origin_uri: primaryOrigin.uri,
    origin_host: route.origin_host || '',
    upstreams: route.upstream_list.slice(1),
    enabled: route.enabled,
    enable_https: route.enable_https,
    cert_id: route.cert_id,
    cert_ids: route.cert_ids,
    domain_cert_ids: route.domain_cert_ids,
    redirect_http: route.redirect_http,
    limit_conn_per_server: route.limit_conn_per_server,
    limit_conn_per_ip: route.limit_conn_per_ip,
    limit_rate: route.limit_rate,
    cache_enabled: route.cache_enabled,
    cache_policy: route.cache_policy || 'url',
    cache_rules: route.cache_rule_list,
    custom_headers: route.custom_header_list,
    remark: route.remark || '',
    pow_enabled: route.pow_enabled,
    pow_config: JSON.stringify(route.pow_config),
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
