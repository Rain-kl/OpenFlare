'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useFieldArray, useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppModal } from '@/components/ui/app-modal';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
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
  ResourceInput,
  ResourceSelect,
  ResourceTextarea,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

const customHeaderSchema = z.object({
  key: z.string(),
  value: z.string(),
});

const cachePolicyValues = [
  'url',
  'suffix',
  'path_prefix',
  'path_exact',
] as const;
const originProtocolValues = ['http', 'https'] as const;

const proxyRouteSchema = z
  .object({
    managed_domain_id: z.string().trim().min(1, '请选择网站'),
    subdomain_label: z.string(),
    origin_id: z.string(),
    origin_scheme: z.enum(originProtocolValues),
    origin_address: z.string().trim().min(1, '请输入源站地址'),
    origin_port: z.string().trim().min(1, '请输入端口'),
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
    upstreams_text: z.string(),
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
        message: '请输入二级域名',
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
        message: '二级域名仅支持单个标签，且只能包含字母、数字和中划线',
      });
    }

    if (value.enable_https && !value.cert_id) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['cert_id'],
        message: '启用 HTTPS 时必须选择证书',
      });
    }

    const normalizedOriginAddress = value.origin_address.trim();
    if (
      normalizedOriginAddress &&
      (/[/?#]/.test(normalizedOriginAddress) ||
        normalizedOriginAddress.includes('://'))
    ) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['origin_address'],
        message: '源站地址仅支持 IP、域名或主机名',
      });
    }

    const normalizedOriginPort = value.origin_port.trim();
    const portNumber = Number(normalizedOriginPort);
    if (
      normalizedOriginPort &&
      (!/^\d+$/.test(normalizedOriginPort) ||
        !Number.isInteger(portNumber) ||
        portNumber < 1 ||
        portNumber > 65535)
    ) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['origin_port'],
        message: '端口需为 1 到 65535 的整数',
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
        message: '源站路径需以 / 或 ? 开头',
      });
    }

    const primaryOriginURL = buildOriginUrl(
      value.origin_scheme,
      value.origin_address,
      value.origin_port,
      value.origin_uri,
    );
    if (!primaryOriginURL) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['origin_address'],
        message: '请输入完整的源站信息',
      });
    }

    const upstreams = parseUpstreamsText(
      primaryOriginURL,
      value.upstreams_text,
    );
    if (upstreams.length === 0) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['origin_address'],
        message: '至少需要一个上游地址',
      });
    }

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

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

const defaultValues: ProxyRouteFormValues = {
  managed_domain_id: '',
  subdomain_label: '',
  origin_id: '',
  origin_scheme: 'https',
  origin_address: '',
  origin_port: '',
  origin_uri: '',
  origin_host: '',
  upstreams_text: '',
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

function parseUpstreamsText(primary: string, value: string) {
  return [primary.trim(), ...value.split(/\r?\n/)]
    .map((item) => item.trim())
    .filter(Boolean);
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
      port: '',
      uri: '',
    };
  }
}

