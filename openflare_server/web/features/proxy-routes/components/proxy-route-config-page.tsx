'use client';

import Link from 'next/link';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { z } from 'zod';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { getManagedDomains } from '@/features/managed-domains/api/managed-domains';
import { getPagesProjects } from '@/features/pages/api/pages';
import {
  getProxyRoute,
  updateProxyRoute,
} from '@/features/proxy-routes/api/proxy-routes';
import {
  getWAFSiteRuleGroups,
  replaceWAFSiteRuleGroups,
} from '@/features/waf/api/waf';
import { getNodes } from '@/features/nodes/api/nodes';
import {
  buildDomainRowsFromRoute,
  DomainListInput,
  type DomainListRow,
} from '@/features/proxy-routes/components/domain-list-input';
import {
  buildPayloadFromRoute,
  customHeadersToText,
  getErrorMessage,
  getWebsiteConfigSection,
  linesFromTextarea,
  normalizeLimitRate,
  parseCustomHeadersText,
  parseOriginUrl,
  parseOriginUrls,
  validateCacheRules,
  validateDomains,
  validateLimitRate,
  validateOriginHost,
  websiteConfigSections,
} from '@/features/proxy-routes/helpers';
import type {
  ProxyRouteItem,
  ProxyRouteMutationPayload,
} from '@/features/proxy-routes/types';
import { getTlsCertificates } from '@/features/tls-certificates/api/tls-certificates';
import type { TlsCertificateItem } from '@/features/tls-certificates/types';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  ResourceTextarea,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
import { cn } from '@/lib/utils/cn';

type FeedbackState = {
  tone: 'success' | 'danger';
  message: string;
};

type SaveContext = {
  message: string;
};

type SaveHandler = (
  payload: ProxyRouteMutationPayload,
  context: SaveContext,
) => void;

const domainSettingsSchema = z
  .object({
    site_name: z
      .string()
      .trim()
      .min(1, '请输入站点标识')
      .max(255, '站点标识不能超过 255 个字符'),
    domain_rows: z
      .array(
        z.object({
          domain: z.string(),
          certificateId: z.string(),
        }),
      )
      .min(1),
    enabled: z.boolean(),
    redirect_http: z.boolean(),
  })
  .superRefine((value, context) => {
    const domains = value.domain_rows
      .map((item) => item.domain.trim().toLowerCase())
      .filter(Boolean);
    const error = validateDomains(domains);
    if (error) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['domain_rows'],
        message: error,
      });
    }

    const selectedCertificateCount = new Set(
      value.domain_rows
        .map((item) => Number(item.certificateId))
        .filter((item) => Number.isFinite(item) && item > 0),
    ).size;
    if (value.redirect_http && selectedCertificateCount === 0) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['redirect_http'],
        message: '启用 HTTP 跳转前，请先为域名选择证书',
      });
    }
  });

const rateLimitSchema = z
  .object({
    limit_conn_per_server: z.string(),
    limit_conn_per_ip: z.string(),
    limit_rate: z.string(),
  })
  .superRefine((value, context) => {
    for (const field of [
      'limit_conn_per_server',
      'limit_conn_per_ip',
    ] as const) {
      const rawValue = value[field].trim();
      if (!rawValue) {
        continue;
      }
      if (!/^\d+$/.test(rawValue)) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: [field],
          message: '请输入大于等于 0 的整数',
        });
      }
    }

    const limitRateError = validateLimitRate(value.limit_rate);
    if (limitRateError) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['limit_rate'],
        message: limitRateError,
      });
    }
  });

const reverseProxySchema = z
  .object({
    upstream_type: z.enum(['direct', 'tunnel', 'pages']),
    origin_urls_text: z.string().trim(),
    origin_host: z.string(),
    tunnel_id: z.string().optional(),
    tunnel_target_addr: z.string().trim().optional(),
    tunnel_target_protocol: z.enum(['http', 'https']).optional(),
    pages_project_id: z.string().optional(),
    custom_headers_text: z.string(),
    remark: z.string().max(255, '备注不能超过 255 个字符'),
  })
  .superRefine((value, context) => {
    if (value.upstream_type === 'direct') {
      if (!value.origin_urls_text.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['origin_urls_text'],
          message: '请至少填写一个上游地址',
        });
      } else {
        const { error } = parseOriginUrls(value.origin_urls_text);
        if (error) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['origin_urls_text'],
            message: error,
          });
        }
      }
    } else if (value.upstream_type === 'tunnel') {
      if (!value.tunnel_id) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['tunnel_id'],
          message: '请选择内网穿透隧道',
        });
      }
      if (!value.tunnel_target_addr) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['tunnel_target_addr'],
          message: '请填写内网服务地址 (如 127.0.0.1:8080)',
        });
      }
    } else if (!value.pages_project_id) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['pages_project_id'],
        message: '请选择 Pages 项目',
      });
    }

    const originHostError = validateOriginHost(value.origin_host);
    if (originHostError) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['origin_host'],
        message: originHostError,
      });
    }

    const { error: headerError } = parseCustomHeadersText(
      value.custom_headers_text,
    );
    if (headerError) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['custom_headers_text'],
        message: headerError,
      });
    }
  });

