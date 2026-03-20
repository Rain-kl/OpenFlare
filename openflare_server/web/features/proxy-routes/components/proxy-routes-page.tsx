'use client';

import { Select, SelectItem, SelectSection } from '@heroui/select';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useFieldArray, useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { Separator } from "@/components/ui/separator"

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AppModal } from '@/components/ui/app-modal';
import { StatusBadge } from '@/components/ui/status-badge';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  getConfigVersionDiff,
  publishConfigVersion,
} from '@/features/config-versions/api/config-versions';
import { getManagedDomains } from '@/features/managed-domains/api/managed-domains';
import type { ManagedDomainItem } from '@/features/managed-domains/types';
import { getOrigins } from '@/features/origins/api/origins';
import type { OriginItem } from '@/features/origins/types';
import {
  createProxyRoute,
  deleteProxyRoute,
  getProxyRoutes,
  getTlsCertificates,
  matchManagedDomainCertificate,
  updateProxyRoute,
} from '@/features/proxy-routes/api/proxy-routes';
import type {
  ManagedDomainMatchResult,
  ProxyRouteCustomHeader,
  ProxyRouteItem,
  ProxyRouteMutationPayload,
  TlsCertificateItem,
} from '@/features/proxy-routes/types';
import {
  buildRouteDomain,
  findManagedDomainForRoute,
  isWildcardManagedDomain,
} from '@/features/proxy-routes/utils';
import {
  DangerButton,
  PrimaryButton,
  ResourceField,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
import { cn } from '@/lib/utils/cn';
import { formatDateTime } from '@/lib/utils/date';

const customHeaderSchema = z.object({
  key: z.string(),
  value: z.string(),
});

const originProtocolValues = ['http', 'https'] as const;
const cachePolicyValues = [
  'url',
  'suffix',
  'path_prefix',
  'path_exact',
] as const;

const originRowSchema = z.object({
  scheme: z.enum(originProtocolValues),
  address: z.string(),
  port: z.string(),
});

const proxyRouteSchema = z
  .object({
    managed_domain_id: z.string().trim().min(1, '请选择目标域名'),
    subdomain_label: z.string(),
    origin_rows: z.array(originRowSchema).min(1),
    origin_uri: z.string(),
    origin_host: z
      .string()
      .trim()
      .refine(
        (value) =>
          !value ||
          (!/[\/\\\s]/.test(value) &&
            !value.includes('://') &&
            (() => {
              try {
                const parsed = new URL(`http://${value}`);
                return parsed.host === value && Boolean(parsed.hostname);
              } catch {
                return false;
              }
            })()),
        '请输入合法的回源主机名',
      ),
    enabled: z.boolean(),
    enable_https: z.boolean(),
    cert_id: z.string(),
    redirect_http: z.boolean(),
    cache_enabled: z.boolean(),
    cache_policy: z.enum(cachePolicyValues),
    cache_rules_text: z.string(),
    custom_headers: z.array(customHeaderSchema).min(1),
    remark: z.string().max(255, '备注不能超过 255 个字符'),
  })
  .superRefine((value, context) => {
    const selectedManagedDomain = value.managed_domain_id.trim();
    const subdomainLabel = value.subdomain_label.trim();
    const isWildcard = selectedManagedDomain.startsWith('*.');

    if (isWildcard && !subdomainLabel) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['subdomain_label'],
        message: '请输入子域名前缀',
      });
    }

    if (
      isWildcard &&
      subdomainLabel &&
      !/^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$/.test(subdomainLabel)
    ) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['subdomain_label'],
        message: '子域名前缀仅支持单个标签，且只能包含字母、数字和中划线',
      });
    }

    if (value.enable_https && !value.cert_id) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['cert_id'],
        message: '启用 HTTPS 时必须选择证书',
      });
    }

    const normalizedOriginURI = value.origin_uri.trim();
    if (
      normalizedOriginURI &&
      !normalizedOriginURI.startsWith('/') &&
      !normalizedOriginURI.startsWith('?')
    ) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['origin_uri'],
        message: '回源路径需以 / 或 ? 开头',
      });
    }

    value.origin_rows.forEach((row, index) => {
      const normalizedAddress = row.address.trim();
      const normalizedPort = row.port.trim();

      if (!normalizedAddress) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['origin_rows', index, 'address'],
          message: '请输入源站地址',
        });
      }

      if (
        normalizedAddress &&
        (/[/?#]/.test(normalizedAddress) || normalizedAddress.includes('://'))
      ) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['origin_rows', index, 'address'],
          message: '源站地址仅支持 IP、域名或主机名',
        });
      }

      if (!normalizedPort) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['origin_rows', index, 'port'],
          message: '请输入端口',
        });
      }

      const portNumber = Number(normalizedPort);
      if (
        normalizedPort &&
        (!/^\d+$/.test(normalizedPort) ||
          !Number.isInteger(portNumber) ||
          portNumber < 1 ||
          portNumber > 65535)
      ) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['origin_rows', index, 'port'],
          message: '端口需为 1 到 65535 的整数',
        });
      }

      const originURL = buildOriginUrl(
        row.scheme,
        row.address,
        row.port,
        index === 0 ? value.origin_uri : '',
      );
      if (!originURL) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['origin_rows', index, 'address'],
          message: '请输入完整的源站信息',
        });
      }
    });

    if (value.cache_enabled) {
      const cacheRules = parseCacheRulesText(value.cache_rules_text);
      if (value.cache_policy !== 'url' && cacheRules.length === 0) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['cache_rules_text'],
          message: '当前缓存策略至少需要填写一条规则',
        });
      }
    }

    value.custom_headers.forEach((header, index) => {
      const key = header.key.trim();
      const headerValue = header.value.trim();

      if (!key && !headerValue) {
        return;
      }

      if (!key) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['custom_headers', index, 'key'],
          message: '请求头名称不能为空',
        });
      }

      if (key && !/^[A-Za-z0-9_-]+$/.test(key)) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['custom_headers', index, 'key'],
          message: '请求头名称仅支持字母、数字、下划线和中划线',
        });
      }

      if (/\r|\n/.test(key) || /\r|\n/.test(headerValue)) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['custom_headers', index, 'value'],
          message: '请求头不能包含换行',
        });
      }
    });
  });

type ProxyRouteFormValues = z.infer<typeof proxyRouteSchema>;
type OriginRowFormValue = ProxyRouteFormValues['origin_rows'][number];

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

type SectionIconProps = {
  className?: string;
};

const INPUT_CLASS_NAME = 'h-10 rounded-xl px-3 py-2 text-sm';
const PANEL_CLASS_NAME =
  'rounded-2xl border border-[var(--border-default)] bg-[color:color-mix(in_srgb,var(--surface-elevated)_82%,white_18%)] p-4 shadow-[var(--shadow-soft)]';
const HERO_SELECT_CLASS_NAMES = {
  base: 'w-full',
  trigger: cn(
    INPUT_CLASS_NAME,
    'min-h-10 rounded-xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-3 shadow-none data-[hover=true]:bg-[var(--surface-elevated)] data-[open=true]:border-[var(--border-strong)]',
  ),
  value: 'text-sm text-[var(--foreground-primary)]',
  popoverContent:
    'rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] shadow-[var(--shadow-soft)]',
  listboxWrapper: 'p-1',
} as const;

