'use client';

import {useEffect, useMemo} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {useQuery} from '@tanstack/react-query';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {Switch} from '@/components/ui/switch';
import type {ProxyRouteItem} from '@/lib/services/openflare';
import {ZoneService, zoneQueryKey} from '@/lib/services/openflare';

import {listAllZoneDomains} from '../../components/helpers';
import {ZoneDomainSelector} from '../../components/zone-domain-selector';
import {proxyRouteFormIds} from '../helpers';
import {useRouteSectionSave} from '../hooks/use-route-section-save';
import {SectionShell} from './section-shell';

const domainSettingsSchema = z
  .object({
    site_name: z
      .string()
      .trim()
      .min(1, '请输入站点标识')
      .max(255, '站点标识不能超过 255 个字符'),
    zone_domain_ids: z.array(z.number().int().positive()).min(1, '请至少选择一个域名'),
    enabled: z.boolean(),
    redirect_http: z.boolean(),
  });

type DomainSettingsValues = z.infer<typeof domainSettingsSchema>;

interface DomainSectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

export function DomainSection({ route, onRouteUpdate, onSavingChange }: DomainSectionProps) {
  const { saving, save } = useRouteSectionSave(route, onRouteUpdate, onSavingChange);

  const zonesQuery = useQuery({
    queryKey: zoneQueryKey,
    queryFn: () => ZoneService.list(),
  });

  const domainsQuery = useQuery({
    queryKey: [...zoneQueryKey, 'all-domains'],
    queryFn: () => listAllZoneDomains(),
  });

  const form = useForm<DomainSettingsValues>({
    resolver: zodResolver(domainSettingsSchema),
    defaultValues: {
      site_name: route.site_name,
      zone_domain_ids: route.zone_domain_ids ?? [],
      enabled: route.enabled,
      redirect_http: route.redirect_http,
    },
  });

  useEffect(() => {
    form.reset({
      site_name: route.site_name,
      zone_domain_ids: route.zone_domain_ids ?? [],
      enabled: route.enabled,
      redirect_http: route.redirect_http,
    });
  }, [form, route]);

  const selectedIDs = form.watch('zone_domain_ids');
  const selectedDomains = useMemo(() => {
    const fromApi = domainsQuery.data ?? [];
    const byId = new Map(fromApi.map((domain) => [domain.id, domain]));
    // Prefer live catalog; fall back to route-bound domains for display before catalog loads.
    return selectedIDs
      .map((id) => {
        const catalog = byId.get(id);
        if (catalog) {
          return catalog;
        }
        const bound = (route.zone_domains ?? []).find((item) => item.id === id);
        if (!bound) {
          return null;
        }
        return {
          id: bound.id,
          zone_id: bound.zone_id,
          proxy_route_id: route.id,
          domain: bound.domain,
          cert_id: bound.cert_id,
          created_at: '',
          updated_at: '',
        };
      })
      .filter((item): item is NonNullable<typeof item> => item != null);
  }, [domainsQuery.data, route.id, route.zone_domains, selectedIDs]);

  const hasCertificate = selectedDomains.some((domain) => domain.cert_id != null);

  return (
    <SectionShell
      title="域名设置"
      description="绑定已在 Zone 中注册的 FQDN。证书请在 Zone 域名管理中维护。"
      formId={proxyRouteFormIds.domains}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.domains}
          className="space-y-5"
          onSubmit={form.handleSubmit(async (values) => {
            if (values.redirect_http && !hasCertificate) {
              form.setError('redirect_http', {
                message: '启用 HTTP 跳转前，请先为所选域名绑定证书（在 Zone 中配置）',
              });
              return;
            }

            await save(
              {
                site_name: values.site_name.trim(),
                zone_domain_ids: values.zone_domain_ids,
                enabled: values.enabled,
                enable_https: hasCertificate,
                redirect_http: hasCertificate ? values.redirect_http : false,
              },
              '域名设置已保存',
            );
          })}
        >
          <FormField
            control={form.control}
            name="enabled"
            render={({ field }) => (
              <FormItem className="flex items-center justify-between rounded-lg border p-3">
                <div className="space-y-0.5">
                  <FormLabel>启用站点</FormLabel>
                  <FormDescription>关闭后会保留配置，但不会参与发布。</FormDescription>
                </div>
                <FormControl>
                  <Switch checked={field.value} onCheckedChange={field.onChange} />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="site_name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>站点标识</FormLabel>
                <FormControl>
                  <Input placeholder="marketing-site" {...field} />
                </FormControl>
                <FormDescription>建议使用稳定、可读的业务标识，不必与域名完全一致。</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="zone_domain_ids"
            render={({ field }) => (
              <FormItem>
                <FormLabel>绑定域名</FormLabel>
                <FormControl>
                  <ZoneDomainSelector
                    value={field.value}
                    onChange={field.onChange}
                    domains={domainsQuery.data ?? []}
                    zones={zonesQuery.data ?? []}
                    currentRouteId={route.id}
                    disabled={domainsQuery.isLoading}
                    onDomainCreated={async () => {
                      await domainsQuery.refetch();
                    }}
                  />
                </FormControl>
                <FormDescription>
                  从已登记域名中勾选绑定；可用「快捷新增域名」在 Zone 下创建 FQDN
                  并自动勾选。
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="redirect_http"
            render={({ field }) => (
              <FormItem className="flex items-center justify-between rounded-lg border p-3">
                <div className="space-y-0.5">
                  <FormLabel>HTTP 自动跳转到 HTTPS</FormLabel>
                  <FormDescription>
                    {hasCertificate
                      ? '开启后会额外生成 80 端口重定向规则。'
                      : '所选域名至少绑定一张证书后才能启用。'}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    disabled={!hasCertificate}
                    onCheckedChange={field.onChange}
                  />
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
