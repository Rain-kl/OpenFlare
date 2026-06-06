'use client';

import {zodResolver} from '@hookform/resolvers/zod';
import {useMutation, useQuery} from '@tanstack/react-query';
import {useEffect, useMemo} from 'react';
import {Controller, useForm} from 'react-hook-form';
import {z} from 'zod';

import {Drawer} from '@/components/ui/drawer';
import {getManagedDomains} from '@/features/managed-domains/api/managed-domains';
import {getPagesProjects} from '@/features/pages/api/pages';
import {createProxyRoute} from '@/features/proxy-routes/api/proxy-routes';
import {DomainListInput, type DomainListRow,} from '@/features/proxy-routes/components/domain-list-input';
import {
    buildOriginUrl,
    getErrorMessage,
    parseOriginUrl,
    parseOriginUrls,
    validateDomains,
} from '@/features/proxy-routes/helpers';
import {getNodes} from '@/features/nodes/api/nodes';
import type {ProxyRouteItem} from '@/features/proxy-routes/types';
import {getTlsCertificates} from '@/features/tls-certificates/api/tls-certificates';
import {
    PrimaryButton,
    ResourceField,
    ResourceInput,
    ResourceTextarea,
    ToggleField,
} from '@/features/shared/components/resource-primitives';

const domainRowSchema = z.object({
  domain: z.string(),
  certificateId: z.string(),
});

const createWebsiteSchema = z
  .object({
    site_name: z.string().trim().max(255, '站点标识不能超过 255 个字符'),
    domain_rows: z.array(domainRowSchema).min(1),
    upstream_type: z.enum(['direct', 'tunnel', 'pages']),
    origin_urls_text: z.string().trim(),
    tunnel_id: z.string().optional(),
    tunnel_target_addr: z.string().trim().optional(),
    tunnel_target_protocol: z.enum(['http', 'https']).optional(),
    pages_project_id: z.string().optional(),
    enabled: z.boolean(),
    redirect_http: z.boolean(),
    remark: z.string().max(255, '备注不能超过 255 个字符'),
  })
  .superRefine((value, context) => {
    const domains = value.domain_rows
      .map((item) => item.domain.trim().toLowerCase())
      .filter(Boolean);
    const domainError = validateDomains(domains);
    if (domainError) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['domain_rows'],
        message: domainError,
      });
    }

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

type CreateWebsiteFormValues = z.infer<typeof createWebsiteSchema>;

const defaultValues: CreateWebsiteFormValues = {
  site_name: '',
  domain_rows: [{ domain: '', certificateId: '' }],
  upstream_type: 'direct',
  origin_urls_text: '',
  tunnel_id: '',
  tunnel_target_addr: '',
  tunnel_target_protocol: 'http',
  pages_project_id: '',
  enabled: true,
  redirect_http: false,
  remark: '',
};

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