const cacheSchema = z
  .object({
    cache_enabled: z.boolean(),
    cache_policy: z.enum(['url', 'suffix', 'path_prefix', 'path_exact']),
    cache_rules_text: z.string(),
  })
  .superRefine((value, context) => {
    if (!value.cache_enabled) {
      return;
    }

    const rules = linesFromTextarea(value.cache_rules_text);
    const error = validateCacheRules(value.cache_policy, rules);
    if (error) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['cache_rules_text'],
        message: error,
      });
    }
  });

type DomainSettingsValues = z.infer<typeof domainSettingsSchema>;
type RateLimitValues = z.infer<typeof rateLimitSchema>;
type ReverseProxyValues = z.infer<typeof reverseProxySchema>;
type CacheValues = z.infer<typeof cacheSchema>;

function normalizeSelectedCertificateIDs(rows: DomainListRow[]) {
  return Array.from(
    new Set(
      rows
        .filter((item) => item.domain.trim() !== '')
        .map((item) => Number(item.certificateId))
        .filter((item) => Number.isFinite(item) && item > 0),
    ),
  );
}

function buildDomainCertificateIDs(rows: DomainListRow[]) {
  return rows
    .filter((item) => item.domain.trim() !== '')
    .map((item) => {
      const certificateID = Number(item.certificateId);
      return Number.isFinite(certificateID) && certificateID > 0
        ? certificateID
        : 0;
    });
}

function buildDomainRows(route: ProxyRouteItem) {
  const selectedCertIDs =
    route.cert_ids.length > 0
      ? route.cert_ids
      : route.cert_id
        ? [route.cert_id]
        : [];

  return buildDomainRowsFromRoute(
    route.domains,
    route.domain_cert_ids,
    selectedCertIDs,
  );
}

function ConfigSectionShell({
  title,
  description,
  formId,
  saving,
  children,
}: {
  title: string;
  description: string;
  formId: string;
  saving: boolean;
  children: ReactNode;
}) {
  return (
    <AppCard
      title={title}
      description={description}
      action={
        <PrimaryButton type="submit" form={formId} disabled={saving}>
          {saving ? '保存中...' : '保存'}
        </PrimaryButton>
      }
    >
      {children}
    </AppCard>
  );
}

