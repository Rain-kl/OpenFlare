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

import { useTranslations } from 'next-intl';

import {
  normalizeLimitRate,
  normalizeLimitReqPerIP,
  validateLimitRate,
  validateLimitReqPerIP,
} from '../../components/helpers';
import { proxyRouteFormIds } from '../helpers';
import { useRouteSectionSave } from '../hooks/use-route-section-save';
import { SectionShell } from './section-shell';

type RateLimitValues = {
  limit_conn_per_server: string;
  limit_conn_per_ip: string;
  limit_rate: string;
  limit_req_per_ip: string;
};

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
  const t = useTranslations('proxyRoutes');
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
            message: t('validation.enterIntegerOrMinusOne'),
          });
        }
      }

      const limitRateError = validateLimitRate(value.limit_rate, t);
      if (limitRateError) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['limit_rate'],
          message: limitRateError,
        });
      }

      const limitReqError = validateLimitReqPerIP(value.limit_req_per_ip, t);
      if (limitReqError) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['limit_req_per_ip'],
          message: limitReqError,
        });
      }
    });
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
      title={t('limits')}
      description={t('limitsDesc')}
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
              t('limitsSaved'),
            );
          })}
        >
          <FormField
            control={form.control}
            name='limit_conn_per_server'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('connLimit')}</FormLabel>
                <FormControl>
                  <Input placeholder='120' {...field} />
                </FormControl>
                <FormDescription>{t('connLimitHint')}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='limit_conn_per_ip'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('ipLimit')}</FormLabel>
                <FormControl>
                  <Input placeholder='12' {...field} />
                </FormControl>
                <FormDescription>{t('ipLimitHint')}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='limit_rate'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('rateLimit')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('rateLimitPlaceholder')} {...field} />
                </FormControl>
                <FormDescription>{t('rateLimitHint')}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='limit_req_per_ip'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('reqLimit')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('reqLimitPlaceholder')} {...field} />
                </FormControl>
                <FormDescription>{t('reqLimitHint')}</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </SectionShell>
  );
}
