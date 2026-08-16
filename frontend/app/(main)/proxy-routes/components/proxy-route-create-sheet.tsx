'use client';

import { useEffect } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Button } from '@/components/ui/button';
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import type { ProxyRouteItem } from '@/lib/services/openflare';
import {
  NodeService,
  PagesService,
  ProxyRouteService,
  ZoneService,
  zoneQueryKey,
} from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import { listAllZoneDomains, parseOriginUrl, parseOriginUrls } from './helpers';
import { ZoneDomainSelector } from './zone-domain-selector';

type CreateProxyRouteFormValues = {
  site_name: string;
  zone_domain_ids: number[];
  upstream_type: 'direct' | 'tunnel' | 'pages';
  origin_urls_text: string;
  tunnel_id?: string;
  tunnel_target_addr?: string;
  tunnel_target_protocol?: 'http' | 'https';
  pages_project_id?: string;
  enabled: boolean;
};

const defaultValues: CreateProxyRouteFormValues = {
  site_name: '',
  zone_domain_ids: [],
  upstream_type: 'direct',
  origin_urls_text: '',
  tunnel_id: '',
  tunnel_target_addr: '',
  tunnel_target_protocol: 'http',
  pages_project_id: '',
  enabled: true,
};

interface ProxyRouteCreateSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (route: ProxyRouteItem) => void;
}

export function ProxyRouteCreateSheet({
  open,
  onOpenChange,
  onCreated,
}: ProxyRouteCreateSheetProps) {
  const t = useTranslations('proxyRoutes');
  const tc = useTranslations('common');
  const createProxyRouteSchema = z
    .object({
      site_name: z.string().trim().max(255, t('validation.siteNameTooLong')),
      zone_domain_ids: z
        .array(z.number().int().positive())
        .min(1, t('validation.selectAtLeastOneDomain')),
      upstream_type: z.enum(['direct', 'tunnel', 'pages']),
      origin_urls_text: z.string().trim(),
      tunnel_id: z.string().optional(),
      tunnel_target_addr: z.string().trim().optional(),
      tunnel_target_protocol: z.enum(['http', 'https']).optional(),
      pages_project_id: z.string().optional(),
      enabled: z.boolean(),
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
        return;
      }
      if (value.upstream_type === 'tunnel') {
        if (!value.tunnel_id) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['tunnel_id'],
            message: t('validation.selectTunnel'),
          });
        }
        if (!value.tunnel_target_addr?.trim()) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['tunnel_target_addr'],
            message: t('validation.enterTunnelTarget'),
          });
        }
        return;
      }
      if (!value.pages_project_id) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['pages_project_id'],
          message: t('validation.selectPagesProject'),
        });
      }
    });
  const form = useForm<CreateProxyRouteFormValues>({
    resolver: zodResolver(createProxyRouteSchema),
    defaultValues,
  });

  const zonesQuery = useQuery({
    queryKey: zoneQueryKey,
    queryFn: () => ZoneService.list(),
    enabled: open,
  });

  const domainsQuery = useQuery({
    queryKey: [...zoneQueryKey, 'all-domains'],
    queryFn: () => listAllZoneDomains(),
    enabled: open,
  });

  const tunnelsQuery = useQuery({
    queryKey: ['openflare', 'nodes'],
    queryFn: () => NodeService.listNodes(),
    enabled: open,
  });

  const pagesProjectsQuery = useQuery({
    queryKey: ['openflare', 'pages-projects'],
    queryFn: () => PagesService.listProjects(),
    enabled: open,
  });

  const tunnelClients = (tunnelsQuery.data ?? []).filter(
    (node) => node.node_type === 'tunnel_client',
  );
  const pagesProjects = (pagesProjectsQuery.data ?? []).filter(
    (project) => project.enabled && project.active_deployment_id,
  );

  const upstreamType = form.watch('upstream_type');

  useEffect(() => {
    if (!open) {
      form.reset(defaultValues);
    }
  }, [form, open]);

  const handleSubmit = form.handleSubmit(async (values) => {
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

    const selectedDomains = (domainsQuery.data ?? []).filter((domain) =>
      values.zone_domain_ids.includes(domain.id),
    );
    const primaryDomain = selectedDomains[0]?.domain ?? '';
    const hasCert = selectedDomains.some((domain) => domain.cert_id != null);

    try {
      const route = await ProxyRouteService.create({
        site_name: values.site_name.trim() || primaryDomain,
        zone_domain_ids: values.zone_domain_ids,
        origin_id: null,
        origin_url: originUrl,
        origin_scheme: originScheme,
        origin_address: originAddress,
        origin_port: originPort,
        origin_uri: originUri,
        origin_host: '',
        upstreams,
        enabled: values.enabled,
        enable_https: hasCert,
        redirect_http: false,
        limit_conn_per_server: 0,
        limit_conn_per_ip: 0,
        limit_rate: '',
        limit_req_per_ip: '',
        cache_enabled: true,
        cache_policy: 'static',
        cache_rules: [],
        custom_headers: [],
        basic_auth_enabled: false,
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

      form.reset(defaultValues);
      onOpenChange(false);
      onCreated(route);
    } catch (error) {
      form.setError('root', {
        message: error instanceof Error ? error.message : t('createFailed'),
      });
    }
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side='right' className='w-full sm:max-w-lg overflow-y-auto'>
        <SheetHeader>
          <SheetTitle>{t('createRule')}</SheetTitle>
          <SheetDescription>{t('createDesc')}</SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form onSubmit={handleSubmit} className='space-y-4 px-4 pb-4'>
            <FormField
              control={form.control}
              name='site_name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('siteId')}</FormLabel>
                  <FormControl>
                    <Input placeholder='marketing-site' {...field} />
                  </FormControl>
                  <FormDescription>{t('siteIdOptionalDesc')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='zone_domain_ids'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('bindDomains')}</FormLabel>
                  <FormControl>
                    <ZoneDomainSelector
                      value={field.value}
                      onChange={field.onChange}
                      domains={domainsQuery.data ?? []}
                      zones={zonesQuery.data ?? []}
                      disabled={domainsQuery.isLoading}
                      onDomainCreated={async () => {
                        await domainsQuery.refetch();
                      }}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

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
                        className='min-h-32 font-mono text-xs'
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
                            <SelectItem
                              key={tunnel.id}
                              value={String(tunnel.id)}
                            >
                              {tunnel.name} (
                              {tunnel.status === 'online'
                                ? t('online')
                                : t('offline')}
                              )
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormDescription>
                        {t('tunnelForwardHint')}
                      </FormDescription>
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
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                      >
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
                      <FormDescription>
                        {t('tunnelAddressHint')}
                      </FormDescription>
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
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            ) : null}

            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between rounded-lg border p-3'>
                  <div className='space-y-0.5'>
                    <FormLabel>{t('enableSite')}</FormLabel>
                    <FormDescription>{t('enableSiteDesc')}</FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />

            {form.formState.errors.root ? (
              <p className='text-sm text-destructive'>
                {form.formState.errors.root.message}
              </p>
            ) : null}

            <SheetFooter className='px-0'>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
              >
                {tc('cancel')}
              </Button>
              <Button type='submit' disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting ? t('creating') : t('create')}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  );
}
