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
import {TlsCertificateService, WebsiteService} from '@/lib/services/openflare';

import {validateDomains} from '../../components/helpers';
import {
  buildDomainCertificateIDs,
  buildDomainRows,
  normalizeSelectedCertificateIDs,
  proxyRouteFormIds,
} from '../helpers';
import {useRouteSectionSave} from '../hooks/use-route-section-save';
import {DomainListInput} from './domain-list-input';
import {SectionShell} from './section-shell';

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

type DomainSettingsValues = z.infer<typeof domainSettingsSchema>;

interface DomainSectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

export function DomainSection({ route, onRouteUpdate, onSavingChange }: DomainSectionProps) {
  const { saving, save } = useRouteSectionSave(route, onRouteUpdate, onSavingChange);

  const certificatesQuery = useQuery({
    queryKey: ['openflare', 'tls-certificates'],
    queryFn: () => TlsCertificateService.list(),
  });

  const managedDomainsQuery = useQuery({
    queryKey: ['openflare', 'managed-domains'],
    queryFn: () => WebsiteService.list(),
  });

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

  const domainSuggestionSources = useMemo(
    () => [
      ...(route.domains ?? []),
      ...(managedDomainsQuery.data?.map((item) => item.domain) ?? []),
    ],
    [managedDomainsQuery.data, route.domains],
  );

  const selectedCertificateIDs = normalizeSelectedCertificateIDs(form.watch('domain_rows'));

  return (
    <SectionShell
      title="域名设置"
      description="在一个列表里同时维护域名、证书和 HTTPS 跳转。保存时会自动汇总站点证书集合。"
      formId={proxyRouteFormIds.domains}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.domains}
          className="space-y-5"
          onSubmit={form.handleSubmit(async (values) => {
            const domains = values.domain_rows
              .map((item) => item.domain.trim().toLowerCase())
              .filter(Boolean);
            const domainCertIDs = buildDomainCertificateIDs(values.domain_rows);
            const certIDs = normalizeSelectedCertificateIDs(values.domain_rows);

            await save(
              {
                site_name: values.site_name.trim(),
                domain: domains[0],
                domains,
                enabled: values.enabled,
                enable_https: certIDs.length > 0,
                cert_id: certIDs[0] ?? null,
                cert_ids: certIDs,
                domain_cert_ids: domainCertIDs,
                redirect_http: certIDs.length > 0 ? values.redirect_http : false,
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
            name="domain_rows"
            render={({ field }) => (
              <FormItem>
                <FormLabel>域名列表</FormLabel>
                <FormControl>
                  <DomainListInput
                    rows={field.value}
                    onChange={field.onChange}
                    onBlur={field.onBlur}
                    suggestionSources={domainSuggestionSources}
                    certificates={certificatesQuery.data ?? []}
                  />
                </FormControl>
                <FormDescription>
                  每行配置一个域名。可为不同域名选择不同证书，托管域名将自动匹配证书。
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
                    {selectedCertificateIDs.length > 0
                      ? '开启后会额外生成 80 端口重定向规则。'
                      : '至少为一个域名选择证书后才能启用。'}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    disabled={selectedCertificateIDs.length === 0}
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
