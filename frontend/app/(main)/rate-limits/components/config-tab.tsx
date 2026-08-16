'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ExternalLink, Gauge, Loader2, Save } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { OptionService } from '@/lib/services/openflare';

const optionsQueryKey = ['openflare', 'options'] as const;

const KEY_CONN_PER_SERVER = 'openresty_default_limit_conn_per_server';
const KEY_CONN_PER_IP = 'openresty_default_limit_conn_per_ip';
const KEY_LIMIT_RATE = 'openresty_default_limit_rate';
const KEY_LIMIT_REQ_PER_IP = 'openresty_default_limit_req_per_ip';

const limitRatePattern = /^\d+(?:[kKmM])?$/;

type RateLimitFields = {
  openresty_default_limit_conn_per_server: string;
  openresty_default_limit_conn_per_ip: string;
  openresty_default_limit_rate: string;
  openresty_default_limit_req_per_ip: string;
};

const defaultFields: RateLimitFields = {
  openresty_default_limit_conn_per_server: '0',
  openresty_default_limit_conn_per_ip: '0',
  openresty_default_limit_rate: '',
  openresty_default_limit_req_per_ip: '',
};

function optionsToMap(options: Array<{ key: string; value: string }>) {
  return options.reduce<Record<string, string>>((acc, option) => {
    acc[option.key] = option.value;
    return acc;
  }, {});
}

function mapOptionsToFields(
  optionMap: Record<string, string>,
): RateLimitFields {
  return {
    openresty_default_limit_conn_per_server:
      optionMap[KEY_CONN_PER_SERVER] ?? '0',
    openresty_default_limit_conn_per_ip: optionMap[KEY_CONN_PER_IP] ?? '0',
    openresty_default_limit_rate: optionMap[KEY_LIMIT_RATE] ?? '',
    openresty_default_limit_req_per_ip: optionMap[KEY_LIMIT_REQ_PER_IP] ?? '',
  };
}

function validateFields(
  fields: RateLimitFields,
  t: (
    key: 'config.invalidConn' | 'config.invalidRate' | 'config.invalidReq',
  ) => string,
) {
  for (const key of [KEY_CONN_PER_SERVER, KEY_CONN_PER_IP] as const) {
    const raw = fields[key].trim();
    if (!raw) continue;
    if (!/^\d+$/.test(raw)) {
      throw new Error(t('config.invalidConn'));
    }
  }

  const rate = fields.openresty_default_limit_rate.trim();
  if (rate && rate !== '0' && !limitRatePattern.test(rate)) {
    throw new Error(t('config.invalidRate'));
  }

  const reqRate = fields.openresty_default_limit_req_per_ip.trim();
  if (reqRate && reqRate !== '0' && !/^\d+r\/[sm]$/i.test(reqRate)) {
    throw new Error(t('config.invalidReq'));
  }
}

function normalizeConnValue(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return '0';
  return trimmed;
}

function normalizeRateValue(value: string) {
  const normalized = value.trim().toLowerCase();
  if (!normalized || normalized === '0') return '';
  return normalized;
}

