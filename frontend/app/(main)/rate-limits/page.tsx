'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ExternalLink, Gauge, Loader2, Save } from 'lucide-react';
import { toast } from 'sonner';

import { useAuth } from '@/components/providers/auth-provider';
import { EmptyStateWithBorder } from '@/components/layout/empty';
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

const limitRatePattern = /^\d+(?:[kKmM])?$/;

type RateLimitFields = {
  openresty_default_limit_conn_per_server: string;
  openresty_default_limit_conn_per_ip: string;
  openresty_default_limit_rate: string;
};

const defaultFields: RateLimitFields = {
  openresty_default_limit_conn_per_server: '0',
  openresty_default_limit_conn_per_ip: '0',
  openresty_default_limit_rate: '',
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
  };
}

function validateFields(fields: RateLimitFields) {
  for (const key of [KEY_CONN_PER_SERVER, KEY_CONN_PER_IP] as const) {
    const raw = fields[key].trim();
    if (!raw) continue;
    if (!/^\d+$/.test(raw)) {
      throw new Error('并发限制请输入非负整数，或留空表示关闭');
    }
  }

  const rate = fields.openresty_default_limit_rate.trim();
  if (rate && rate !== '0' && !limitRatePattern.test(rate)) {
    throw new Error('限速格式不合法，请使用 512k、1m、纯数字，或留空关闭');
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

export default function RateLimitsPage() {
  const { user, loading: authLoading } = useAuth();
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<RateLimitFields>(defaultFields);
  const [saving, setSaving] = useState(false);

  const optionsQuery = useQuery({
    queryKey: optionsQueryKey,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  useEffect(() => {
    if (!optionsQuery.data) return;
    setFields(mapOptionsToFields(optionsToMap(optionsQuery.data)));
  }, [optionsQuery.data]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      validateFields(fields);
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
      ]);
    },
    onSuccess: async () => {
      toast.success('限流参数已保存');
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
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  const updateField = <K extends keyof RateLimitFields>(
    key: K,
    value: RateLimitFields[K],
  ) => {
    setFields((prev) => ({ ...prev, [key]: value }));
  };

  if (authLoading) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder icon={Gauge} description='加载权限信息...' />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='py-6 px-1'>
        <EmptyStateWithBorder
          icon={Gauge}
          title='权限不足'
          description='只有管理员可以访问限流设置。'
        />
      </div>
    );
  }

  if (optionsQuery.isLoading) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder icon={Gauge} description='加载限流参数...' />
      </div>
    );
  }

  if (optionsQuery.isError) {
    return (
      <div className='py-6 px-1'>
        <ErrorInline
          message={
            optionsQuery.error instanceof Error
              ? optionsQuery.error.message
              : '加载失败'
          }
          onRetry={() => void optionsQuery.refetch()}
        />
      </div>
    );
  }

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between'>
        <div className='flex items-center gap-2'>
          <Gauge className='size-5 text-primary' />
          <div>
            <h1 className='text-2xl font-semibold tracking-tight'>限流</h1>
            <p className='text-sm text-muted-foreground'>
              配置边缘站点默认并发与带宽限流。0
              或空表示默认关闭；站点未单独配置时继承此处设置。修改后需在版本发布中生效。
            </p>
          </div>
        </div>
        <Button variant='outline' size='sm' asChild>
          <Link href='/config-versions'>
            <ExternalLink className='size-3.5 mr-1' />
            查看配置预览
          </Link>
        </Button>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between'>
          <div>
            <CardTitle className='text-base'>全局默认限流</CardTitle>
            <CardDescription>
              0 或空表示默认关闭；站点未单独配置时继承此处设置。
            </CardDescription>
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
            保存
          </Button>
        </CardHeader>
        <CardContent className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
          <div className='space-y-1.5'>
            <Label className='text-xs text-muted-foreground'>
              默认并发限制（每站点）
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
              站点最大并发连接数默认值
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label className='text-xs text-muted-foreground'>
              默认单 IP 并发
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
              单个 IP 最大并发连接数默认值
            </p>
          </div>
          <div className='space-y-1.5'>
            <Label className='text-xs text-muted-foreground'>默认限速</Label>
            <Input
              value={fields.openresty_default_limit_rate}
              placeholder='512k / 1m'
              onChange={(e) =>
                updateField('openresty_default_limit_rate', e.target.value)
              }
              className='h-9 text-xs'
            />
            <p className='text-xs text-muted-foreground'>
              单请求带宽默认值，例如 512k 或 1m
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
