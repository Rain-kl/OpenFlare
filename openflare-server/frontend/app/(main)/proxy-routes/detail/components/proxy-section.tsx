'use client';

import {useEffect} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {useQuery} from '@tanstack/react-query';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from '@/components/ui/select';
import {Textarea} from '@/components/ui/textarea';
import type {ProxyRouteItem} from '@/lib/services/openflare';
import {NodeService, PagesService} from '@/lib/services/openflare';

import {
  customHeadersToText,
  parseCustomHeadersText,
  parseOriginUrl,
  parseOriginUrls,
  validateOriginHost,
} from '../../components/helpers';
import {proxyRouteFormIds} from '../helpers';
import {useRouteSectionSave} from '../hooks/use-route-section-save';
import {SectionShell} from './section-shell';

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

    const { error: headerError } = parseCustomHeadersText(value.custom_headers_text);
    if (headerError) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['custom_headers_text'],
        message: headerError,
      });
    }
  });

type ReverseProxyValues = z.infer<typeof reverseProxySchema>;

interface ProxySectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

export function ProxySection({ route, onRouteUpdate, onSavingChange }: ProxySectionProps) {
  const { saving, save } = useRouteSectionSave(route, onRouteUpdate, onSavingChange);

  const tunnelsQuery = useQuery({
    queryKey: ['openflare', 'nodes'],
    queryFn: () => NodeService.listNodes(),
  });

  const pagesProjectsQuery = useQuery({
    queryKey: ['openflare', 'pages-projects'],
    queryFn: () => PagesService.listProjects(),
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
      pages_project_id: route.pages_project_id ? String(route.pages_project_id) : '',
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
      pages_project_id: route.pages_project_id ? String(route.pages_project_id) : '',
      custom_headers_text: customHeadersToText(route.custom_header_list),
      remark: route.remark || '',
    });
  }, [form, route]);

  const upstreamType = form.watch('upstream_type');

  return (
    <SectionShell
      title="反向代理"
      description="配置请求回源上游的策略与地址。"
      formId={proxyRouteFormIds.proxy}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.proxy}
          className="space-y-5"
          onSubmit={form.handleSubmit(async (values) => {
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

            const { headers } = parseCustomHeadersText(values.custom_headers_text);

            await save(
              {
                origin_id: null,
                origin_url: originUrl,
                origin_scheme: originScheme,
                origin_address: originAddress,
                origin_port: originPort,
                origin_uri: originUri,
                origin_host: values.origin_host.trim(),
                upstreams,
                custom_headers: headers,
                remark: values.remark.trim(),
                upstream_type: values.upstream_type,
                tunnel_node_id:
                  values.upstream_type === 'tunnel' && values.tunnel_id
                    ? Number(values.tunnel_id)
                    : null,
                tunnel_target_addr:
                  values.upstream_type === 'tunnel' ? values.tunnel_target_addr : '',
                tunnel_target_protocol:
                  values.upstream_type === 'tunnel' ? values.tunnel_target_protocol : '',
                pages_project_id:
                  values.upstream_type === 'pages' && values.pages_project_id
                    ? Number(values.pages_project_id)
                    : null,
              },
              '反向代理设置已保存',
            );
          })}
        >
          <FormField
            control={form.control}
            name="upstream_type"
            render={({ field }) => (
              <FormItem className="space-y-3">
                <FormLabel>回源方式</FormLabel>
                <div className="flex flex-wrap gap-4">
                  {(
                    [
                      ['direct', '直连上游'],
                      ['tunnel', '内网穿透 (Tunnel)'],
                      ['pages', 'Pages 静态站点'],
                    ] as const
                  ).map(([value, label]) => (
                    <label key={value} className="flex cursor-pointer items-center gap-2 text-sm">
                      <input
                        type="radio"
                        value={value}
                        checked={field.value === value}
                        onChange={() => field.onChange(value)}
                        className="size-4 accent-primary"
                      />
                      <Label className="font-normal">{label}</Label>
                    </label>
                  ))}
                </div>
                <FormMessage />
              </FormItem>
            )}
          />

          {upstreamType === 'direct' ? (
            <FormField
              control={form.control}
              name="origin_urls_text"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>上游地址</FormLabel>
                  <FormControl>
                    <Textarea
                      className="min-h-40 font-mono text-xs"
                      placeholder={
                        'https://origin-a.internal:443\nhttps://origin-b.internal:443'
                      }
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    每行一个完整 URL。第一行作为主回源，多上游模式请保持相同协议且不要包含 path 或 query。
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          ) : null}

          {upstreamType === 'tunnel' ? (
            <div className="space-y-4 rounded-lg border border-dashed bg-muted/30 p-4">
              <FormField
                control={form.control}
                name="tunnel_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>选择内网穿透隧道</FormLabel>
                    <Select value={field.value || 'none'} onValueChange={(value) => field.onChange(value === 'none' ? '' : value)}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder="请选择..." />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="none">请选择...</SelectItem>
                        {tunnelClients.map((tunnel) => (
                          <SelectItem key={tunnel.id} value={String(tunnel.id)}>
                            {tunnel.name} ({tunnel.status === 'online' ? '在线' : '离线'})
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormDescription>将请求转发到该隧道连接的客户端节点。</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="tunnel_target_protocol"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>内网服务协议</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="http">HTTP</SelectItem>
                        <SelectItem value="https">HTTPS</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="tunnel_target_addr"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>内网服务地址</FormLabel>
                    <FormControl>
                      <Input placeholder="127.0.0.1:8080" {...field} />
                    </FormControl>
                    <FormDescription>例如: 127.0.0.1:8080 或 192.168.1.10:80</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          ) : null}

          {upstreamType === 'pages' ? (
            <div className="rounded-lg border border-dashed bg-muted/30 p-4">
              <FormField
                control={form.control}
                name="pages_project_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>选择 Pages 项目</FormLabel>
                    <Select value={field.value || 'none'} onValueChange={(value) => field.onChange(value === 'none' ? '' : value)}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder="请选择..." />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value="none">请选择...</SelectItem>
                        {pagesProjects.map((project) => (
                          <SelectItem key={project.id} value={String(project.id)}>
                            {project.name} ({project.slug})
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormDescription>仅显示已启用且已有激活部署的 Pages 项目。</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          ) : null}

          <FormField
            control={form.control}
            name="origin_host"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Origin Host Header</FormLabel>
                <FormControl>
                  <Input placeholder="origin.example.internal" {...field} />
                </FormControl>
                <FormDescription>留空时默认透传访问域名 $host。</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="custom_headers_text"
            render={({ field }) => (
              <FormItem>
                <FormLabel>自定义请求头</FormLabel>
                <FormControl>
                  <Textarea
                    className="min-h-32 font-mono text-xs"
                    placeholder={'X-Trace-Id: $request_id\nX-Site: marketing'}
                    {...field}
                  />
                </FormControl>
                <FormDescription>每行一条，格式为 Key: Value。</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="remark"
            render={({ field }) => (
              <FormItem>
                <FormLabel>备注</FormLabel>
                <FormControl>
                  <Textarea placeholder="例如：多活回源，优先使用上海入口" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </SectionShell>
  );
}
