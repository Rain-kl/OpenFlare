'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Gauge, Save } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useEffect, useId, useState } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { OptionService } from '@/lib/services/openflare';

const optionsQueryKey = ['openflare', 'options'] as const;

const KEY_CONN_PER_SERVER = 'edge_default_limit_conn_per_server';
const KEY_CONN_PER_IP = 'edge_default_limit_conn_per_ip';
const KEY_LIMIT_RATE = 'edge_default_limit_rate';
const KEY_LIMIT_REQ_PER_IP = 'edge_default_limit_req_per_ip';

const limitRatePattern = /^\d+(?:[kKmM])?$/;

type RateLimitFields = {
  edge_default_limit_conn_per_server: string;
  edge_default_limit_conn_per_ip: string;
  edge_default_limit_rate: string;
  edge_default_limit_req_per_ip: string;
};

const defaultFields: RateLimitFields = {
  edge_default_limit_conn_per_server: '0',
  edge_default_limit_conn_per_ip: '0',
  edge_default_limit_rate: '',
  edge_default_limit_req_per_ip: '',
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
    edge_default_limit_conn_per_server: optionMap[KEY_CONN_PER_SERVER] ?? '0',
    edge_default_limit_conn_per_ip: optionMap[KEY_CONN_PER_IP] ?? '0',
    edge_default_limit_rate: optionMap[KEY_LIMIT_RATE] ?? '',
    edge_default_limit_req_per_ip: optionMap[KEY_LIMIT_REQ_PER_IP] ?? '',
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

  const rate = fields.edge_default_limit_rate.trim();
  if (rate && rate !== '0' && !limitRatePattern.test(rate)) {
    throw new Error(t('config.invalidRate'));
  }

  const reqRate = fields.edge_default_limit_req_per_ip.trim();
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

export function RateLimitConfigCard() {
  const t = useTranslations('rateLimits');
  const tCommon = useTranslations('common');
  const queryClient = useQueryClient();
  const id = useId();
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
          value: normalizeConnValue(fields.edge_default_limit_conn_per_server),
        },
        {
          key: KEY_CONN_PER_IP,
          value: normalizeConnValue(fields.edge_default_limit_conn_per_ip),
        },
        {
          key: KEY_LIMIT_RATE,
          value: normalizeRateValue(fields.edge_default_limit_rate),
        },
        {
          key: KEY_LIMIT_REQ_PER_IP,
          value: normalizeRateValue(fields.edge_default_limit_req_per_ip),
        },
      ]);
    },
    onSuccess: async () => {
      toast.success(t('config.saved'));
      await queryClient.invalidateQueries({ queryKey: optionsQueryKey });
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
      <div className='rounded-lg border border-dashed p-6'>
        <ErrorInline
          message={
            optionsQuery.error instanceof Error
              ? optionsQuery.error.message
              : t('loadFailed')
          }
          onRetry={() => void optionsQuery.refetch()}
          className='justify-center'
        />
      </div>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-base'>{t('config.title')}</CardTitle>
        <CardDescription>{t('config.description')}</CardDescription>
        <CardAction>
          <Button
            size='sm'
            disabled={saving}
            onClick={() => saveMutation.mutate()}
          >
            {saving ? (
              <Spinner data-icon='inline-start' />
            ) : (
              <Save data-icon='inline-start' />
            )}
            {tCommon('save')}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        <FieldGroup className='grid gap-x-6 gap-y-5 md:grid-cols-2'>
          <Field>
            <FieldLabel htmlFor={`${id}-conn-server`}>
              {t('config.connPerServer')}
            </FieldLabel>
            <Input
              id={`${id}-conn-server`}
              type='number'
              min={0}
              value={fields.edge_default_limit_conn_per_server}
              placeholder='0'
              aria-describedby={`${id}-conn-server-help`}
              onChange={(e) =>
                updateField(
                  'edge_default_limit_conn_per_server',
                  e.target.value,
                )
              }
            />
            <FieldDescription id={`${id}-conn-server-help`}>
              {t('config.connPerServerHint')}
            </FieldDescription>
          </Field>

          <Field>
            <FieldLabel htmlFor={`${id}-conn-ip`}>
              {t('config.connPerIp')}
            </FieldLabel>
            <Input
              id={`${id}-conn-ip`}
              type='number'
              min={0}
              value={fields.edge_default_limit_conn_per_ip}
              placeholder='0'
              aria-describedby={`${id}-conn-ip-help`}
              onChange={(e) =>
                updateField('edge_default_limit_conn_per_ip', e.target.value)
              }
            />
            <FieldDescription id={`${id}-conn-ip-help`}>
              {t('config.connPerIpHint')}
            </FieldDescription>
          </Field>

          <Field>
            <FieldLabel htmlFor={`${id}-limit-rate`}>
              {t('config.limitRate')}
            </FieldLabel>
            <Input
              id={`${id}-limit-rate`}
              value={fields.edge_default_limit_rate}
              placeholder='512k / 1m'
              aria-describedby={`${id}-limit-rate-help`}
              onChange={(e) =>
                updateField('edge_default_limit_rate', e.target.value)
              }
            />
            <FieldDescription id={`${id}-limit-rate-help`}>
              {t('config.limitRateHint')}
            </FieldDescription>
          </Field>

          <Field>
            <FieldLabel htmlFor={`${id}-req-ip`}>
              {t('config.reqPerIp')}
            </FieldLabel>
            <Input
              id={`${id}-req-ip`}
              value={fields.edge_default_limit_req_per_ip}
              placeholder='10r/s / 100r/m'
              aria-describedby={`${id}-req-ip-help`}
              onChange={(e) =>
                updateField('edge_default_limit_req_per_ip', e.target.value)
              }
            />
            <FieldDescription id={`${id}-req-ip-help`}>
              {t('config.reqPerIpHint')}
            </FieldDescription>
          </Field>
        </FieldGroup>
      </CardContent>
    </Card>
  );
}