function toPayload(values: ProxyRouteFormValues): ProxyRouteMutationPayload {
  const primaryOriginUrl = buildOriginUrl(
    values.origin_scheme,
    values.origin_address,
    values.origin_port,
    values.origin_uri,
  );

  return {
    domain: buildRouteDomain(values.managed_domain_id, values.subdomain_label),
    origin_id: values.origin_id ? Number(values.origin_id) : null,
    origin_url: primaryOriginUrl,
    origin_scheme: values.origin_scheme,
    origin_address: values.origin_address.trim(),
    origin_port: values.origin_port.trim(),
    origin_uri: values.origin_uri.trim(),
    origin_host: values.origin_host.trim(),
    upstreams: parseUpstreamsText(
      primaryOriginUrl,
      values.upstreams_text,
    ).slice(1),
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

  const parsedOrigin = parseOriginUrl(route.origin_url);

  return {
    managed_domain_id: managedDomainMatch.managedDomainId,
    subdomain_label: managedDomainMatch.subdomainLabel,
    origin_id: route.origin_id ? String(route.origin_id) : '',
    origin_scheme: parsedOrigin.scheme,
    origin_address: parsedOrigin.address,
    origin_port: parsedOrigin.port,
    origin_uri: parsedOrigin.uri,
    origin_host: route.origin_host || '',
    upstreams_text: upstreams.slice(1).join('\n'),
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
    return '启用 HTTPS 后，系统会根据输入域名尝试自动匹配托管证书。';
  }

  if (isMatching) {
    return '正在按域名自动匹配托管证书...';
  }

  if (!domain.trim()) {
    return '输入域名后会自动匹配证书，并优先推荐精确匹配规则。';
  }

  if (matchResult?.matched && matchResult.candidate) {
    return `已匹配${matchResult.candidate.match_type === 'exact' ? '精确' : '通配符'}规则 ${matchResult.candidate.domain}，推荐证书：${matchResult.candidate.certificate_name}`;
  }

  return '未找到匹配证书，可继续手动选择。';
}

export function ProxyRoutesPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [editingRouteId, setEditingRouteId] = useState<number | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [matchResult, setMatchResult] =
    useState<ManagedDomainMatchResult | null>(null);
  const [isMatching, setIsMatching] = useState(false);

  const form = useForm<ProxyRouteFormValues>({
    resolver: zodResolver(proxyRouteSchema),
    defaultValues,
  });

  const { fields, append, remove, replace } = useFieldArray({
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
  const watchedOriginAddress = useWatch({
    control: form.control,
    name: 'origin_address',
  });
  const watchedOriginScheme = useWatch({
    control: form.control,
    name: 'origin_scheme',
  });
  const watchedOriginPort = useWatch({
    control: form.control,
    name: 'origin_port',
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

  const saveMutation = useMutation({
    mutationFn: async (values: ProxyRouteFormValues) => {
      const payload = toPayload(values);
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

  const managedDomains = useMemo(
    () => managedDomainsQuery.data ?? [],
    [managedDomainsQuery.data],
  );
  const origins = useMemo(() => originsQuery.data ?? [], [originsQuery.data]);

  const matchedOrigin = useMemo(
    () =>
      origins.find(
        (item) =>
          item.address.toLowerCase() ===
          watchedOriginAddress.trim().toLowerCase(),
      ) ?? null,
    [origins, watchedOriginAddress],
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
  const primaryOriginPreview = buildOriginUrl(
    watchedOriginScheme,
    watchedOriginAddress,
    watchedOriginPort,
    watchedOriginURI,
  );

  useEffect(() => {
    form.setValue('origin_id', matchedOrigin ? String(matchedOrigin.id) : '', {
      shouldDirty: true,
      shouldValidate: false,
    });
  }, [form, matchedOrigin]);

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
  }, [effectiveDomain, watchedEnableHttps, form]);

  const certificates = useMemo(
    () => certificatesQuery.data ?? [],
    [certificatesQuery.data],
  );

  const handleReset = () => {
    setFeedback(null);
    setEditingRouteId(null);
    setIsEditorOpen(false);
    setMatchResult(null);
    form.reset(defaultValues);
  };

  const handleCreate = () => {
    setFeedback(null);
    setEditingRouteId(null);
    setMatchResult(null);
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
    if (fields.length === 1) {
      replace([{ key: '', value: '' }]);
      return;
    }

    remove(index);
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
          className="space-y-5"
          onSubmit={handleSubmit}
        >
          <div className="grid gap-4 md:grid-cols-2">
            <ResourceField
              label="网站"
              hint="先选择已托管的网站，再根据类型补充规则域名。"
              error={form.formState.errors.managed_domain_id?.message}
            >
              <ResourceSelect
                value={watchedManagedDomain}
                disabled={managedDomainsQuery.isLoading}
                onChange={(event) => {
                  const nextDomain = event.target.value;

                  form.setValue('managed_domain_id', nextDomain, {
                    shouldDirty: true,
                    shouldValidate: true,
                  });

                  if (!isWildcardManagedDomain(nextDomain)) {
                    form.setValue('subdomain_label', '', {
                      shouldDirty: true,
                      shouldValidate: true,
                    });
                  }
                }}
              >
                <option value="">请选择网站</option>
                {managedDomains.map((domain) => (
                  <option key={domain.id} value={domain.domain}>
                    {domain.domain}
                  </option>
                ))}
              </ResourceSelect>
            </ResourceField>
            <ResourceField
              label="源站协议"
              hint="先选择回源协议，再选择或输入源站地址。"
            >
              <ResourceSelect
                value={watchedOriginScheme}
                onChange={(event) =>
                  form.setValue(
                    'origin_scheme',
                    event.target.value as ProxyRouteFormValues['origin_scheme'],
                    {
                      shouldDirty: true,
                      shouldValidate: true,
                    },
                  )
                }
              >
                <option value="https">https</option>
                <option value="http">http</option>
              </ResourceSelect>
            </ResourceField>
          </div>

          <div className="grid gap-4 md:grid-cols-[1.2fr_0.8fr]">
            <ResourceField
              label="源站"
              hint="可从已有源站中选择，也可以直接输入新地址；保存规则后会自动创建不存在的源站。"
              error={form.formState.errors.origin_address?.message}
            >
              <ResourceInput
                list="proxy-route-origin-addresses"
                placeholder="origin.internal"
                {...form.register('origin_address')}
              />
              <datalist id="proxy-route-origin-addresses">
                {origins.map((origin) => (
                  <option
                    key={origin.id}
                    value={origin.address}
                    label={origin.name}
                  />
                ))}
              </datalist>
            </ResourceField>
            <ResourceField
              label="端口"
              hint="规则主源站的访问端口。"
              error={form.formState.errors.origin_port?.message}
            >
              <ResourceInput
                placeholder="443"
                {...form.register('origin_port')}
              />
            </ResourceField>
          </div>

          {selectedManagedDomain ? (
            isWildcardSelection ? (
              <div className="grid gap-4 md:grid-cols-[0.9fr_1.1fr]">
                <ResourceField
                  label="二级域名"
                  hint={`当前网站为通配符 ${selectedManagedDomain.domain}，这里只需填写前缀，例如 ai。`}
                  error={form.formState.errors.subdomain_label?.message}
                >
                  <ResourceInput
                    placeholder="ai"
                    {...form.register('subdomain_label')}
                  />
                </ResourceField>
                <AppCard
                  title="规则域名预览"
                  description="系统会自动拼接通配符后缀，生成最终规则域名。"
                >
                  <p className="text-sm leading-6 text-[var(--foreground-secondary)]">
                    {effectiveDomain
                      ? `当前将生成规则域名 ${effectiveDomain}`
                      : `请输入二级域名前缀，系统会自动生成 *.${selectedManagedDomain.domain.slice(2)} 下的规则域名。`}
                  </p>
                </AppCard>
              </div>
            ) : (
              <AppCard
                title="规则域名预览"
                description="当前网站为精确域名，规则会直接使用该网站。"
              >
                <p className="text-sm leading-6 text-[var(--foreground-secondary)]">
                  {`当前将直接使用 ${selectedManagedDomain.domain} 作为规则域名，无需再填写网站名。`}
                </p>
              </AppCard>
            )
          ) : null}

          <div className="grid gap-4 md:grid-cols-[1fr_1fr]">
            <ResourceField
              label="源站路径"
              hint="可选，用于保留已有带路径或查询参数的回源能力，例如 /api 或 ?token=demo。"
              error={form.formState.errors.origin_uri?.message}
            >
              <ResourceInput
                placeholder="/api"
                {...form.register('origin_uri')}
              />
            </ResourceField>
            <AppCard
              title="主源站预览"
              description={
                matchedOrigin
                  ? `当前会复用已存在的源站 ${matchedOrigin.name}。`
                  : '如果这里的地址尚未收录，保存规则时会自动创建一个新源站。'
              }
            >
              <p className="text-sm leading-6 text-[var(--foreground-secondary)]">
                {primaryOriginPreview ||
                  '选择协议、输入源站和端口后，这里会显示最终主源站地址。'}
              </p>
            </AppCard>
          </div>

          <ResourceField
            label="回源主机名"
            hint="可选。填写后将覆盖回源请求的 Host，留空则默认使用访问域名 $host"
            error={form.formState.errors.origin_host?.message}
          >
            <ResourceInput {...form.register('origin_host')} />
          </ResourceField>

          <ResourceField
            label="附加上游"
            hint="可选。每行一个上游地址，用于同一规则下的负载均衡；需与主上游保持相同协议，且不能带路径或查询参数。"
            error={form.formState.errors.upstreams_text?.message}
          >
            <ResourceTextarea
              placeholder={
                'https://origin-b.internal\nhttps://origin-c.internal'
              }
              {...form.register('upstreams_text')}
            />
          </ResourceField>

          <div className="grid gap-4 lg:grid-cols-2">
            <ToggleField
              label="启用规则"
              description="关闭后该规则不会参与配置渲染与发布。"
              checked={watchedEnabled}
              onChange={(checked) =>
                form.setValue('enabled', checked, { shouldDirty: true })
              }
            />
            <ToggleField
              label="启用 HTTPS"
              description="启用后必须关联 TLS 证书，并会默认为客户端开启 HTTP/2；可选择是否将 HTTP 自动重定向到 HTTPS。"
              checked={watchedEnableHttps}
              onChange={(checked) => {
                form.setValue('enable_https', checked, {
                  shouldDirty: true,
                  shouldValidate: true,
                });
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

          <div className="grid gap-4 lg:grid-cols-[1.2fr_0.8fr]">
            <ResourceField
              label="TLS 证书"
              hint="启用 HTTPS 后可自动推荐匹配证书，也支持手动选择。"
              error={form.formState.errors.cert_id?.message}
            >
              <ResourceSelect
                value={watchedCertId}
                disabled={!watchedEnableHttps || certificatesQuery.isLoading}
                onChange={(event) =>
                  form.setValue('cert_id', event.target.value, {
                    shouldDirty: true,
                    shouldValidate: true,
                  })
                }
              >
                <option value="">请选择证书</option>
                {certificates.map((certificate) => (
                  <option key={certificate.id} value={certificate.id}>
                    {buildCertificateLabel(certificate)}
                  </option>
                ))}
              </ResourceSelect>
            </ResourceField>
            <ToggleField
              label="HTTP 跳转 HTTPS"
              description="仅在启用 HTTPS 后可开启。开启后会将 HTTP 请求重定向到 HTTPS。"
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

          <div className="grid gap-4 lg:grid-cols-[0.8fr_1.2fr]">
            <ToggleField
              label="启用规则缓存"
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
              <ResourceSelect
                value={watchedCachePolicy}
                disabled={!watchedCacheEnabled}
                onChange={(event) =>
                  form.setValue(
                    'cache_policy',
                    event.target.value as ProxyRouteFormValues['cache_policy'],
                    {
                      shouldDirty: true,
                      shouldValidate: true,
                    },
                  )
                }
              >
                <option value="url">按 URL 缓存</option>
                <option value="suffix">按后缀匹配缓存</option>
                <option value="path_prefix">按路径前缀缓存</option>
                <option value="path_exact">按精确路径缓存</option>
              </ResourceSelect>
            </ResourceField>
          </div>

          <ResourceField
            label="缓存规则"
            hint={getCacheRulesHint(watchedCachePolicy)}
            error={form.formState.errors.cache_rules_text?.message}
          >
            <ResourceTextarea
              placeholder={
                watchedCachePolicy === 'suffix'
                  ? 'jpg\ncss\njs'
                  : watchedCachePolicy === 'path_prefix'
                    ? '/assets\n/static/images'
                    : watchedCachePolicy === 'path_exact'
                      ? '/robots.txt\n/manifest.json'
                      : '按 URL 缓存无需填写规则'
              }
              disabled={!watchedCacheEnabled || watchedCachePolicy === 'url'}
              {...form.register('cache_rules_text')}
            />
          </ResourceField>

          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                  自定义请求头
                </p>
                <p className="mt-1 text-xs leading-5 text-[var(--foreground-secondary)]">
                  可为空。若填写，请保证 Header 名称合法且不包含换行。
                </p>
              </div>
              <SecondaryButton
                type="button"
                onClick={() => append({ key: '', value: '' })}
                className="px-3 py-2 text-xs"
              >
                添加请求头
              </SecondaryButton>
            </div>

            <div className="mt-4 space-y-4">
              {fields.map((field, index) => (
                <div
                  key={field.id}
                  className="grid gap-3 md:grid-cols-[1fr_1fr_auto]"
                >
                  <ResourceField
                    label={
                      index === 0 ? 'Header 名称' : `Header 名称 ${index + 1}`
                    }
                    error={
                      form.formState.errors.custom_headers?.[index]?.key
                        ?.message
                    }
                  >
                    <ResourceInput
                      placeholder="X-Trace-Id"
                      {...form.register(`custom_headers.${index}.key`)}
                    />
                  </ResourceField>
                  <ResourceField
                    label={index === 0 ? 'Header 值' : `Header 值 ${index + 1}`}
                    error={
                      form.formState.errors.custom_headers?.[index]?.value
                        ?.message
                    }
                  >
                    <ResourceInput
                      placeholder="$request_id"
                      {...form.register(`custom_headers.${index}.value`)}
                    />
                  </ResourceField>
                  <div className="flex items-end">
                    <DangerButton
                      type="button"
                      onClick={() => handleRemoveHeader(index)}
                      className="w-full px-3 py-3 text-xs md:w-auto"
                    >
                      删除
                    </DangerButton>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <AppCard
            title="证书匹配提示"
            description="根据域名自动匹配托管证书，优先使用精确匹配规则。"
          >
            <div className="space-y-3">
              <p className="text-sm leading-6 text-[var(--foreground-secondary)]">
                {getMatchMessage(
                  matchResult,
                  isMatching,
                  effectiveDomain,
                  watchedEnableHttps,
                )}
              </p>
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
              <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4 text-sm text-[var(--foreground-secondary)]">
                当前可选证书：
                {certificatesQuery.isLoading
                  ? '加载中...'
                  : `${certificates.length} 张`}
              </div>
            </div>
          </AppCard>

          <ResourceField
            label="备注"
            error={form.formState.errors.remark?.message}
          >
            <ResourceTextarea
              placeholder="例如：主站生产流量入口"
              {...form.register('remark')}
            />
          </ResourceField>
        </form>
      </AppModal>
    </>
  );
}