const defaultValues: ProxyRouteFormValues = {
  managed_domain_id: '',
  subdomain_label: '',
  origin_rows: [{ scheme: 'https', address: '', port: '443' }],
  origin_uri: '',
  origin_host: '',
  enabled: true,
  enable_https: false,
  cert_id: '',
  redirect_http: false,
  cache_enabled: false,
  cache_policy: 'url',
  cache_rules_text: '',
  custom_headers: [{ key: '', value: '' }],
  remark: '',
};

const routesQueryKey = ['proxy-routes'];
const certificatesQueryKey = ['tls-certificates'];
const managedDomainsQueryKey = ['managed-domains'];
const originsQueryKey = ['origins'];
const versionsQueryKey = ['config-versions'];

function GlobeIcon({ className }: SectionIconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      className={className}
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18" />
      <path d="M12 3c3 3.2 4.5 6.2 4.5 9s-1.5 5.8-4.5 9c-3-3.2-4.5-6.2-4.5-9S9 6.2 12 3Z" />
    </svg>
  );
}

function ServerIcon({ className }: SectionIconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      className={className}
      aria-hidden="true"
    >
      <rect x="4" y="4" width="16" height="6" rx="1.5" />
      <rect x="4" y="14" width="16" height="6" rx="1.5" />
      <path d="M8 7h.01M8 17h.01M12 7h6M12 17h6" />
    </svg>
  );
}

function ShieldIcon({ className }: SectionIconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      className={className}
      aria-hidden="true"
    >
      <path d="M12 3 5 6v5c0 4.7 2.8 8.9 7 10 4.2-1.1 7-5.3 7-10V6l-7-3Z" />
      <path d="m9.5 12 1.8 1.8 3.7-4" />
    </svg>
  );
}

function SlidersIcon({ className }: SectionIconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      className={className}
      aria-hidden="true"
    >
      <path d="M5 6h14M5 18h14M8 6v12M16 6v12" />
      <circle cx="8" cy="10" r="2" />
      <circle cx="16" cy="14" r="2" />
    </svg>
  );
}

function SearchIcon({ className }: SectionIconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      className={className}
      aria-hidden="true"
    >
      <circle cx="11" cy="11" r="6" />
      <path d="m20 20-4.2-4.2" />
    </svg>
  );
}

function PlusIcon({ className }: SectionIconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      className={className}
      aria-hidden="true"
    >
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

function TrashIcon({ className }: SectionIconProps) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      className={className}
      aria-hidden="true"
    >
      <path d="M4 7h16M9 7V5h6v2M8 10v7M12 10v7M16 10v7M6 7l1 12h10l1-12" />
    </svg>
  );
}

function ChevronIcon({
  className,
  expanded = false,
}: SectionIconProps & { expanded?: boolean }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      className={cn(
        'transition-transform duration-200',
        expanded ? 'rotate-180' : '',
        className,
      )}
      aria-hidden="true"
    >
      <path d="m6 9 6 6 6-6" />
    </svg>
  );
}