function DomainSettingsSection({
  route,
  certificates,
  saving,
  onSave,
  suggestionSources,
}: {
  route: ProxyRouteItem;
  certificates: TlsCertificateItem[];
  saving: boolean;
  onSave: SaveHandler;
  suggestionSources: string[];
}) {
  const form = useForm<DomainSettingsValues>({
    resolver: zodResolver(domainSettingsSchema),
    defaultValues: {
      site_name: route.site_name,
      domain_rows: buildDomainRows(route),
      enabled: route.enabled,
      redirect_http: route.redirect_http,
    },
  });

  useEffect(() => {
    form.reset({
      site_name: route.site_name,
      domain_rows: buildDomainRows(route),
      enabled: route.enabled,
      redirect_http: route.redirect_http,
    });
  }, [form, route]);

  const selectedCertificateIDs = normalizeSelectedCertificateIDs(
    form.watch('domain_rows'),
  );

  return (
    <ConfigSectionShell
      title="域名设置"
      description="在一个列表里同时维护域名、证书和 HTTPS 跳转。保存时会自动汇总站点证书集合。"
      formId="proxy-route-domains-form"
      saving={saving}
    >
      <form
        id="proxy-route-domains-form"
        className="space-y-5"
        onSubmit={form.handleSubmit((values) => {
          const domains = values.domain_rows
            .map((item) => item.domain.trim().toLowerCase())
            .filter(Boolean);
          const domainCertIDs = buildDomainCertificateIDs(values.domain_rows);
          const certIDs = normalizeSelectedCertificateIDs(values.domain_rows);

          onSave(
            buildPayloadFromRoute(route, {
              site_name: values.site_name.trim(),
              domain: domains[0],
              domains,
              enabled: values.enabled,
              enable_https: certIDs.length > 0,
              cert_id: certIDs[0] ?? null,
              cert_ids: certIDs,
              domain_cert_ids: domainCertIDs,
              redirect_http: certIDs.length > 0 ? values.redirect_http : false,
            }),
            { message: '域名设置已保存。' },
          );
        })}
      >
        <ToggleField
          label="启用站点"
          description="关闭后会保留配置，但不会参与发布。"
          checked={form.watch('enabled')}
          onChange={(checked) =>
            form.setValue('enabled', checked, { shouldDirty: true })
          }
        />

        <ResourceField
          label="站点标识"
          hint="建议使用稳定、可读的业务标识，不必与域名完全一致。"
          error={form.formState.errors.site_name?.message}
        >
          <ResourceInput
            placeholder="marketing-site"
            {...form.register('site_name')}
          />
        </ResourceField>

        <ResourceField
          label="域名列表"
          hint="每行配置一个域名。可为不同域名选择不同证书，相同证书也可以重复选择。"
          error={
            form.formState.errors.domain_rows?.message as string | undefined
          }
          container="div"
        >
          <Controller
            control={form.control}
            name="domain_rows"
            render={({ field }) => (
              <DomainListInput
                rows={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                suggestionSources={suggestionSources}
                certificates={certificates}
              />
            )}
          />
        </ResourceField>

        <ToggleField
          label="HTTP 自动跳转到 HTTPS"
          description={
            selectedCertificateIDs.length > 0
              ? '开启后会额外生成 80 端口重定向规则。'
              : '至少为一个域名选择证书后才能启用。'
          }
          checked={form.watch('redirect_http')}
          disabled={selectedCertificateIDs.length === 0}
          onChange={(checked) =>
            form.setValue('redirect_http', checked, { shouldDirty: true })
          }
        />
      </form>
    </ConfigSectionShell>
  );
}

function RateLimitSection({
  route,
  saving,
  onSave,
}: {
  route: ProxyRouteItem;
  saving: boolean;
  onSave: SaveHandler;
}) {
  const form = useForm<RateLimitValues>({
    resolver: zodResolver(rateLimitSchema),
    defaultValues: {
      limit_conn_per_server: route.limit_conn_per_server
        ? String(route.limit_conn_per_server)
        : '',
      limit_conn_per_ip: route.limit_conn_per_ip
        ? String(route.limit_conn_per_ip)
        : '',
      limit_rate: route.limit_rate || '',
    },
  });

  useEffect(() => {
    form.reset({
      limit_conn_per_server: route.limit_conn_per_server
        ? String(route.limit_conn_per_server)
        : '',
      limit_conn_per_ip: route.limit_conn_per_ip
        ? String(route.limit_conn_per_ip)
        : '',
      limit_rate: route.limit_rate || '',
    });
  }, [form, route]);

  return (
    <ConfigSectionShell
      title="流量限制"
      description="站点限流，空值或 0 表示关闭。"
      formId="proxy-route-limits-form"
      saving={saving}
    >
      <form
        id="proxy-route-limits-form"
        className="grid gap-5 md:grid-cols-2"
        onSubmit={form.handleSubmit((values) => {
          onSave(
            buildPayloadFromRoute(route, {
              limit_conn_per_server: Number(
                values.limit_conn_per_server.trim() || '0',
              ),
              limit_conn_per_ip: Number(values.limit_conn_per_ip.trim() || '0'),
              limit_rate: normalizeLimitRate(values.limit_rate),
            }),
            { message: '流量限制已保存。' },
          );
        })}
      >
        <ResourceField
          label="并发限制"
          hint="限制当前站点最大并发连接数。"
          error={form.formState.errors.limit_conn_per_server?.message}
        >
          <ResourceInput
            placeholder="120"
            {...form.register('limit_conn_per_server')}
          />
        </ResourceField>

        <ResourceField
          label="单 IP 限制"
          hint="限制单个 IP 的最大并发数。"
          error={form.formState.errors.limit_conn_per_ip?.message}
        >
          <ResourceInput
            placeholder="12"
            {...form.register('limit_conn_per_ip')}
          />
        </ResourceField>

        <ResourceField
          label="限速"
          hint="限制单请求带宽，例如 512k 或 1m。"
          error={form.formState.errors.limit_rate?.message}
          className="md:col-span-2"
        >
          <ResourceInput
            placeholder="512k/1m"
            {...form.register('limit_rate')}
          />
        </ResourceField>
      </form>
    </ConfigSectionShell>
  );
}

function ReverseProxySection({
  route,
  saving,
  onSave,
}: {
  route: ProxyRouteItem;
  saving: boolean;
  onSave: SaveHandler;
}) {
  const tunnelsQuery = useQuery({
    queryKey: ['nodes'],
    queryFn: getNodes,
  });
  const pagesProjectsQuery = useQuery({
    queryKey: ['pages-projects'],
    queryFn: getPagesProjects,
  });

  const tunnelClients = (tunnelsQuery.data ?? []).filter(
    (node) => node.node_type === 'tunnel_client',
  );
  const pagesProjects = (pagesProjectsQuery.data ?? []).filter(
    (project) => project.enabled && project.active_deployment_id,
  );

  const form = useForm<ReverseProxyValues>({
    resolver: zodResolver(reverseProxySchema),
    defaultValues: {
      upstream_type: route.upstream_type || 'direct',
      origin_urls_text: route.upstream_list.join('\n'),
      origin_host: route.origin_host || '',
      tunnel_id: route.tunnel_node_id ? String(route.tunnel_node_id) : '',
      tunnel_target_addr: route.tunnel_target_addr || '',
      tunnel_target_protocol:
        (route.tunnel_target_protocol as 'http' | 'https') || 'http',
      pages_project_id: route.pages_project_id
        ? String(route.pages_project_id)
        : '',
      custom_headers_text: customHeadersToText(route.custom_header_list),
      remark: route.remark || '',
    },
  });

  useEffect(() => {
    form.reset({
      upstream_type: route.upstream_type || 'direct',
      origin_urls_text: route.upstream_list.join('\n'),
      origin_host: route.origin_host || '',
      tunnel_id: route.tunnel_node_id ? String(route.tunnel_node_id) : '',
      tunnel_target_addr: route.tunnel_target_addr || '',
      tunnel_target_protocol:
        (route.tunnel_target_protocol as 'http' | 'https') || 'http',
      pages_project_id: route.pages_project_id
        ? String(route.pages_project_id)
        : '',
      custom_headers_text: customHeadersToText(route.custom_header_list),
      remark: route.remark || '',
    });
  }, [form, route]);

  return (
    <ConfigSectionShell
      title="回源代理"
      description="配置请求回源上游的策略与地址。"
      formId="proxy-route-proxy-form"
      saving={saving}
    >
      <form
        id="proxy-route-proxy-form"
        className="space-y-5"
        onSubmit={form.handleSubmit((values) => {
          let originUrl = '';
          let originScheme: 'http' | 'https' = 'http';
          let originAddress = '';
          let originPort = '';
          let originUri = '';
          let upstreams: string[] = [];

          if (values.upstream_type === 'direct') {
            const { urls } = parseOriginUrls(values.origin_urls_text);
            const primaryOrigin = parseOriginUrl(urls[0]);
            originUrl = urls[0];
            originScheme = primaryOrigin.scheme;
            originAddress = primaryOrigin.address;
            originPort = primaryOrigin.port;
            originUri = primaryOrigin.uri;
            upstreams = urls.slice(1);
          } else if (values.upstream_type === 'tunnel') {
            originUrl = `${values.tunnel_target_protocol}://${values.tunnel_target_addr}`;
            originScheme = values.tunnel_target_protocol as 'http' | 'https';
            originAddress = values.tunnel_target_addr || '';
          } else {
            originUrl = 'http://127.0.0.1';
            originScheme = 'http';
            originAddress = '127.0.0.1';
            originPort = '80';
          }

          const { headers } = parseCustomHeadersText(
            values.custom_headers_text,
          );

          onSave(
            buildPayloadFromRoute(route, {
              origin_id: null,
              origin_url: originUrl,
              origin_scheme: originScheme,
              origin_address: originAddress,
              origin_port: originPort,
              origin_uri: originUri,
              origin_host: values.origin_host.trim(),
              upstreams: upstreams,
              custom_headers: headers,
              remark: values.remark.trim(),
              upstream_type: values.upstream_type,
              tunnel_node_id:
                values.upstream_type === 'tunnel' && values.tunnel_id
                  ? Number(values.tunnel_id)
                  : null,
              tunnel_target_addr:
                values.upstream_type === 'tunnel'
                  ? values.tunnel_target_addr
                  : '',
              tunnel_target_protocol:
                values.upstream_type === 'tunnel'
                  ? values.tunnel_target_protocol
                  : '',
              pages_project_id:
                values.upstream_type === 'pages' && values.pages_project_id
                  ? Number(values.pages_project_id)
                  : null,
            }),
            { message: '反向代理设置已保存。' },
          );
        })}
      >
        <div className="space-y-3">
          <label className="block text-sm font-medium text-[var(--foreground-primary)]">
            回源方式
          </label>
          <div className="flex gap-4">
            <label className="flex cursor-pointer items-center gap-2 text-sm text-[var(--foreground-primary)]">
              <input
                type="radio"
                value="direct"
                {...form.register('upstream_type')}
                className="h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              直连上游
            </label>
            <label className="flex cursor-pointer items-center gap-2 text-sm text-[var(--foreground-primary)]">
              <input
                type="radio"
                value="tunnel"
                {...form.register('upstream_type')}
                className="h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              内网穿透 (Tunnel)
            </label>
            <label className="flex cursor-pointer items-center gap-2 text-sm text-[var(--foreground-primary)]">
              <input
                type="radio"
                value="pages"
                {...form.register('upstream_type')}
                className="h-4 w-4 border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              Pages 静态站点
            </label>
          </div>
        </div>

        {form.watch('upstream_type') === 'direct' ? (
          <ResourceField
            label="上游地址"
            hint="每行一个完整 URL。第一行作为主回源，多上游模式请保持相同协议且不要包含 path 或 query。"
            error={form.formState.errors.origin_urls_text?.message}
          >
            <ResourceTextarea
              aria-label="上游地址"
              className="min-h-40"
              placeholder={
                'https://origin-a.internal:443\nhttps://origin-b.internal:443'
              }
              {...form.register('origin_urls_text')}
            />
          </ResourceField>
        ) : form.watch('upstream_type') === 'tunnel' ? (
          <div className="space-y-4 rounded-xl border border-[var(--border-default)] bg-[var(--surface-muted)] p-4">
            <ResourceField
              label="选择内网穿透隧道"
              hint="将请求转发到该隧道连接的客户端节点。"
              error={form.formState.errors.tunnel_id?.message}
            >
              <select
                {...form.register('tunnel_id')}
                className="block w-full rounded-xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-2.5 text-sm text-[var(--foreground-primary)] placeholder-[var(--foreground-muted)] transition outline-none focus:border-[var(--border-strong)] focus:ring-1 focus:ring-[var(--border-strong)]"
              >
                <option value="">请选择...</option>
                {tunnelClients.map((tunnel) => (
                  <option key={tunnel.id} value={tunnel.id}>
                    {tunnel.name} (
                    {tunnel.status === 'online' ? '在线' : '离线'})
                  </option>
                ))}
              </select>
            </ResourceField>

            <ResourceField
              label="内网服务协议"
              hint="转发到内网服务时使用的协议。"
              error={form.formState.errors.tunnel_target_protocol?.message}
            >
              <select
                {...form.register('tunnel_target_protocol')}
                className="block w-full rounded-xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-2.5 text-sm text-[var(--foreground-primary)] placeholder-[var(--foreground-muted)] transition outline-none focus:border-[var(--border-strong)] focus:ring-1 focus:ring-[var(--border-strong)]"
              >
                <option value="http">HTTP</option>
                <option value="https">HTTPS</option>
              </select>
            </ResourceField>

            <ResourceField
              label="内网服务地址"
              hint="例如: 127.0.0.1:8080 或 192.168.1.10:80"
              error={form.formState.errors.tunnel_target_addr?.message}
            >
              <ResourceInput
                placeholder="127.0.0.1:8080"
                {...form.register('tunnel_target_addr')}
              />
            </ResourceField>
          </div>
        ) : (
          <div className="space-y-4 rounded-xl border border-[var(--border-default)] bg-[var(--surface-muted)] p-4">
            <ResourceField
              label="选择 Pages 项目"
              hint="仅显示已启用且已有激活部署的 Pages 项目。"
              error={form.formState.errors.pages_project_id?.message}
            >
              <select
                {...form.register('pages_project_id')}
                className="block w-full rounded-xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-2.5 text-sm text-[var(--foreground-primary)] placeholder-[var(--foreground-muted)] transition outline-none focus:border-[var(--border-strong)] focus:ring-1 focus:ring-[var(--border-strong)]"
              >
                <option value="">请选择...</option>
                {pagesProjects.map((project) => (
                  <option key={project.id} value={project.id}>
                    {project.name} ({project.slug})
                  </option>
                ))}
              </select>
            </ResourceField>
          </div>
        )}

        <ResourceField
          label="Origin Host Header"
          hint="留空时默认透传访问域名 $host。"
          error={form.formState.errors.origin_host?.message}
        >
          <ResourceInput
            placeholder="origin.example.internal"
            {...form.register('origin_host')}
          />
        </ResourceField>

        <ResourceField
          label="自定义请求头"
          hint="每行一条，格式为 Key: Value。"
          error={form.formState.errors.custom_headers_text?.message}
        >
          <ResourceTextarea
            className="min-h-32"
            placeholder={'X-Trace-Id: $request_id\nX-Site: marketing'}
            {...form.register('custom_headers_text')}
          />
        </ResourceField>

        <ResourceField
          label="备注"
          error={form.formState.errors.remark?.message}
        >
          <ResourceTextarea
            placeholder="例如：多活回源，优先使用上海入口"
            {...form.register('remark')}
          />
        </ResourceField>
      </form>
    </ConfigSectionShell>
  );
}

function CacheSection({
  route,
  saving,
  onSave,
}: {
  route: ProxyRouteItem;
  saving: boolean;
  onSave: SaveHandler;
}) {
  const form = useForm<CacheValues>({
    resolver: zodResolver(cacheSchema),
    defaultValues: {
      cache_enabled: route.cache_enabled,
      cache_policy: (route.cache_policy ||
        'url') as CacheValues['cache_policy'],
      cache_rules_text: route.cache_rule_list.join('\n'),
    },
  });

  useEffect(() => {
    form.reset({
      cache_enabled: route.cache_enabled,
      cache_policy: (route.cache_policy ||
        'url') as CacheValues['cache_policy'],
      cache_rules_text: route.cache_rule_list.join('\n'),
    });
  }, [form, route]);

  const watchedEnabled = form.watch('cache_enabled');
  const watchedPolicy = form.watch('cache_policy');

  return (
    <ConfigSectionShell
      title="缓存"
      description="保留现有安全绕过逻辑，只对当前站点生效。"
      formId="proxy-route-cache-form"
      saving={saving}
    >
      <form
        id="proxy-route-cache-form"
        className="space-y-5"
        onSubmit={form.handleSubmit((values) => {
          const rules = linesFromTextarea(values.cache_rules_text);
          onSave(
            buildPayloadFromRoute(route, {
              cache_enabled: values.cache_enabled,
              cache_policy: values.cache_enabled ? values.cache_policy : 'url',
              cache_rules:
                values.cache_enabled && values.cache_policy !== 'url'
                  ? rules
                  : [],
            }),
            { message: '缓存设置已保存。' },
          );
        })}
      >
        <ToggleField
          label="启用站点缓存"
          description="系统仍会自动绕过非 GET、带 Authorization 或常见登录态 Cookie 的请求。"
          checked={watchedEnabled}
          onChange={(checked) =>
            form.setValue('cache_enabled', checked, { shouldDirty: true })
          }
        />

        <ResourceField label="缓存策略">
          <ResourceSelect
            disabled={!watchedEnabled}
            {...form.register('cache_policy')}
          >
            <option value="url">按 URL 缓存</option>
            <option value="suffix">按后缀缓存</option>
            <option value="path_prefix">按路径前缀缓存</option>
            <option value="path_exact">按精确路径缓存</option>
          </ResourceSelect>
        </ResourceField>

        <ResourceField
          label="缓存规则"
          error={form.formState.errors.cache_rules_text?.message}
          hint={
            watchedPolicy === 'suffix'
              ? '每行一个后缀，例如 jpg、css、js。'
              : watchedPolicy === 'path_prefix'
                ? '每行一个路径前缀，例如 /assets、/static。'
                : watchedPolicy === 'path_exact'
                  ? '每行一个精确路径，例如 /robots.txt。'
                  : '按 URL 缓存时无需额外规则。'
          }
        >
          <ResourceTextarea
            disabled={!watchedEnabled || watchedPolicy === 'url'}
            className="min-h-32"
            placeholder={
              watchedPolicy === 'suffix'
                ? 'jpg\ncss\njs'
                : watchedPolicy === 'path_prefix'
                  ? '/assets\n/static'
                  : watchedPolicy === 'path_exact'
                    ? '/robots.txt\n/manifest.json'
                    : '按 URL 缓存时无需额外规则'
            }
            {...form.register('cache_rules_text')}
          />
        </ResourceField>
      </form>
    </ConfigSectionShell>
  );
}

function WAFBindingSection({ route }: { route: ProxyRouteItem }) {
  const queryClient = useQueryClient();
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);
  const wafQuery = useQuery({
    queryKey: ['waf', 'site-rule-groups', route.id],
    queryFn: () => getWAFSiteRuleGroups(route.id),
  });
  const wafMutation = useMutation({
    mutationFn: (ids: number[]) => replaceWAFSiteRuleGroups(route.id, ids),
    onSuccess: async (result) => {
      setSelectedIDs(result.applied_ids);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['waf', 'site-rule-groups', route.id],
        }),
        queryClient.invalidateQueries({ queryKey: ['waf', 'rule-groups'] }),
        queryClient.invalidateQueries({
          queryKey: ['config-versions', 'diff'],
        }),
      ]);
    },
  });

  useEffect(() => {
    if (wafQuery.data) {
      setSelectedIDs(wafQuery.data.applied_ids);
    }
  }, [wafQuery.data]);

  const selectedSet = useMemo(() => new Set(selectedIDs), [selectedIDs]);

  return (
    <ConfigSectionShell
      title="WAF"
      description="全局规则组始终生效；这里可以为当前网站叠加自定义规则组。"
      formId="proxy-route-waf-form"
      saving={wafMutation.isPending}
    >
      {wafQuery.isLoading ? (
        <LoadingState />
      ) : wafQuery.isError ? (
        <ErrorState
          title="WAF 规则加载失败"
          description={getErrorMessage(wafQuery.error)}
        />
      ) : (
        <form
          id="proxy-route-waf-form"
          className="space-y-5"
          onSubmit={(e) => {
            e.preventDefault();
            wafMutation.mutate(selectedIDs);
          }}
        >
          {wafMutation.isError ? (
            <InlineMessage
              tone="danger"
              message={getErrorMessage(wafMutation.error)}
            />
          ) : null}
          {wafMutation.isSuccess ? (
            <InlineMessage tone="success" message="WAF 规则组已更新。" />
          ) : null}

          {wafQuery.data?.global_rule_group ? (
            <div className="rounded-[26px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-xs font-medium tracking-[0.18em] text-[var(--foreground-muted)] uppercase">
                    Global Rule Group
                  </p>
                  <p className="mt-2 text-sm font-semibold text-[var(--foreground-primary)]">
                    {wafQuery.data.global_rule_group.name}
                  </p>
                </div>
                <span className="rounded-full border border-[var(--border-default)] px-3 py-1.5 text-xs text-[var(--foreground-secondary)]">
                  始终生效
                </span>
              </div>
            </div>
          ) : null}

          <div className="grid gap-3 md:grid-cols-2">
            {(wafQuery.data?.rule_groups ?? []).map((group) => (
              <label
                key={group.id}
                className={cn(
                  'flex cursor-pointer items-start gap-3 rounded-[22px] border p-4 transition',
                  selectedSet.has(group.id)
                    ? 'border-[var(--border-strong)] bg-[var(--accent-soft)]'
                    : 'border-[var(--border-default)] bg-[var(--surface-elevated)] hover:bg-[var(--surface-muted)]',
                )}
              >
                <input
                  type="checkbox"
                  checked={selectedSet.has(group.id)}
                  onChange={(event) => {
                    const checked = event.target.checked;
                    setSelectedIDs((current) =>
                      checked
                        ? [...current, group.id].sort(
                            (left, right) => left - right,
                          )
                        : current.filter((id) => id !== group.id),
                    );
                  }}
                  className="mt-1 h-4 w-4 rounded border-[var(--border-default)] accent-[var(--brand-primary)]"
                />
                <span className="min-w-0">
                  <span className="block text-sm font-semibold text-[var(--foreground-primary)]">
                    {group.name}
                  </span>
                  <span className="mt-1 block text-xs leading-5 text-[var(--foreground-secondary)]">
                    {group.enabled ? '启用中' : '已停用'} ·{' '}
                    {group.ip_whitelist.length +
                      group.ip_blacklist.length +
                      group.country_whitelist.length +
                      group.country_blacklist.length}{' '}
                    条规则
                  </span>
                </span>
              </label>
            ))}
          </div>

          {(wafQuery.data?.rule_groups ?? []).length === 0 ? (
            <EmptyState title="暂无自定义 WAF 规则组" />
          ) : null}
        </form>
      )}
    </ConfigSectionShell>
  );
}

