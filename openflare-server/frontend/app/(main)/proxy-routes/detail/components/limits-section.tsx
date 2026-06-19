'use client';

import {useEffect} from 'react';
import {zodResolver} from '@hookform/resolvers/zod';
import {useForm} from 'react-hook-form';
import {z} from 'zod';

import {Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form';
import {Input} from '@/components/ui/input';
import type {ProxyRouteItem} from '@/lib/services/openflare';

import {normalizeLimitRate, validateLimitRate} from '../../components/helpers';
import {proxyRouteFormIds} from '../helpers';
import {useRouteSectionSave} from '../hooks/use-route-section-save';
import {SectionShell} from './section-shell';

const rateLimitSchema = z
  .object({
    limit_conn_per_server: z.string(),
    limit_conn_per_ip: z.string(),
    limit_rate: z.string(),
  })
  .superRefine((value, context) => {
    for (const field of ['limit_conn_per_server', 'limit_conn_per_ip'] as const) {
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

type RateLimitValues = z.infer<typeof rateLimitSchema>;

interface LimitsSectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

export function LimitsSection({ route, onRouteUpdate, onSavingChange }: LimitsSectionProps) {
  const { saving, save } = useRouteSectionSave(route, onRouteUpdate, onSavingChange);

  const form = useForm<RateLimitValues>({
    resolver: zodResolver(rateLimitSchema),
    defaultValues: {
      limit_conn_per_server: route.limit_conn_per_server
        ? String(route.limit_conn_per_server)
        : '',
      limit_conn_per_ip: route.limit_conn_per_ip ? String(route.limit_conn_per_ip) : '',
      limit_rate: route.limit_rate || '',
    },
  });

  useEffect(() => {
    form.reset({
      limit_conn_per_server: route.limit_conn_per_server
        ? String(route.limit_conn_per_server)
        : '',
      limit_conn_per_ip: route.limit_conn_per_ip ? String(route.limit_conn_per_ip) : '',
      limit_rate: route.limit_rate || '',
    });
  }, [form, route]);

  return (
    <SectionShell
      title="流量限制"
      description="站点限流，空值或 0 表示关闭。"
      formId={proxyRouteFormIds.limits}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.limits}
          className="grid gap-5 md:grid-cols-2"
          onSubmit={form.handleSubmit(async (values) => {
            await save(
              {
                limit_conn_per_server: Number(values.limit_conn_per_server.trim() || '0'),
                limit_conn_per_ip: Number(values.limit_conn_per_ip.trim() || '0'),
                limit_rate: normalizeLimitRate(values.limit_rate),
              },
              '流量限制已保存',
            );
          })}
        >
          <FormField
            control={form.control}
            name="limit_conn_per_server"
            render={({ field }) => (
              <FormItem>
                <FormLabel>并发限制</FormLabel>
                <FormControl>
                  <Input placeholder="120" {...field} />
                </FormControl>
                <FormDescription>限制当前站点最大并发连接数。</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="limit_conn_per_ip"
            render={({ field }) => (
              <FormItem>
                <FormLabel>单 IP 限制</FormLabel>
                <FormControl>
                  <Input placeholder="12" {...field} />
                </FormControl>
                <FormDescription>限制单个 IP 的最大并发数。</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="limit_rate"
            render={({ field }) => (
              <FormItem className="md:col-span-2">
                <FormLabel>限速</FormLabel>
                <FormControl>
                  <Input placeholder="512k/1m" {...field} />
                </FormControl>
                <FormDescription>限制单请求带宽，例如 512k 或 1m。</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </SectionShell>
  );
}