function hasConfigChanges(diff: {
  active_version?: string;
  added_domains: string[];
  removed_domains: string[];
  modified_domains: string[];
  main_config_changed: boolean;
  changed_option_keys: string[];
}) {
  return (
    diff.added_domains.length > 0 ||
    diff.removed_domains.length > 0 ||
    diff.modified_domains.length > 0 ||
    diff.main_config_changed ||
    diff.changed_option_keys.length > 0 ||
    !diff.active_version
  );
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function parseCustomHeaders(rawValue: string) {
  if (!rawValue) {
    return [] as ProxyRouteCustomHeader[];
  }

  try {
    const parsed = JSON.parse(rawValue) as ProxyRouteCustomHeader[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function parseCacheRules(rawValue: string) {
  if (!rawValue) {
    return [] as string[];
  }

  try {
    const parsed = JSON.parse(rawValue) as string[];
    return Array.isArray(parsed) ? parsed.filter(Boolean) : [];
  } catch {
    return [];
  }
}

function parseCacheRulesText(value: string) {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseUpstreams(rawValue: string) {
  if (!rawValue) {
    return [] as string[];
  }

  try {
    const parsed = JSON.parse(rawValue) as string[];
    return Array.isArray(parsed) ? parsed.filter(Boolean) : [];
  } catch {
    return [];
  }
}

function buildCachePolicyLabel(policy: string) {
  switch (policy) {
    case 'suffix':
      return '按后缀';
    case 'path_prefix':
      return '按前缀';
    case 'path_exact':
      return '按路径';
    default:
      return '按 URL';
  }
}

function getCacheRulesHint(policy: string) {
  switch (policy) {
    case 'suffix':
      return '每行一个后缀，例如：jpg、css、js。';
    case 'path_prefix':
      return '每行一个路径前缀，例如：/assets、/static/images。';
    case 'path_exact':
      return '每行一个精确路径，例如：/robots.txt、/manifest.json。';
    default:
      return '按 URL 缓存时无需额外规则，系统会按请求 URL 粒度缓存。';
  }
}

function buildCertificateLabel(certificate: TlsCertificateItem) {
  return certificate.not_after
    ? `${certificate.name}（到期：${formatDateTime(certificate.not_after)}）`
    : certificate.name;
}

function buildOriginUrl(
  scheme: 'http' | 'https',
  address: string,
  port: string,
  uri: string,
) {
  const normalizedAddress = address.trim();
  const normalizedPort = port.trim();
  if (!normalizedAddress || !normalizedPort) {
    return '';
  }

  const host =
    normalizedAddress.includes(':') && !normalizedAddress.startsWith('[')
      ? `[${normalizedAddress}]`
      : normalizedAddress;
  const normalizedURI = uri.trim();

  return `${scheme}://${host}:${normalizedPort}${normalizedURI}`;
}

function parseOriginUrl(rawValue: string) {
  try {
    const parsed = new URL(rawValue);
    const uri = parsed.pathname === '/' ? '' : parsed.pathname;
    return {
      scheme: (parsed.protocol.replace(':', '') || 'https') as 'http' | 'https',
      address: parsed.hostname,
      port: parsed.port || (parsed.protocol === 'http:' ? '80' : '443'),
      uri: parsed.search ? `${uri}${parsed.search}` || parsed.search : uri,
    };
  } catch {
    return {
      scheme: 'https' as const,
      address: '',
      port: '443',
      uri: '',
    };
  }
}

function getDefaultPortForScheme(scheme: 'http' | 'https') {
  return scheme === 'http' ? '80' : '443';
}

function toPayload(
  values: ProxyRouteFormValues,
  origins: OriginItem[],
): ProxyRouteMutationPayload {
  const primaryOrigin = values.origin_rows[0];
  const primaryOriginUrl = buildOriginUrl(
    primaryOrigin.scheme,
    primaryOrigin.address,
    primaryOrigin.port,
    values.origin_uri,
  );
  const primaryOriginRecord =
    origins.find(
      (item) =>
        item.address.toLowerCase() === primaryOrigin.address.trim().toLowerCase(),
    ) ?? null;

  return {
    domain: buildRouteDomain(values.managed_domain_id, values.subdomain_label),
    origin_id: primaryOriginRecord ? primaryOriginRecord.id : null,
    origin_url: primaryOriginUrl,
    origin_scheme: primaryOrigin.scheme,
    origin_address: primaryOrigin.address.trim(),
    origin_port: primaryOrigin.port.trim(),
    origin_uri: values.origin_uri.trim(),
    origin_host: values.origin_host.trim(),
    upstreams: values.origin_rows
      .slice(1)
      .map((row) => buildOriginUrl(row.scheme, row.address, row.port, ''))
      .filter(Boolean),
    enabled: values.enabled,
    enable_https: values.enable_https,
    cert_id:
      values.enable_https && values.cert_id ? Number(values.cert_id) : null,
    redirect_http: values.enable_https ? values.redirect_http : false,
    cache_enabled: values.cache_enabled,
    cache_policy: values.cache_enabled ? values.cache_policy : 'url',
    cache_rules: values.cache_enabled
      ? parseCacheRulesText(values.cache_rules_text)
      : [],
    custom_headers: values.custom_headers
      .map((item) => ({ key: item.key.trim(), value: item.value.trim() }))
      .filter((item) => item.key || item.value),
    remark: values.remark.trim(),
  };
}

function toFormValues(
  route: ProxyRouteItem,
  managedDomains: ManagedDomainItem[],
): ProxyRouteFormValues {
  const headers = parseCustomHeaders(route.custom_headers);
  const cacheRules = parseCacheRules(route.cache_rules);
  const upstreams = parseUpstreams(route.upstreams);
  const managedDomainMatch = findManagedDomainForRoute(
    route.domain,
    managedDomains,
  );

  if (!managedDomainMatch) {
    throw new Error(
      `规则 ${route.domain} 未匹配到网站，请先补充对应网站后再编辑。`,
    );
  }

  const primaryOrigin = parseOriginUrl(route.origin_url);
  const normalizedOrigins = Array.from(
    new Set([route.origin_url, ...upstreams].filter(Boolean)),
  );
  const originRows: OriginRowFormValue[] = normalizedOrigins.map((origin) => {
    const parsed = parseOriginUrl(origin);
    return {
      scheme: parsed.scheme,
      address: parsed.address,
      port: parsed.port,
    };
  });

  return {
    managed_domain_id: managedDomainMatch.managedDomainId,
    subdomain_label: managedDomainMatch.subdomainLabel,
    origin_rows:
      originRows.length > 0
        ? originRows
        : [{ scheme: 'https', address: '', port: '443' }],
    origin_uri: primaryOrigin.uri,
    origin_host: route.origin_host || '',
    enabled: route.enabled,
    enable_https: route.enable_https,
    cert_id: route.cert_id ? String(route.cert_id) : '',
    redirect_http: route.redirect_http,
    cache_enabled: route.cache_enabled,
    cache_policy: (route.cache_policy ||
      'url') as ProxyRouteFormValues['cache_policy'],
    cache_rules_text: cacheRules.join('\n'),
    custom_headers: headers.length > 0 ? headers : [{ key: '', value: '' }],
    remark: route.remark || '',
  };
}

function getMatchMessage(
  matchResult: ManagedDomainMatchResult | null,
  isMatching: boolean,
  domain: string,
  enabled: boolean,
) {
  if (!enabled) {
    return '若目标域名已绑定证书，HTTPS 会默认开启；你也可以稍后手动开启。';
  }

  if (isMatching) {
    return '正在按域名自动匹配托管证书...';
  }

  if (!domain.trim()) {
    return '输入完整域名后会自动匹配证书，并优先推荐精确匹配规则。';
  }

  if (matchResult?.matched && matchResult.candidate) {
    return `已匹配${matchResult.candidate.match_type === 'exact' ? '精确' : '通配符'}证书规则 ${matchResult.candidate.domain}，默认推荐 ${matchResult.candidate.certificate_name}`;
  }

  return '未找到匹配证书，可继续手动选择。';
}

function isLocalOriginAddress(address: string) {
  const normalized = address.trim().toLowerCase();
  return (
    normalized === 'localhost' ||
    normalized.endsWith('.local') ||
    normalized.endsWith('.internal') ||
    /^10\./.test(normalized) ||
    /^192\.168\./.test(normalized) ||
    /^172\.(1[6-9]|2\d|3[0-1])\./.test(normalized) ||
    normalized === '::1' ||
    normalized.startsWith('fc') ||
    normalized.startsWith('fd')
  );
}

function ProxyRuleSection({
  title,
  icon,
  action,
  children,
}: {
  title: string;
  icon: React.ReactNode;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3 text-[var(--foreground-primary)]">
          <span className="flex h-8 w-8 items-center justify-center rounded-full border border-[var(--border-default)] bg-[var(--surface-elevated)] text-[var(--foreground-secondary)]">
            {icon}
          </span>
          <h3 className="text-sm font-semibold uppercase tracking-[0.16em]">
            {title}
          </h3>
        </div>
        {action}
      </div>
      <div className={PANEL_CLASS_NAME}>{children}</div>
    </section>
  );
}

type SearchOption = {
  key: string;
  label: string;
  keywords?: string[];
  selected?: boolean;
  content?: React.ReactNode;
  trailing?: React.ReactNode;
};

function SearchableSelectInput({
  label,
  hint,
  error,
  showFieldMeta = true,
  value,
  placeholder,
  emptyText,
  options,
  onValueChange,
  onSelect,
}: {
  label: string;
  hint?: string;
  error?: string;
  showFieldMeta?: boolean;
  value: string;
  placeholder: string;
  emptyText: string;
  options: SearchOption[];
  onValueChange: (value: string) => void;
  onSelect: (option: SearchOption) => void;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const containerRef = useRef<HTMLDivElement | null>(null);

  const filteredOptions = useMemo(() => {
    const keyword = value.trim().toLowerCase();
    if (!keyword) {
      return options;
    }

    return options.filter((option) => {
      const haystacks = [option.label, ...(option.keywords ?? [])];
      return haystacks.some((item) => item.toLowerCase().includes(keyword));
    });
  }, [options, value]);

  const highlightedOption =
    filteredOptions[
      Math.min(highlightedIndex, Math.max(filteredOptions.length - 1, 0))
    ] ?? null;

  useEffect(() => {
    setHighlightedIndex(0);
  }, [value]);

  useEffect(() => {
    if (highlightedIndex > filteredOptions.length - 1) {
      setHighlightedIndex(0);
    }
  }, [filteredOptions.length, highlightedIndex]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handlePointerDown);
    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
    };
  }, [isOpen]);

  const content = (
    <div ref={containerRef} className="relative">
      <div className="relative">
        <SearchIcon className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-[var(--foreground-muted)]" />
        <input
          value={value}
          placeholder={placeholder}
          className={cn(
            'w-full rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-3 text-sm text-[var(--foreground-primary)] outline-none transition placeholder:text-[var(--foreground-muted)] focus:border-[var(--border-strong)] focus:ring-2 focus:ring-[var(--accent-soft)]',
            INPUT_CLASS_NAME,
            'pl-9 pr-10',
          )}
          onFocus={() => setIsOpen(true)}
          onChange={(event) => {
            onValueChange(event.target.value);
            setIsOpen(true);
          }}
          onKeyDown={(event) => {
            if (event.key === 'ArrowDown' && filteredOptions.length > 0) {
              event.preventDefault();
              setIsOpen(true);
              setHighlightedIndex((current) =>
                current >= filteredOptions.length - 1 ? 0 : current + 1,
              );
              return;
            }

            if (event.key === 'ArrowUp' && filteredOptions.length > 0) {
              event.preventDefault();
              setIsOpen(true);
              setHighlightedIndex((current) =>
                current <= 0 ? filteredOptions.length - 1 : current - 1,
              );
              return;
            }

            if (event.key === 'Enter' && highlightedOption) {
              event.preventDefault();
              onSelect(highlightedOption);
              setIsOpen(false);
              setHighlightedIndex(0);
              return;
            }

            if (event.key === 'Escape') {
              setIsOpen(false);
            }
          }}
        />
        <button
          type="button"
          onClick={() => setIsOpen((current) => !current)}
          aria-label="切换候选列表"
          className="absolute right-1.5 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-lg text-[var(--foreground-muted)] transition hover:bg-[var(--control-background-hover)]"
        >
          <ChevronIcon
            expanded={isOpen}
            className="h-4 w-4 text-[var(--foreground-muted)]"
          />
        </button>
      </div>

      {isOpen ? (
        <div className="absolute inset-x-0 top-[calc(100%+8px)] z-20 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] p-3 shadow-[var(--shadow-soft)]">
          <div className="mb-2 flex items-center justify-between gap-3 px-1 text-xs text-[var(--foreground-secondary)]">
            <span>
              {highlightedOption
                ? `按 Enter 选择 ${highlightedOption.label}`
                : '继续输入以匹配候选项'}
            </span>
            <span>{`${filteredOptions.length} 个结果`}</span>
          </div>
          <div className="max-h-56 space-y-1 overflow-y-auto">
            {filteredOptions.length > 0 ? (
              filteredOptions.map((option, index) => (
                  <button
                    key={option.key}
                  type="button"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => {
                    onSelect(option);
                    setIsOpen(false);
                    setHighlightedIndex(0);
                  }}
                  className={cn(
                    'flex w-full items-center justify-between rounded-xl px-3 py-2 text-left text-sm transition hover:bg-[var(--control-background-hover)]',
                    option.selected
                      ? 'bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                      : index === highlightedIndex
                        ? 'bg-[var(--control-background)] text-[var(--foreground-primary)]'
                        : 'text-[var(--foreground-secondary)]',
                  )}
                  >
                    <span>{option.content ?? option.label}</span>
                    <div className="flex items-center gap-2">
                    {index === highlightedIndex ? (
                      <span className="rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--foreground-muted)]">
                        Enter
                      </span>
                    ) : null}
                    {option.trailing}
                  </div>
                </button>
              ))
            ) : (
              <p className="rounded-xl border border-dashed border-[var(--border-default)] px-3 py-4 text-sm text-[var(--foreground-secondary)]">
                {emptyText}
              </p>
            )}
          </div>
        </div>
      ) : null}
    </div>
  );

  if (!showFieldMeta) {
    return content;
  }

  return (
    <ResourceField label={label} hint={hint} error={error}>
      {content}
    </ResourceField>
  );
}

function SearchableManagedDomainField({
  value,
  domains,
  error,
  onSelect,
}: {
  value: string;
  domains: ManagedDomainItem[];
  error?: string;
  onSelect: (value: string) => void;
}) {
  const [inputValue, setInputValue] = useState('');

  useEffect(() => {
    if (value) {
      setInputValue(value);
      return;
    }

    setInputValue('');
  }, [value]);

  return (
    <SearchableSelectInput
      label="Select Target Domain"
      hint="直接输入域名关键字进行本地模糊匹配，按 Enter 选择当前匹配项。"
      error={error}
      value={inputValue}
      placeholder="输入并搜索目标域名"
      emptyText="未发现匹配域名"
      options={domains.map((domain) => ({
        key: String(domain.id),
        label: domain.domain,
        keywords: [domain.domain],
        selected: value === domain.domain,
        trailing: domain.cert_id ? (
          <span className="rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--foreground-muted)]">
            TLS
          </span>
        ) : null,
      }))}
      onValueChange={setInputValue}
      onSelect={(option) => {
        onSelect(option.label);
        setInputValue(option.label);
      }}
    />
  );
}

function SearchableOriginAddressField({
  value,
  origins,
  onValueChange,
}: {
  value: string;
  origins: OriginItem[];
  onValueChange: (value: string) => void;
}) {
  return (
    <SearchableSelectInput
      label="源站地址"
      showFieldMeta={false}
      value={value}
      placeholder="192.168.1.45"
      emptyText="未发现匹配资产，请手动输入"
      options={origins.map((origin) => ({
        key: String(origin.id),
        label: origin.address,
        keywords: [origin.address, origin.name],
        selected: value.trim().toLowerCase() === origin.address.toLowerCase(),
        content: (
          <span className="text-[var(--foreground-primary)]">
            {origin.address}
            {origin.name ? (
              <span className="ml-1 text-[var(--foreground-muted)]">
                ({origin.name})
              </span>
            ) : null}
          </span>
        ),
        trailing: isLocalOriginAddress(origin.address) ? (
          <span className="rounded-md bg-[var(--accent-soft)] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--foreground-secondary)]">
            Local
          </span>
        ) : null,
      }))}
      onValueChange={onValueChange}
      onSelect={(option) => onValueChange(option.label)}
    />
  );
}

function HeroTextInput(
  props: React.ComponentProps<typeof Input> & { className?: string },
) {
  const { className, ...restProps } = props;

  return (
    <Input
      className={cn(INPUT_CLASS_NAME, className)}
      {...restProps}
    />
  );
}

function HeroTextArea(
  props: React.ComponentProps<typeof Textarea> & { className?: string },
) {
  const { className, ...restProps } = props;

  return (
    <Textarea
      className={className}
      {...restProps}
    />
  );
}

function getSelectedKeys(value: string) {
  return value ? new Set([value]) : new Set<string>();
}

function HeroSelectField({
  ariaLabel,
  value,
  disabled,
  onChange,
  children,
}: {
  ariaLabel: string;
  value: string;
  disabled?: boolean;
  onChange: (value: string) => void;
  children: React.ComponentProps<typeof Select>['children'];
}) {
  return (
    <Select
      aria-label={ariaLabel}
      variant="bordered"
      radius="lg"
      disallowEmptySelection
      selectedKeys={getSelectedKeys(value)}
      isDisabled={disabled}
      classNames={HERO_SELECT_CLASS_NAMES}
      onSelectionChange={(keys) => {
        if (keys === 'all') {
          return;
        }

        const [nextValue] = Array.from(keys);
        if (typeof nextValue === 'string') {
          onChange(nextValue);
        }
      }}
    >
      {children}
    </Select>
  );
}

function HeroSwitchField({
  id,
  label,
  description,
  checked,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  description?: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex items-start justify-between gap-4 rounded-2xl border border-[var(--border-default)] bg-[color:color-mix(in_srgb,var(--surface-elevated)_82%,white_18%)] px-4 py-4">
      <div className="space-y-1">
        <Label htmlFor={id}>{label}</Label>
        {description ? (
          <p className="text-sm leading-5 text-[var(--foreground-secondary)]">
            {description}
          </p>
        ) : null}
      </div>
      <Switch
        id={id}
        checked={checked}
        disabled={disabled}
        aria-label={label}
        onCheckedChange={onChange}
      />
    </div>
  );
}

export function ProxyRoutesPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [editingRouteId, setEditingRouteId] = useState<number | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [matchResult, setMatchResult] =
    useState<ManagedDomainMatchResult | null>(null);
  const [isMatching, setIsMatching] = useState(false);
  const [isAdvancedOpen, setIsAdvancedOpen] = useState(true);

  const form = useForm<ProxyRouteFormValues>({
    resolver: zodResolver(proxyRouteSchema),
    defaultValues,
  });

  const {
    fields: originFields,
    append: appendOrigin,
    remove: removeOrigin,
    replace: replaceOrigins,
  } = useFieldArray({
    control: form.control,
    name: 'origin_rows',
  });

  const {
    fields: headerFields,
    append: appendHeader,
    remove: removeHeader,
    replace: replaceHeaders,
  } = useFieldArray({
    control: form.control,
    name: 'custom_headers',
  });

  const watchedManagedDomain = useWatch({
    control: form.control,
    name: 'managed_domain_id',
  });
  const watchedSubdomainLabel = useWatch({
    control: form.control,
    name: 'subdomain_label',
  });
  const watchedOriginRows = useWatch({
    control: form.control,
    name: 'origin_rows',
  });
  const watchedOriginURI = useWatch({
    control: form.control,
    name: 'origin_uri',
  });
  const watchedEnabled = useWatch({ control: form.control, name: 'enabled' });
  const watchedEnableHttps = useWatch({
    control: form.control,
    name: 'enable_https',
  });
  const watchedRedirectHttp = useWatch({
    control: form.control,
    name: 'redirect_http',
  });
  const watchedCacheEnabled = useWatch({
    control: form.control,
    name: 'cache_enabled',
  });
  const watchedCachePolicy = useWatch({
    control: form.control,
    name: 'cache_policy',
  });
  const watchedCertId = useWatch({ control: form.control, name: 'cert_id' });

  const routesQuery = useQuery({
    queryKey: routesQueryKey,
    queryFn: getProxyRoutes,
  });

  const certificatesQuery = useQuery({
    queryKey: certificatesQueryKey,
    queryFn: getTlsCertificates,
  });

  const managedDomainsQuery = useQuery({
    queryKey: managedDomainsQueryKey,
    queryFn: getManagedDomains,
  });

  const originsQuery = useQuery({
    queryKey: originsQueryKey,
    queryFn: getOrigins,
  });

  const managedDomains = useMemo(
    () => managedDomainsQuery.data ?? [],
    [managedDomainsQuery.data],
  );
  const origins = useMemo(() => originsQuery.data ?? [], [originsQuery.data]);
  const certificates = useMemo(
    () => certificatesQuery.data ?? [],
    [certificatesQuery.data],
  );

  const selectedManagedDomain = useMemo(
    () =>
      managedDomains.find((item) => item.domain === watchedManagedDomain) ??
      null,
    [managedDomains, watchedManagedDomain],
  );

  const selectedManagedDomainValue =
    selectedManagedDomain?.domain ?? watchedManagedDomain;
  const isWildcardSelection = isWildcardManagedDomain(
    selectedManagedDomainValue,
  );
  const effectiveDomain = buildRouteDomain(
    selectedManagedDomainValue,
    watchedSubdomainLabel,
  );

  const primaryOriginRow =
    watchedOriginRows?.[0] ?? defaultValues.origin_rows[0];
  const saveMutation = useMutation({
    mutationFn: async (values: ProxyRouteFormValues) => {
      const payload = toPayload(values, origins);
      return editingRouteId
        ? updateProxyRoute(editingRouteId, payload)
        : createProxyRoute(payload);
    },
    onSuccess: async () => {
      setFeedback({
        tone: 'success',
        message: editingRouteId ? '规则已更新。' : '规则已创建。',
      });
      setEditingRouteId(null);
      setIsEditorOpen(false);
      setMatchResult(null);
      setIsAdvancedOpen(false);
      form.reset(defaultValues);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: routesQueryKey }),
        queryClient.invalidateQueries({ queryKey: managedDomainsQueryKey }),
        queryClient.invalidateQueries({ queryKey: originsQueryKey }),
      ]);
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteProxyRoute,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '规则已删除。' });
      await queryClient.invalidateQueries({ queryKey: routesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const publishMutation = useMutation({
    mutationFn: publishConfigVersion,
    onSuccess: async (version) => {
      setFeedback({
        tone: 'success',
        message: `发布成功，版本 ${version.version}`,
      });
      await queryClient.invalidateQueries({ queryKey: versionsQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  useEffect(() => {
    if (!watchedEnableHttps) {
      setMatchResult(null);
      setIsMatching(false);
      return;
    }

    const normalizedDomain = effectiveDomain.trim().toLowerCase();
    if (!normalizedDomain) {
      setMatchResult(null);
      return;
    }

    let cancelled = false;
    const timer = window.setTimeout(async () => {
      try {
        setIsMatching(true);
        const result = await matchManagedDomainCertificate(normalizedDomain);
        if (cancelled) {
          return;
        }

        setMatchResult(result);
        if (result.candidate?.certificate_id && !form.getValues('cert_id')) {
          form.setValue('cert_id', String(result.candidate.certificate_id), {
            shouldDirty: true,
            shouldValidate: true,
          });
        }
      } catch (error) {
        if (!cancelled) {
          setMatchResult(null);
          setFeedback({ tone: 'danger', message: getErrorMessage(error) });
        }
      } finally {
        if (!cancelled) {
          setIsMatching(false);
        }
      }
    }, 400);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [effectiveDomain, form, watchedEnableHttps]);

  const handlePublish = async () => {
    setFeedback(null);

    try {
      const diff = await getConfigVersionDiff();
      if (!hasConfigChanges(diff)) {
        setFeedback({
          tone: 'info',
          message: '当前规则没有变更，已阻止重复发布。',
        });
        return;
      }

      publishMutation.mutate();
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    }
  };

  const handleReset = () => {
    setFeedback(null);
    setEditingRouteId(null);
    setIsEditorOpen(false);
    setMatchResult(null);
    setIsAdvancedOpen(false);
    form.reset(defaultValues);
  };

  const handleCreate = () => {
    setFeedback(null);
    setEditingRouteId(null);
    setMatchResult(null);
    setIsAdvancedOpen(false);
    form.reset(defaultValues);
    setIsEditorOpen(true);
  };

  const handleSubmit = form.handleSubmit((values) => {
    setFeedback(null);
    saveMutation.mutate(values);
  });

  const handleEdit = (route: ProxyRouteItem) => {
    setFeedback(null);
    setEditingRouteId(route.id);
    setMatchResult(null);
    try {
      form.reset(toFormValues(route, managedDomains));
      setIsAdvancedOpen(Boolean(route.origin_host || route.remark));
      setIsEditorOpen(true);
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    }
  };

  const handleDelete = (route: ProxyRouteItem) => {
    if (!window.confirm(`确认删除规则 ${route.domain} 吗？`)) {
      return;
    }

    setFeedback(null);
    deleteMutation.mutate(route.id);
  };

  const handleRemoveHeader = (index: number) => {
    if (headerFields.length === 1) {
      replaceHeaders([{ key: '', value: '' }]);
      return;
    }

    removeHeader(index);
  };

  const handleManagedDomainSelect = (domainValue: string) => {
    const domain = managedDomains.find((item) => item.domain === domainValue);

    form.setValue('managed_domain_id', domainValue, {
      shouldDirty: true,
      shouldValidate: true,
    });

    if (!isWildcardManagedDomain(domainValue)) {
      form.setValue('subdomain_label', '', {
        shouldDirty: true,
        shouldValidate: true,
      });
    }

    const currentCertId = form.getValues('cert_id');
    const autoCertId = domain?.cert_id ? String(domain.cert_id) : '';
    if (autoCertId) {
      form.setValue('enable_https', true, {
        shouldDirty: true,
        shouldValidate: true,
      });
      if (!currentCertId) {
        form.setValue('cert_id', autoCertId, {
          shouldDirty: true,
          shouldValidate: true,
        });
      }
    }
  };

  const routes = routesQuery.data || [];

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="反代规则"
          description="维护域名到源站的映射、HTTPS 证书绑定与自定义请求头，并可直接触发配置发布。"
          action={
            <>
              <PrimaryButton
                type="button"
                onClick={() => void handlePublish()}
                disabled={publishMutation.isPending}
              >
                {publishMutation.isPending ? '发布中...' : '发布当前规则'}
              </PrimaryButton>
              <SecondaryButton type="button" onClick={handleCreate}>
                新增规则
              </SecondaryButton>
            </>
          }
        />

        {feedback ? (
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        <AppCard title="规则列表">
          {routesQuery.isLoading ? (
            <LoadingState />
          ) : routesQuery.isError ? (
            <ErrorState
              title="规则列表加载失败"
              description={getErrorMessage(routesQuery.error)}
            />
          ) : routes.length === 0 ? (
            <EmptyState
              title="暂无反代规则"
              description="请先创建至少一条规则，然后再进行发布。"
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                <thead>
                  <tr className="text-[var(--foreground-secondary)]">
                    <th className="px-3 py-3 font-medium">域名</th>
                    <th className="px-3 py-3 font-medium">源站地址</th>
                    <th className="px-3 py-3 font-medium">HTTPS</th>
                    <th className="px-3 py-3 font-medium">缓存</th>
                    <th className="px-3 py-3 font-medium">请求头</th>
                    <th className="px-3 py-3 font-medium">状态</th>
                    <th className="px-3 py-3 font-medium">备注</th>
                    <th className="px-3 py-3 font-medium">更新时间</th>
                    <th className="px-3 py-3 font-medium">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-default)]">
                  {routes.map((route) => {
                    const headers = parseCustomHeaders(route.custom_headers);
                    const cacheRules = parseCacheRules(route.cache_rules);
                    const upstreams = parseUpstreams(route.upstreams);

                    return (
                      <tr key={route.id} className="align-top">
                        <td className="px-3 py-4 font-medium text-[var(--foreground-primary)]">
                          {route.domain}
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          <div className="space-y-1">
                            <p>{route.origin_url}</p>
                            <p className="text-xs text-[var(--foreground-muted)]">
                              回源主机名: {route.origin_host || '$host'}
                            </p>
                            <p className="text-xs text-[var(--foreground-muted)]">
                              上游数量: {Math.max(upstreams.length, 1)}
                            </p>
                          </div>
                        </td>
                        <td className="px-3 py-4">
                          {route.enable_https ? (
                            <div className="space-y-2">
                              <StatusBadge
                                label={
                                  route.redirect_http
                                    ? 'HTTPS + 重定向'
                                    : 'HTTPS'
                                }
                                variant="info"
                              />
                            </div>
                          ) : (
                            <StatusBadge label="HTTP" variant="warning" />
                          )}
                        </td>
                        <td className="px-3 py-4">
                          {route.cache_enabled ? (
                            <div className="space-y-2">
                              <StatusBadge
                                label={buildCachePolicyLabel(
                                  route.cache_policy,
                                )}
                                variant="success"
                              />
                              <p className="text-xs text-[var(--foreground-muted)]">
                                {cacheRules.length > 0
                                  ? `${cacheRules.length} 条规则`
                                  : '按 URL 粒度缓存'}
                              </p>
                            </div>
                          ) : (
                            <StatusBadge label="关闭" variant="warning" />
                          )}
                        </td>
                        <td className="px-3 py-4">
                          <StatusBadge
                            label={
                              headers.length > 0 ? `${headers.length} 条` : '无'
                            }
                            variant={headers.length > 0 ? 'success' : 'warning'}
                          />
                        </td>
                        <td className="px-3 py-4">
                          <StatusBadge
                            label={route.enabled ? '启用' : '停用'}
                            variant={route.enabled ? 'success' : 'warning'}
                          />
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {route.remark || '—'}
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {formatDateTime(route.updated_at)}
                        </td>
                        <td className="px-3 py-4">
                          <div className="flex flex-wrap gap-2">
                            <SecondaryButton
                              type="button"
                              onClick={() => handleEdit(route)}
                              className="px-3 py-2 text-xs"
                            >
                              编辑
                            </SecondaryButton>
                            <DangerButton
                              type="button"
                              onClick={() => handleDelete(route)}
                              disabled={deleteMutation.isPending}
                              className="px-3 py-2 text-xs"
                            >
                              删除
                            </DangerButton>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </AppCard>
      </div>

      <AppModal
        isOpen={isEditorOpen}
        onClose={handleReset}
        title={editingRouteId ? '编辑规则' : '新增规则'}
        description="新增或修改反代规则后，可直接回到列表页继续发布。"
        size="xl"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <SecondaryButton
              type="button"
              onClick={handleReset}
              disabled={saveMutation.isPending}
            >
              取消
            </SecondaryButton>
            <PrimaryButton
              type="submit"
              form="proxy-route-editor-form"
              disabled={saveMutation.isPending}
            >
              {saveMutation.isPending
                ? '保存中...'
                : editingRouteId
                  ? '保存修改'
                  : '新增规则'}
            </PrimaryButton>
          </div>
        }
      >
        <form
          id="proxy-route-editor-form"
          className="space-y-6"
          onSubmit={handleSubmit}
        >
          <HeroSwitchField
            id="proxy-route-enabled"
            label="启用规则"
            description="关闭后该规则不会参与配置渲染与发布。"
            checked={watchedEnabled}
            onChange={(checked) =>
              form.setValue('enabled', checked, { shouldDirty: true })
            }
          />

          <ProxyRuleSection
            title="Domain"
            icon={<GlobeIcon className="h-4 w-4" />}
          >
            <div className="space-y-4">
              <div
                className={cn(
                  'grid gap-4',
                  selectedManagedDomain && isWildcardSelection
                    ? 'md:grid-cols-2'
                    : 'md:grid-cols-1',
                )}
              >
                <SearchableManagedDomainField
                  value={watchedManagedDomain}
                  domains={managedDomains}
                  error={form.formState.errors.managed_domain_id?.message}
                  onSelect={handleManagedDomainSelect}
                />

                {selectedManagedDomain && isWildcardSelection ? (
                  <ResourceField
                    label="Subdomain Prefix"
                    hint="当前选择的是通配符域名，仅输入最前面的前缀即可。"
                    error={form.formState.errors.subdomain_label?.message}
                  >
                    <HeroTextInput
                      aria-label="Subdomain Prefix"
                      placeholder="e.g. ai"
                      {...form.register('subdomain_label')}
                    />
                  </ResourceField>
                ) : null}
              </div>
            </div>
          </ProxyRuleSection>

          <ProxyRuleSection
            title="Origin"
            icon={<ServerIcon className="h-4 w-4" />}
            action={
              <button
                type="button"
                onClick={() =>
                  appendOrigin({
                    scheme: primaryOriginRow.scheme,
                    address: '',
                    port: getDefaultPortForScheme(primaryOriginRow.scheme),
                  })
                }
                className="inline-flex items-center gap-1 text-sm font-medium text-[var(--brand-primary)] transition hover:opacity-80"
              >
                <PlusIcon className="h-4 w-4" />
                Add Row
              </button>
            }
          >
            <div className="space-y-3">
              {originFields.map((field, index) => {
                const currentRow =
                  watchedOriginRows?.[index] ?? defaultValues.origin_rows[0];

                return (
                  <div key={field.id} className="space-y-2">
                    <div className="grid gap-3 md:grid-cols-[120px_minmax(0,1fr)_96px_44px]">
                      <div>
                        <span className="sr-only">{`协议 ${index + 1}`}</span>
                        <HeroSelectField
                          ariaLabel={`协议 ${index + 1}`}
                          value={currentRow.scheme}
                          onChange={(nextValue) => {
                            const nextScheme =
                              nextValue as OriginRowFormValue['scheme'];
                            form.setValue(
                              `origin_rows.${index}.scheme`,
                              nextScheme,
                              {
                                shouldDirty: true,
                                shouldValidate: true,
                              },
                            );

                            const currentPort = form.getValues(
                              `origin_rows.${index}.port`,
                            );
                            if (
                              !currentPort ||
                              currentPort === '80' ||
                              currentPort === '443'
                            ) {
                              form.setValue(
                                `origin_rows.${index}.port`,
                                getDefaultPortForScheme(nextScheme),
                                {
                                  shouldDirty: true,
                                  shouldValidate: true,
                                },
                              );
                            }
                          }}
                        >
                          <SelectItem key="https">HTTPS</SelectItem>
                          <SelectItem key="http">HTTP</SelectItem>
                        </HeroSelectField>
                      </div>

                      <SearchableOriginAddressField
                        value={currentRow.address}
                        origins={origins}
                        onValueChange={(nextValue) =>
                          form.setValue(`origin_rows.${index}.address`, nextValue, {
                            shouldDirty: true,
                            shouldValidate: true,
                          })
                        }
                      />

                      <div>
                        <span className="sr-only">{`端口 ${index + 1}`}</span>
                        <HeroTextInput
                          aria-label={`端口 ${index + 1}`}
                          placeholder="443"
                          {...form.register(`origin_rows.${index}.port`)}
                        />
                      </div>

                      <button
                        type="button"
                        onClick={() => {
                          if (originFields.length === 1) {
                            replaceOrigins([defaultValues.origin_rows[0]]);
                            return;
                          }
                          removeOrigin(index);
                        }}
                        className="flex h-10 w-10 items-center justify-center rounded-xl border border-[var(--border-default)] bg-[var(--surface-panel)] text-[var(--foreground-muted)] transition hover:border-[var(--status-danger-border)] hover:text-[var(--status-danger-foreground)]"
                        aria-label={`删除源站 ${index + 1}`}
                      >
                        <TrashIcon className="h-4 w-4" />
                      </button>
                    </div>

                    {(form.formState.errors.origin_rows?.[index]?.address
                      ?.message ||
                      form.formState.errors.origin_rows?.[index]?.port
                        ?.message) && (
                      <p className="text-xs text-[var(--status-danger-foreground)]">
                        {form.formState.errors.origin_rows?.[index]?.address
                          ?.message ||
                          form.formState.errors.origin_rows?.[index]?.port
                            ?.message}
                      </p>
                    )}

                    {index === 0 ? (
                        <div>
                          <span className="sr-only">Origin Path / Query</span>
                          <HeroTextInput
                              aria-label="Origin Path / Query"
                              placeholder="/api"
                              {...form.register('origin_uri')}
                          />
                        </div>
                    ) : null}
                  </div>
                );
              })}
            </div>
          </ProxyRuleSection>

          <ProxyRuleSection
            title="Protocol"
            icon={<ShieldIcon className="h-4 w-4" />}
          >
            <div className="space-y-4">
              <div className="rounded-2xl border border-[var(--border-default)] bg-[color:color-mix(in_srgb,var(--surface-elevated)_82%,white_18%)] p-4">
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1">
                    <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                      HTTPS Access
                    </p>
                    <p className="text-sm leading-5 text-[var(--foreground-secondary)]">
                      Enforce secure TLS connections for this rule.
                    </p>
                  </div>
                  <Switch
                    id="proxy-route-enable-https"
                    checked={watchedEnableHttps}
                    aria-label="HTTPS Access"
                    onCheckedChange={(checked) => {
                      form.setValue('enable_https', checked, {
                        shouldDirty: true,
                        shouldValidate: true,
                      });

                      if (
                        checked &&
                        !form.getValues('cert_id') &&
                        selectedManagedDomain?.cert_id
                      ) {
                        form.setValue(
                          'cert_id',
                          String(selectedManagedDomain.cert_id),
                          {
                            shouldDirty: true,
                            shouldValidate: true,
                          },
                        );
                      }

                      if (!checked) {
                        form.setValue('cert_id', '', {
                          shouldDirty: true,
                          shouldValidate: true,
                        });
                        form.setValue('redirect_http', false, {
                          shouldDirty: true,
                          shouldValidate: true,
                        });
                      }
                    }}
                  />
                </div>

                {watchedEnableHttps ? (
                  <div className="mt-5 space-y-4">
                  <ResourceField
                    label="Select Certificate"
                    hint={getMatchMessage(
                      matchResult,
                      isMatching,
                      effectiveDomain,
                      watchedEnableHttps,
                    )}
                    error={form.formState.errors.cert_id?.message}
                  >
                    <HeroSelectField
                      ariaLabel="Select Certificate"
                      value={watchedCertId}
                      disabled={certificatesQuery.isLoading}
                      onChange={(nextValue) =>
                        form.setValue('cert_id', nextValue, {
                          shouldDirty: true,
                          shouldValidate: true,
                        })
                      }
                    >
                      {matchResult?.candidate ? (
                        <SelectSection title="自动匹配">
                          <SelectItem key={String(matchResult.candidate.certificate_id)}>
                            {matchResult.candidate.certificate_name}
                          </SelectItem>
                        </SelectSection>
                      ) : null}
                      <SelectSection title="全部证书">
                        {certificates
                          .filter(
                            (certificate) =>
                              String(certificate.id) !==
                              String(matchResult?.candidate?.certificate_id ?? ''),
                          )
                          .map((certificate) => (
                            <SelectItem
                              key={String(certificate.id)}
                              textValue={buildCertificateLabel(certificate)}
                            >
                              {buildCertificateLabel(certificate)}
                            </SelectItem>
                          ))}
                      </SelectSection>
                    </HeroSelectField>
                  </ResourceField>

                    <ToggleField
                      label="HTTP 跳转 HTTPS"
                      description="开启后会将 HTTP 请求重定向到 HTTPS。"
                      checked={watchedRedirectHttp}
                      disabled={!watchedEnableHttps}
                      onChange={(checked) =>
                        form.setValue('redirect_http', checked, {
                          shouldDirty: true,
                          shouldValidate: true,
                        })
                      }
                    />
                  </div>
                ) : null}
              </div>

              {matchResult?.matched && matchResult.candidates.length > 1 ? (
                <div className="flex flex-wrap gap-2">
                  {matchResult.candidates.map((candidate) => (
                    <StatusBadge
                      key={`${candidate.managed_domain_id}-${candidate.certificate_id}`}
                      label={`${candidate.domain} → ${candidate.certificate_name}`}
                      variant={
                        candidate.match_type === 'exact' ? 'success' : 'info'
                      }
                    />
                  ))}
                </div>
              ) : null}
            </div>
          </ProxyRuleSection>

          <section className="space-y-3">
            <button
              type="button"
              onClick={() => setIsAdvancedOpen((current) => !current)}
              className="flex w-full items-center justify-between gap-3 text-left"
            >
              <div className="flex items-center gap-3 text-[var(--foreground-primary)]">
                <span className="flex h-8 w-8 items-center justify-center rounded-full border border-[var(--border-default)] bg-[var(--surface-elevated)] text-[var(--foreground-secondary)]">
                  <SlidersIcon className="h-4 w-4" />
                </span>
                <h3 className="text-sm font-semibold uppercase tracking-[0.16em]">
                  Advanced
                </h3>
              </div>
              <ChevronIcon
                expanded={isAdvancedOpen}
                className="h-5 w-5 text-[var(--foreground-muted)]"
              />
            </button>

            {isAdvancedOpen ? (
              <div className={cn(PANEL_CLASS_NAME, 'space-y-4')}>
                <div className="grid gap-4 lg:grid-cols-1">
                  <div className="space-y-4">
                    <ResourceField
                      label="Origin Host Header"
                      hint="留空则默认使用访问域名 $host。"
                      error={form.formState.errors.origin_host?.message}
                    >
                      <HeroTextInput
                        aria-label="Origin Host Header"
                        placeholder="example.com"
                        {...form.register('origin_host')}
                      />
                    </ResourceField>

                    <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] p-4">
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <div>
                          <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                            Custom Request Headers
                          </p>
                          <p className="mt-1 text-xs leading-5 text-[var(--foreground-secondary)]">
                            使用 Key | Value 横向排列，方便快速录入运维透传头。
                          </p>
                        </div>
                        <button
                          type="button"
                          onClick={() => appendHeader({ key: '', value: '' })}
                          className="text-sm font-medium text-[var(--brand-primary)] transition hover:opacity-80"
                        >
                          + Add Header
                        </button>
                      </div>

                      <div className="mt-4 space-y-3">
                        {headerFields.map((field, index) => (
                          <div
                            key={field.id}
                            className="grid gap-3 md:grid-cols-[1fr_1fr_28px]"
                          >
                            <HeroTextInput
                              aria-label={`Custom Header Key ${index + 1}`}
                              placeholder="X-Forwarded-For"
                              {...form.register(`custom_headers.${index}.key`)}
                            />
                            <HeroTextInput
                              aria-label={`Custom Header Value ${index + 1}`}
                              placeholder="$remote_addr"
                              {...form.register(`custom_headers.${index}.value`)}
                            />
                            <button
                              type="button"
                              onClick={() => handleRemoveHeader(index)}
                              className="flex h-10 w-7 items-center justify-center text-lg text-[var(--foreground-muted)] transition hover:text-[var(--status-danger-foreground)]"
                              aria-label={`删除请求头 ${index + 1}`}
                            >
                              ×
                            </button>

                            {(form.formState.errors.custom_headers?.[index]?.key
                              ?.message ||
                              form.formState.errors.custom_headers?.[index]
                                ?.value?.message) && (
                              <p className="md:col-span-3 text-xs text-[var(--status-danger-foreground)]">
                                {form.formState.errors.custom_headers?.[index]
                                  ?.key?.message ||
                                  form.formState.errors.custom_headers?.[index]
                                    ?.value?.message}
                              </p>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                  <Separator />
                  <div className="space-y-4">
                    <ToggleField
                      label="Enable Rule Caching"
                      description="仅对当前规则生效；系统会自动绕过非 GET、Authorization 和常见登录态 Cookie 请求。"
                      checked={watchedCacheEnabled}
                      onChange={(checked) => {
                        form.setValue('cache_enabled', checked, {
                          shouldDirty: true,
                          shouldValidate: true,
                        });
                        if (!checked) {
                          form.setValue('cache_policy', 'url', {
                            shouldDirty: true,
                            shouldValidate: true,
                          });
                          form.setValue('cache_rules_text', '', {
                            shouldDirty: true,
                            shouldValidate: true,
                          });
                        }
                      }}
                    />

                    <ResourceField
                      label="缓存策略"
                      hint="按 URL 会缓存所有符合安全条件的 URL；其余策略会先匹配规则再决定是否缓存。"
                    >
                      <HeroSelectField
                        ariaLabel="缓存策略"
                        value={watchedCachePolicy}
                        disabled={!watchedCacheEnabled}
                        onChange={(nextValue) =>
                          form.setValue(
                            'cache_policy',
                            nextValue as ProxyRouteFormValues['cache_policy'],
                            {
                              shouldDirty: true,
                              shouldValidate: true,
                            },
                          )
                        }
                      >
                        <SelectItem key="url">按 URL 缓存</SelectItem>
                        <SelectItem key="suffix">按后缀匹配缓存</SelectItem>
                        <SelectItem key="path_prefix">按路径前缀缓存</SelectItem>
                        <SelectItem key="path_exact">按精确路径缓存</SelectItem>
                      </HeroSelectField>
                    </ResourceField>

                    <ResourceField
                      label="缓存规则"
                      hint={getCacheRulesHint(watchedCachePolicy)}
                      error={form.formState.errors.cache_rules_text?.message}
                    >
                      <HeroTextArea
                        aria-label="缓存规则"
                        placeholder={
                          watchedCachePolicy === 'suffix'
                            ? 'jpg\ncss\njs'
                            : watchedCachePolicy === 'path_prefix'
                              ? '/assets\n/static/images'
                              : watchedCachePolicy === 'path_exact'
                                ? '/robots.txt\n/manifest.json'
                                : '按 URL 缓存无需填写规则'
                        }
                        disabled={
                          !watchedCacheEnabled || watchedCachePolicy === 'url'
                        }
                        className="min-h-32"
                        {...form.register('cache_rules_text')}
                      />
                    </ResourceField>
                  </div>
                </div>

                <ResourceField
                  label="Remarks"
                  error={form.formState.errors.remark?.message}
                >
                  <HeroTextArea
                    aria-label="Remarks"
                    placeholder="Internal notes for this rule..."
                    {...form.register('remark')}
                  />
                </ResourceField>
              </div>
            ) : null}
          </section>
        </form>
      </AppModal>
    </>
  );
}
