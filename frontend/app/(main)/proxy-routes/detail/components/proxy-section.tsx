'use client';

import { useEffect } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import type { ProxyRouteItem } from '@/lib/services/openflare';
import { NodeService, PagesService } from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import {
  customHeadersToText,
  parseCustomHeadersText,
  parseOriginUrl,
  parseOriginUrls,
  validateOriginHost,
} from '../../components/helpers';
import { proxyRouteFormIds } from '../helpers';
import { useRouteSectionSave } from '../hooks/use-route-section-save';
import { SectionShell } from './section-shell';

type ReverseProxyValues = {
  upstream_type: 'direct' | 'tunnel' | 'pages';
  origin_urls_text: string;
  origin_host: string;
  tunnel_id?: string;
  tunnel_target_addr?: string;
  tunnel_target_protocol?: 'http' | 'https';
  pages_project_id?: string;
  custom_headers_text: string;
};

interface ProxySectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

export function ProxySection({
  route,
  onRouteUpdate,
  onSavingChange,
}: ProxySectionProps) {
  const t = useTranslations('proxyRoutes');
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
    })
    .superRefine((value, context) => {
      if (value.upstream_type === 'direct') {
        if (!value.origin_urls_text.trim()) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['origin_urls_text'],
            message: t('validation.enterAtLeastOneUpstream'),
          });
        } else {
          const { error } = parseOriginUrls(value.origin_urls_text, t);
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
            message: t('validation.selectTunnel'),
          });
        }
        if (!value.tunnel_target_addr) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['tunnel_target_addr'],
            message: t('validation.enterTunnelTarget'),
          });
        }
      } else if (!value.pages_project_id) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['pages_project_id'],
          message: t('validation.selectPagesProject'),
        });
      }

      const originHostError = validateOriginHost(value.origin_host, t);
      if (originHostError) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['origin_host'],
          message: originHostError,
        });
      }

      const { error: headerError } = parseCustomHeadersText(
        value.custom_headers_text,
        t,
      );
      if (headerError) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['custom_headers_text'],
          message: headerError,
        });
      }
    });
  const { saving, save } = useRouteSectionSave(
    route,
    onRouteUpdate,
    onSavingChange,
  );

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
      pages_project_id: route.pages_project_id
        ? String(route.pages_project_id)
        : '',
      custom_headers_text: customHeadersToText(route.custom_header_list),
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
    });
  }, [form, route]);

  const upstreamType = form.watch('upstream_type');

  return (
    <SectionShell
      title={t('reverseProxy')}
      description={t('reverseProxyDesc')}
      formId={proxyRouteFormIds.proxy}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.proxy}
          className='space-y-5'
          onSubmit={form.handleSubmit(async (values) => {
            let originUrl = '';
            let originScheme: 'http' | 'https' = 'http';
            let originAddress = '';
            let originPort = '';
            let originUri = '';
            let upstreams: string[] = [];

            if (values.upstream_type === 'direct') {
              const { urls } = parseOriginUrls(values.origin_urls_text, t);
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
              t,
            );

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
              },
              t('proxySaved'),
            );
          })}
        >
          <FormField
            control={form.control}
            name='upstream_type'
            render={({ field }) => (
              <FormItem className='space-y-3'>
                <FormLabel>{t('upstreamType')}</FormLabel>
                <div className='flex flex-wrap gap-4'>
                  {(
                    [
                      ['direct', t('upstreamDirect')],
                      ['tunnel', t('upstreamTunnel')],
                      ['pages', t('upstreamPages')],
                    ] as const
                  ).map(([value, label]) => (
                    <label
                      key={value}
                      className='flex cursor-pointer items-center gap-2 text-sm'
                    >
                      <input
                        type='radio'
                        value={value}
                        checked={field.value === value}
                        onChange={() => field.onChange(value)}
                        className='size-4 accent-primary'
                      />
                      <Label className='font-normal'>{label}</Label>
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
              name='origin_urls_text'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('upstreamAddresses')}</FormLabel>
                  <FormControl>
                    <Textarea
                      className='min-h-40 font-mono text-xs'
                      placeholder={
                        'https://origin-a.internal:443\nhttps://origin-b.internal:443'
                      }
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>{t('upstreamUrlsHint')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          ) : null}

          {upstreamType === 'tunnel' ? (
            <div className='space-y-4 rounded-lg border border-dashed bg-muted/30 p-4'>
              <FormField
                control={form.control}
                name='tunnel_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('selectTunnel')}</FormLabel>
                    <Select
                      value={field.value || 'none'}
                      onValueChange={(value) =>
                        field.onChange(value === 'none' ? '' : value)
                      }
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t('pleaseSelect')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='none'>
                          {t('pleaseSelect')}
                        </SelectItem>
                        {tunnelClients.map((tunnel) => (
                          <SelectItem key={tunnel.id} value={String(tunnel.id)}>
                            {tunnel.name} (
                            {tunnel.status === 'online'
                              ? t('online')
                              : t('offline')}
                            )
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormDescription>{t('tunnelForwardHint')}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='tunnel_target_protocol'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('tunnelProtocol')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='http'>HTTP</SelectItem>
                        <SelectItem value='https'>HTTPS</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='tunnel_target_addr'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('tunnelAddress')}</FormLabel>
                    <FormControl>
                      <Input placeholder='127.0.0.1:8080' {...field} />
                    </FormControl>
                    <FormDescription>{t('tunnelAddressHint')}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          ) : null}

          {upstreamType === 'pages' ? (
            <div className='rounded-lg border border-dashed bg-muted/30 p-4'>
              <FormField
                control={form.control}
                name='pages_project_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('selectPagesProject')}</FormLabel>
                    <Select
                      value={field.value || 'none'}
                      onValueChange={(value) =>
                        field.onChange(value === 'none' ? '' : value)
                      }
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t('pleaseSelect')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='none'>
                          {t('pleaseSelect')}
                        </SelectItem>
                        {pagesProjects.map((project) => (
                          <SelectItem
                            key={project.id}
                            value={String(project.id)}
                          >
                            {project.name} ({project.slug})
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormDescription>{t('pagesProjectHint')}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          ) : null}

          <FormField
            control={form.control}
            name='origin_host'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Origin Host Header</FormLabel>
                <FormControl>
                  <Input placeholder='origin.example.internal' {...field} />
                </FormControl>
                <FormDescription>{t('originHostHint')}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='custom_headers_text'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('customHeaders')}</FormLabel>
                <FormControl>
                  <Textarea
                    className='min-h-32 font-mono text-xs'
                    placeholder={'X-Trace-Id: $request_id\nX-Site: marketing'}
                    {...field}
                  />
                </FormControl>
                <FormDescription>{t('customHeadersHint')}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </SectionShell>
  );
}