const basicAuthSchema = z
  .object({
    basic_auth_enabled: z.boolean(),
    basic_auth_username: z.string(),
    basic_auth_password: z.string(),
  })
  .superRefine((value, context) => {
    if (value.basic_auth_enabled) {
      if (!value.basic_auth_username.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['basic_auth_username'],
          message: '请输入账号',
        });
      }
      if (!value.basic_auth_password.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['basic_auth_password'],
          message: '请输入密码',
        });
      }
    }
  });

type BasicAuthValues = z.infer<typeof basicAuthSchema>;

function BasicAuthSection({
  route,
  saving,
  onSave,
}: {
  route: ProxyRouteItem;
  saving: boolean;
  onSave: SaveHandler;
}) {
  const form = useForm<BasicAuthValues>({
    resolver: zodResolver(basicAuthSchema),
    defaultValues: {
      basic_auth_enabled: route.basic_auth_enabled,
      basic_auth_username: route.basic_auth_username || '',
      basic_auth_password: route.basic_auth_password || '',
    },
  });

  useEffect(() => {
    form.reset({
      basic_auth_enabled: route.basic_auth_enabled,
      basic_auth_username: route.basic_auth_username || '',
      basic_auth_password: route.basic_auth_password || '',
    });
  }, [form, route]);

  const watchedEnabled = form.watch('basic_auth_enabled');

  return (
    <ConfigSectionShell
      title="认证配置"
      description="配置基础鉴权访问，需要输入账号密码才能访问网站。"
      formId="proxy-route-auth-form"
      saving={saving}
    >
      <form
        id="proxy-route-auth-form"
        className="space-y-5"
        onSubmit={form.handleSubmit((values) => {
          onSave(
            buildPayloadFromRoute(route, {
              basic_auth_enabled: values.basic_auth_enabled,
              basic_auth_username: values.basic_auth_username.trim(),
              basic_auth_password: values.basic_auth_password.trim(),
            }),
            { message: '认证配置已保存。' },
          );
        })}
      >
        <ToggleField
          label="启用 Basic Auth 鉴权"
          description="拦截所有请求，需要输入正确的账号和密码才能访问。"
          checked={watchedEnabled}
          onChange={(checked) =>
            form.setValue('basic_auth_enabled', checked, { shouldDirty: true })
          }
        />

        <ResourceField
          label="账号"
          error={form.formState.errors.basic_auth_username?.message}
        >
          <ResourceInput
            disabled={!watchedEnabled}
            placeholder="admin"
            {...form.register('basic_auth_username')}
          />
        </ResourceField>

        <ResourceField
          label="密码"
          error={form.formState.errors.basic_auth_password?.message}
        >
          <ResourceInput
            disabled={!watchedEnabled}
            type="text"
            placeholder="secret123"
            {...form.register('basic_auth_password')}
          />
        </ResourceField>
      </form>
    </ConfigSectionShell>
  );
}

