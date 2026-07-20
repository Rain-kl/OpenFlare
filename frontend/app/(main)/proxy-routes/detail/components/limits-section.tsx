'use client';

import { useEffect } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
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
import type { ProxyRouteItem } from '@/lib/services/openflare';

import {
  normalizeLimitRate,
  normalizeLimitReqPerIP,
  validateLimitRate,
  validateLimitReqPerIP,
} from '../../components/helpers';
import { proxyRouteFormIds } from '../helpers';
import { useRouteSectionSave } from '../hooks/use-route-section-save';
import { SectionShell } from './section-shell';

const rateLimitSchema = z
  .object({
    limit_conn_per_server: z.string(),
    limit_conn_per_ip: z.string(),
    limit_rate: z.string(),
    limit_req_per_ip: z.string(),
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
      if (!/^-1$|^\d+$/.test(rawValue)) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: [field],
          message: '请输入 -1、0 或正整数',
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

    const limitReqError = validateLimitReqPerIP(value.limit_req_per_ip);
    if (limitReqError) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['limit_req_per_ip'],
        message: limitReqError,
      });
    }
  });

type RateLimitValues = z.infer<typeof rateLimitSchema>;

function formatConnValue(value: number | null | undefined) {
  if (value === null || value === undefined || value === 0) {
    return '';
  }
  return String(value);
}

function parseConnValue(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return 0;
  }
  return Number(trimmed);
}

interface LimitsSectionProps {
  route: ProxyRouteItem;
  onRouteUpdate: (route: ProxyRouteItem) => void;
  onSavingChange?: (saving: boolean) => void;
}

export function LimitsSection({
  route,
  onRouteUpdate,
  onSavingChange,
}: LimitsSectionProps) {
  const { saving, save } = useRouteSectionSave(
    route,
    onRouteUpdate,
    onSavingChange,
  );

  const form = useForm<RateLimitValues>({
    resolver: zodResolver(rateLimitSchema),
    defaultValues: {
      limit_conn_per_server: formatConnValue(route.limit_conn_per_server),
      limit_conn_per_ip: formatConnValue(route.limit_conn_per_ip),
      limit_rate: route.limit_rate || '',
      limit_req_per_ip: route.limit_req_per_ip || '',
    },
  });

  useEffect(() => {
    form.reset({
      limit_conn_per_server: formatConnValue(route.limit_conn_per_server),
      limit_conn_per_ip: formatConnValue(route.limit_conn_per_ip),
      limit_rate: route.limit_rate || '',
      limit_req_per_ip: route.limit_req_per_ip || '',
    });
  }, [form, route]);

  return (
    <SectionShell
      title='流量限制'
      description='站点限流。空或 0 继承全局默认；-1 显式关闭；大于 0 为自定义。'
      formId={proxyRouteFormIds.limits}
      saving={saving}
    >
      <Form {...form}>
        <form
          id={proxyRouteFormIds.limits}
          className='grid gap-5 md:grid-cols-2'
          onSubmit={form.handleSubmit(async (values) => {
            await save(
              {
                limit_conn_per_server: parseConnValue(
                  values.limit_conn_per_server,
                ),
                limit_conn_per_ip: parseConnValue(values.limit_conn_per_ip),
                limit_rate: normalizeLimitRate(values.limit_rate),
                limit_req_per_ip: normalizeLimitReqPerIP(
                  values.limit_req_per_ip,
                ),
              },
              '流量限制已保存',
            );
          })}
        >
          <FormField
            control={form.control}
            name='limit_conn_per_server'
            render={({ field }) => (
              <FormItem>
                <FormLabel>并发限制</FormLabel>
                <FormControl>
                  <Input placeholder='120' {...field} />
                </FormControl>
                <FormDescription>
                  空或 0 继承全局默认；-1 关闭；大于 0 为自定义并发上限。
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='limit_conn_per_ip'
            render={({ field }) => (
              <FormItem>
                <FormLabel>单 IP 限制</FormLabel>
                <FormControl>
                  <Input placeholder='12' {...field} />
                </FormControl>
                <FormDescription>
                  空或 0 继承全局默认；-1 关闭；大于 0 为单 IP 自定义上限。
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='limit_rate'
            render={({ field }) => (
              <FormItem>
                <FormLabel>限速</FormLabel>
                <FormControl>
                  <Input placeholder='512k/1m 或 -1' {...field} />
                </FormControl>
                <FormDescription>
                  空或 0 继承全局默认；-1 关闭；例如 512k、1m 为自定义带宽。
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='limit_req_per_ip'
            render={({ field }) => (
              <FormItem>
                <FormLabel>单 IP 请求频率</FormLabel>
                <FormControl>
                  <Input placeholder='10r/s / 100r/m 或 -1' {...field} />
                </FormControl>
                <FormDescription>
                  空或 0 继承全局默认；-1 关闭；例如 10r/s、100r/m
                  为自定义频率。
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </SectionShell>
  );
}