export function ProxyRouteCreateDrawer({
  open,
  onOpenChange,
  onCreated,
  domainSuggestionSources = [],
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (route: ProxyRouteItem) => void;
  domainSuggestionSources?: string[];
}) {
  const form = useForm<CreateWebsiteFormValues>({
    resolver: zodResolver(createWebsiteSchema),
    defaultValues,
  });
  const managedDomainsQuery = useQuery({
    queryKey: ['managed-domains'],
    queryFn: getManagedDomains,
    enabled: open,
  });
  const certificatesQuery = useQuery({
    queryKey: ['tls-certificates', 'list'],
    queryFn: getTlsCertificates,
    enabled: open,
  });
  const tunnelsQuery = useQuery({
    queryKey: ['nodes'],
    queryFn: getNodes,
    enabled: open,
  });
  const pagesProjectsQuery = useQuery({
    queryKey: ['pages-projects'],
    queryFn: getPagesProjects,
    enabled: open,
  });
  const tunnelClients = (tunnelsQuery.data ?? []).filter(
    (node) => node.node_type === 'tunnel_client',
  );
  const pagesProjects = (pagesProjectsQuery.data ?? []).filter(
    (project) => project.enabled && project.active_deployment_id,
  );

  const combinedDomainSuggestions = useMemo(
    () => [
      ...domainSuggestionSources,
      ...(managedDomainsQuery.data?.map((item) => item.domain) ?? []),
    ],
    [domainSuggestionSources, managedDomainsQuery.data],
  );
  const selectedCertificateIDs = normalizeSelectedCertificateIDs(
    form.watch('domain_rows'),
  );

  const createMutation = useMutation({
    mutationFn: async (values: CreateWebsiteFormValues) => {
      const domains = values.domain_rows
        .map((item) => item.domain.trim().toLowerCase())
        .filter(Boolean);
      const domainCertIDs = buildDomainCertificateIDs(values.domain_rows);
      const selectedCertIDs = normalizeSelectedCertificateIDs(
        values.domain_rows,
      );

      let originUrl = '';
      const originHost = '';
      let upstreams: string[] = [];

      if (values.upstream_type === 'direct') {
        const { urls } = parseOriginUrls(values.origin_urls_text);
        const primaryOrigin = parseOriginUrl(urls[0]);
        originUrl = buildOriginUrl(
          primaryOrigin.scheme,
          primaryOrigin.address,
          primaryOrigin.port,
          primaryOrigin.uri,
        );
        upstreams = urls.slice(1);
      } else if (values.upstream_type === 'tunnel') {
        originUrl = `${values.tunnel_target_protocol}://${values.tunnel_target_addr}`;
      } else {
        originUrl = 'http://127.0.0.1';
      }

      return createProxyRoute({
        site_name: values.site_name.trim() || domains[0],
        domain: domains[0],
        domains,
        origin_id: null,
        origin_url: originUrl,
        origin_scheme:
          values.upstream_type === 'direct'
            ? parseOriginUrl(originUrl).scheme
            : 'http',
        origin_address:
          values.upstream_type === 'direct'
            ? parseOriginUrl(originUrl).address
            : values.tunnel_target_addr || '127.0.0.1',
        origin_port:
          values.upstream_type === 'direct'
            ? parseOriginUrl(originUrl).port
            : '80',
        origin_uri:
          values.upstream_type === 'direct'
            ? parseOriginUrl(originUrl).uri
            : '',
        origin_host: originHost,
        upstreams,
        enabled: values.enabled,
        enable_https: selectedCertIDs.length > 0,
        cert_id: selectedCertIDs.length > 0 ? selectedCertIDs[0] : null,
        cert_ids: selectedCertIDs,
        domain_cert_ids: domainCertIDs,
        redirect_http:
          selectedCertIDs.length > 0 ? values.redirect_http : false,
        limit_conn_per_server: 0,
        limit_conn_per_ip: 0,
        limit_rate: '',
        cache_enabled: false,
        cache_policy: 'url',
        cache_rules: [],
        custom_headers: [],
        pow_enabled: false,
        pow_config: '{}',
        basic_auth_enabled: false,
        remark: values.remark.trim(),
        upstream_type: values.upstream_type,
        tunnel_node_id:
          values.upstream_type === 'tunnel' && values.tunnel_id
            ? Number(values.tunnel_id)
            : null,
        tunnel_target_addr:
          values.upstream_type === 'tunnel' ? values.tunnel_target_addr : '',
        tunnel_target_protocol:
          values.upstream_type === 'tunnel'
            ? values.tunnel_target_protocol
            : '',
        pages_project_id:
          values.upstream_type === 'pages' && values.pages_project_id
            ? Number(values.pages_project_id)
            : null,
      });
    },
    onSuccess: (route) => {
      form.reset(defaultValues);
      onOpenChange(false);
      onCreated(route);
    },
  });

  useEffect(() => {
    if (!open) {
      form.reset(defaultValues);
    }
  }, [form, open]);

  return (
    <Drawer
      open={open}
      onOpenChange={onOpenChange}
      direction="right"
      title="新建规则"
      footer={
        <div className="flex items-center justify-end gap-3">
          <PrimaryButton
            type="submit"
            form="create-website-form"
            disabled={createMutation.isPending}
          >
            {createMutation.isPending ? '创建中...' : '创建'}
          </PrimaryButton>
        </div>
      }
    >
      <form
        id="create-website-form"
        className="space-y-5"
        onSubmit={form.handleSubmit((values) => createMutation.mutate(values))}
      >
        <ResourceField
          label="站点标识"
          hint="可选，留空时会自动使用第一个域名。"
          error={form.formState.errors.site_name?.message}
        >
          <ResourceInput
            {...form.register('site_name')}
            placeholder="marketing-site"
          />
        </ResourceField>

        <ResourceField
          label="域名列表"
          hint="每行配置一个域名，可按需为该行选择证书。保存时会自动汇总站点证书集合。"
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
                suggestionSources={combinedDomainSuggestions}
                certificates={certificatesQuery.data ?? []}
              />
            )}
          />
        </ResourceField>

        <ToggleField
          label="HTTP 自动跳转到 HTTPS"
          description={
            selectedCertificateIDs.length > 0
              ? '勾选后会额外生成 80 端口重定向规则。'
              : '至少为一个域名选择证书后才能启用。'
          }
          checked={form.watch('redirect_http')}
          disabled={selectedCertificateIDs.length === 0}
          onChange={(checked) =>
            form.setValue('redirect_http', checked, { shouldDirty: true })
          }
        />

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

        <ToggleField
          label="创建后立即启用"
          description="关闭后站点会以草稿保存，后续仍可继续编辑。"
          checked={form.watch('enabled')}
          onChange={(checked) =>
            form.setValue('enabled', checked, { shouldDirty: true })
          }
        />

        <ResourceField
          label="备注"
          error={form.formState.errors.remark?.message}
        >
          <ResourceTextarea {...form.register('remark')} />
        </ResourceField>

        {createMutation.isError ? (
          <p className="text-sm text-[var(--status-danger-foreground)]">
            {getErrorMessage(createMutation.error)}
          </p>
        ) : null}
      </form>
    </Drawer>
  );
}