export function ProxyRouteConfigPage({
  routeId,
  initialSection,
}: {
  routeId: string;
  initialSection?: string;
}) {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);

  const numericRouteID = Number(routeId);
  const currentSection = getWebsiteConfigSection(initialSection);

  const routeQuery = useQuery({
    queryKey: ['proxy-routes', 'detail', numericRouteID],
    queryFn: () => getProxyRoute(numericRouteID),
    enabled: Number.isFinite(numericRouteID) && numericRouteID > 0,
  });
  const certificatesQuery = useQuery({
    queryKey: ['tls-certificates', 'list'],
    queryFn: getTlsCertificates,
  });
  const managedDomainsQuery = useQuery({
    queryKey: ['managed-domains'],
    queryFn: getManagedDomains,
  });

  const saveMutation = useMutation({
    mutationFn: async ({
      payload,
      context,
    }: {
      payload: Parameters<typeof updateProxyRoute>[1];
      context: SaveContext;
    }) => {
      const updatedRoute = await updateProxyRoute(numericRouteID, payload);
      return { updatedRoute, context };
    },
    onSuccess: async ({ updatedRoute, context }) => {
      queryClient.setQueryData(
        ['proxy-routes', 'detail', numericRouteID],
        updatedRoute,
      );
      setFeedback({ tone: 'success', message: context.message });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['proxy-routes'] }),
        queryClient.invalidateQueries({
          queryKey: ['config-versions', 'diff'],
        }),
      ]);
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const route = routeQuery.data;
  const certificates = useMemo(
    () => certificatesQuery.data ?? [],
    [certificatesQuery.data],
  );
  const domainSuggestionSources = useMemo(
    () => [
      ...(route?.domains ?? []),
      ...(managedDomainsQuery.data?.map((item) => item.domain) ?? []),
    ],
    [managedDomainsQuery.data, route?.domains],
  );

  if (!Number.isFinite(numericRouteID) || numericRouteID <= 0) {
    return (
      <EmptyState
        title="缺少站点 ID"
        description="请从站点列表进入配置页面。"
      />
    );
  }

  if (routeQuery.isLoading || certificatesQuery.isLoading) {
    return <LoadingState />;
  }

  if (routeQuery.isError) {
    return (
      <ErrorState
        title="站点详情加载失败"
        description={getErrorMessage(routeQuery.error)}
      />
    );
  }

  if (certificatesQuery.isError) {
    return (
      <ErrorState
        title="证书列表加载失败"
        description={getErrorMessage(certificatesQuery.error)}
      />
    );
  }

  if (!route) {
    return (
      <EmptyState
        title="站点不存在"
        description="该站点可能已被删除，或当前 ID 无法匹配到记录。"
      />
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={route.site_name}
        description={`主域名 ${route.primary_domain}，共 ${route.domain_count} 个域名`}
        action={
          <div className="flex flex-wrap gap-3">
            <Link
              href="/proxy-route"
              className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
            >
              返回列表
            </Link>
            <SecondaryButton
              type="button"
              onClick={() =>
                queryClient.invalidateQueries({
                  queryKey: ['proxy-routes', 'detail', numericRouteID],
                })
              }
            >
              刷新详情
            </SecondaryButton>
          </div>
        }
      />

      {feedback ? (
        <InlineMessage
          tone={feedback.tone}
          message={feedback.message}
          onClear={() => setFeedback(null)}
        />
      ) : null}

      <div className="grid gap-6 xl:grid-cols-[280px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <AppCard title="配置分区">
            <div className="space-y-2">
              {websiteConfigSections.map((section) => {
                const active = section.key === currentSection;
                return (
                  <Link
                    key={section.key}
                    href={`/proxy-route/detail?id=${route.id}&section=${section.key}`}
                    className={cn(
                      'block rounded-2xl border px-4 py-3 transition',
                      active
                        ? 'border-[var(--border-strong)] bg-[var(--accent-soft)]'
                        : 'border-[var(--border-default)] bg-[var(--surface-elevated)] hover:border-[var(--border-strong)]',
                    )}
                  >
                    <p className="text-sm font-medium text-[var(--foreground-primary)]">
                      {section.label}
                    </p>
                    <p className="mt-1 text-xs leading-5 text-[var(--foreground-secondary)]">
                      {section.description}
                    </p>
                  </Link>
                );
              })}
            </div>
          </AppCard>
        </aside>

        <div className="min-w-0 space-y-6">
          {currentSection === 'domains' ? (
            <DomainSettingsSection
              route={route}
              certificates={certificates}
              saving={saveMutation.isPending}
              suggestionSources={domainSuggestionSources}
              onSave={(payload, context) =>
                saveMutation.mutate({ payload, context })
              }
            />
          ) : null}

          {currentSection === 'limits' ? (
            <RateLimitSection
              route={route}
              saving={saveMutation.isPending}
              onSave={(payload, context) =>
                saveMutation.mutate({ payload, context })
              }
            />
          ) : null}

          {currentSection === 'proxy' ? (
            <ReverseProxySection
              route={route}
              saving={saveMutation.isPending}
              onSave={(payload, context) =>
                saveMutation.mutate({ payload, context })
              }
            />
          ) : null}

          {currentSection === 'cache' ? (
            <CacheSection
              route={route}
              saving={saveMutation.isPending}
              onSave={(payload, context) =>
                saveMutation.mutate({ payload, context })
              }
            />
          ) : null}

          {currentSection === 'waf' ? (
            <WAFBindingSection route={route} />
          ) : null}

          {currentSection === 'auth' ? (
            <BasicAuthSection
              route={route}
              saving={saveMutation.isPending}
              onSave={(payload, context) =>
                saveMutation.mutate({ payload, context })
              }
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}