export function ConfigTab() {
  const t = useTranslations('rateLimits');
  const tCommon = useTranslations('common');
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<RateLimitFields>(defaultFields);
  const [saving, setSaving] = useState(false);

  const optionsQuery = useQuery({
    queryKey: optionsQueryKey,
    queryFn: () => OptionService.list(),
  });

  useEffect(() => {
    if (!optionsQuery.data) return;
    setFields(mapOptionsToFields(optionsToMap(optionsQuery.data)));
  }, [optionsQuery.data]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      validateFields(fields, (key) => t(key));
      setSaving(true);
      await OptionService.updateBatch([
        {
          key: KEY_CONN_PER_SERVER,
          value: normalizeConnValue(
            fields.openresty_default_limit_conn_per_server,
          ),
        },
        {
          key: KEY_CONN_PER_IP,
          value: normalizeConnValue(fields.openresty_default_limit_conn_per_ip),
        },
        {
          key: KEY_LIMIT_RATE,
          value: normalizeRateValue(fields.openresty_default_limit_rate),
        },
        {
          key: KEY_LIMIT_REQ_PER_IP,
          value: normalizeRateValue(fields.openresty_default_limit_req_per_ip),
        },
      ]);
    },
    onSuccess: async () => {
      toast.success(t('config.saved'));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: optionsQueryKey }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'config-preview'],
        }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'config-versions'],
        }),
      ]);
      setSaving(false);
    },
    onError: (error) => {
      setSaving(false);
      toast.error(
        error instanceof Error ? error.message : t('config.saveFailed'),
      );
    },
  });

  const updateField = <K extends keyof RateLimitFields>(
    key: K,
    value: RateLimitFields[K],
  ) => {
    setFields((prev) => ({ ...prev, [key]: value }));
  };

  if (optionsQuery.isLoading) {
    return (
      <LoadingStateWithBorder icon={Gauge} description={t('config.loading')} />
    );
  }

  if (optionsQuery.isError) {
    return (
      <ErrorInline
        message={
          optionsQuery.error instanceof Error
            ? optionsQuery.error.message
            : t('loadFailed')
        }
        onRetry={() => void optionsQuery.refetch()}
      />
    );
  }

  return (
    <div className='space-y-4'>
      <div className='flex justify-end'>
        <Button variant='outline' size='sm' asChild>
          <Link href='/config-versions'>
            <ExternalLink className='size-3.5 mr-1' />
            {t('config.preview')}
          </Link>
        </Button>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between'>
          <div>
            <CardTitle className='text-base'>{t('config.title')}</CardTitle>
            <CardDescription>{t('config.description')}</CardDescription>
          </div>
          <Button
            size='sm'
            disabled={saving}
            onClick={() => saveMutation.mutate()}
          >
            {saving ? (
              <Loader2 className='size-4 animate-spin mr-1' />
            ) : (
              <Save className='size-3.5 mr-1' />
            )}
            {tCommon('save')}
          </Button>
        </CardHeader>
        <CardContent className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
          <div className='space-y-1.5'>
            <Label className='text-xs text-muted-foreground'>
              {t('config.connPerServer')}
            </Label>
            <Input
              type='number'
              min={0}
              value={fields.openresty_default_limit_conn_per_server}
              placeholder='0'
              onChange={(e) =>
                updateField(
                  'openresty_default_limit_conn_per_server',
                  e.target.value,
                )
              }
              className='h-9 text-xs'
            />
            <p className='text-xs text-muted-foreground'>
              {t('config.connPerServerHint')}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label className='text-xs text-muted-foreground'>
              {t('config.connPerIp')}
            </Label>
            <Input
              type='number'
              min={0}
              value={fields.openresty_default_limit_conn_per_ip}
              placeholder='0'
              onChange={(e) =>
                updateField(
                  'openresty_default_limit_conn_per_ip',
                  e.target.value,
                )
              }
              className='h-9 text-xs'
            />
            <p className='text-xs text-muted-foreground'>
              {t('config.connPerIpHint')}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label className='text-xs text-muted-foreground'>
              {t('config.limitRate')}
            </Label>
            <Input
              value={fields.openresty_default_limit_rate}
              placeholder='512k / 1m'
              onChange={(e) =>
                updateField('openresty_default_limit_rate', e.target.value)
              }
              className='h-9 text-xs'
            />
            <p className='text-xs text-muted-foreground'>
              {t('config.limitRateHint')}
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label className='text-xs text-muted-foreground'>
              {t('config.reqPerIp')}
            </Label>
            <Input
              value={fields.openresty_default_limit_req_per_ip}
              placeholder='10r/s / 100r/m'
              onChange={(e) =>
                updateField(
                  'openresty_default_limit_req_per_ip',
                  e.target.value,
                )
              }
              className='h-9 text-xs'
            />
            <p className='text-xs text-muted-foreground'>
              {t('config.reqPerIpHint')}
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
