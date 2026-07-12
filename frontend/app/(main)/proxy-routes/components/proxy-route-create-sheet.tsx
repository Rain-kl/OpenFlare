'use client';

import {useEffect} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {useQuery} from '@tanstack/react-query';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {Button} from '@/components/ui/button';
import {Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import {Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle,} from '@/components/ui/sheet';
import {Switch} from '@/components/ui/switch';
import {Textarea} from '@/components/ui/textarea';
import type {ProxyRouteItem} from '@/lib/services/openflare';
import {ProxyRouteService, ZoneService, zoneQueryKey} from '@/lib/services/openflare';

import {listAllZoneDomains, parseOriginUrl} from './helpers';
import {ZoneDomainSelector} from './zone-domain-selector';

const createProxyRouteSchema = z
  .object({
    site_name: z.string().trim().max(255, '站点标识不能超过 255 个字符'),
    zone_domain_ids: z.array(z.number().int().positive()).min(1, '请至少选择一个域名'),
    origin_url: z.string().trim().min(1, '请输入上游地址'),
    enabled: z.boolean(),
    remark: z.string().max(255, '备注不能超过 255 个字符'),
  })
  .superRefine((value, context) => {
    try {
      const parsed = new URL(value.origin_url);
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['origin_url'],
          message: '上游地址必须以 http:// 或 https:// 开头',
        });
      }
    } catch {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['origin_url'],
        message: '上游地址格式不合法',
      });
    }
  });

type CreateProxyRouteFormValues = z.infer<typeof createProxyRouteSchema>;

const defaultValues: CreateProxyRouteFormValues = {
  site_name: '',
  zone_domain_ids: [],
  origin_url: '',
  enabled: true,
  remark: '',
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

  useEffect(() => {
    if (!open) {
      form.reset(defaultValues);
    }
  }, [form, open]);

  const handleSubmit = form.handleSubmit(async (values) => {
    const origin = parseOriginUrl(values.origin_url.trim());
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
        origin_url: values.origin_url.trim(),
        origin_scheme: origin.scheme,
        origin_address: origin.address,
        origin_port: origin.port,
        origin_uri: origin.uri,
        origin_host: '',
        upstreams: [],
        enabled: values.enabled,
        enable_https: hasCert,
        redirect_http: false,
        limit_conn_per_server: 0,
        limit_conn_per_ip: 0,
        limit_rate: '',
        cache_enabled: false,
        cache_policy: 'url',
        cache_rules: [],
        custom_headers: [],
        basic_auth_enabled: false,
        remark: values.remark.trim(),
        upstream_type: 'direct',
      });

      form.reset(defaultValues);
      onOpenChange(false);
      onCreated(route);
    } catch (error) {
      form.setError('root', {
        message: error instanceof Error ? error.message : '创建失败，请稍后重试',
      });
    }
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>新建规则</SheetTitle>
          <SheetDescription>
            从已注册的 Zone 域名中选择绑定关系，创建后可继续配置缓存和限流等高级选项。
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form onSubmit={handleSubmit} className="space-y-4 px-4 pb-4">
            <FormField
              control={form.control}
              name="site_name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>站点标识</FormLabel>
                  <FormControl>
                    <Input placeholder="marketing-site" {...field} />
                  </FormControl>
                  <FormDescription>可选，留空时会自动使用首个域名。</FormDescription>
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
                      disabled={domainsQuery.isLoading}
                    />
                  </FormControl>
                  <FormDescription>
                    域名与证书在 Zone 中维护；此处只选择本站点要绑定的 FQDN。
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="origin_url"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>上游地址</FormLabel>
                  <FormControl>
                    <Input placeholder="http://127.0.0.1:8080" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

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
              name="remark"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>备注</FormLabel>
                  <FormControl>
                    <Textarea placeholder="可选备注" rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {form.formState.errors.root ? (
              <p className="text-sm text-destructive">{form.formState.errors.root.message}</p>
            ) : null}

            <SheetFooter className="px-0">
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button type="submit" disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting ? '创建中…' : '创建'}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  );
}
