'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from '@tanstack/react-query';
import { Database, KeyRound, Save, ShieldAlert, X } from 'lucide-react';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import services from '@/lib/services';
import type { SystemConfig } from '@/lib/services/admin';
import { TemplatesManager } from './templates';
import { toast } from 'sonner';
import { useTranslations } from 'next-intl';

const LOG_RETENTION_FIELDS = [
  {
    key: 'log_retention_days_postgres',
    label: 'PostgreSQL',
    descKey: 'retentionPostgresDesc',
  },
  {
    key: 'log_retention_days_sqlite',
    label: 'SQLite',
    descKey: 'retentionSqliteDesc',
  },
  {
    key: 'log_retention_days_clickhouse',
    label: 'ClickHouse',
    descKey: 'retentionClickhouseDesc',
  },
] as const;

interface OperationTabProps {
  configs: Record<string, SystemConfig>;
  systemConfigsQuery: UseQueryResult<SystemConfig[], Error>;
}

export function OperationTab({
  configs,
  systemConfigsQuery,
}: OperationTabProps) {
  const queryClient = useQueryClient();
  const t = useTranslations('settings.operation');

  const uploadTypesQuery = useQuery({
    queryKey: ['admin', 'upload-types'],
    queryFn: () => services.adminSystemConfig.listUploadTypes(),
  });

  const businessConfigsQuery = useQuery({
    queryKey: ['admin', 'system-configs', 'business'],
    queryFn: () => services.adminSystemConfig.listSystemConfigs('business'),
  });

  const businessConfigs = useMemo(() => {
    return (businessConfigsQuery.data ?? []).reduce<
      Record<string, SystemConfig>
    >((acc, config) => {
      acc[config.key] = config;
      return acc;
    }, {});
  }, [businessConfigsQuery.data]);

  const [retentionValues, setRetentionValues] = useState<
    Record<string, string>
  >({});

  useEffect(() => {
    if (!businessConfigsQuery.data) return;
    setRetentionValues((prev) => {
      const next: Record<string, string> = {};
      LOG_RETENTION_FIELDS.forEach((field) => {
        const config = businessConfigs[field.key];
        next[field.key] = config?.value || prev[field.key] || '90';
      });
      return next;
    });
  }, [businessConfigsQuery.data, businessConfigs]);

  const updateRetentionMutation = useMutation({
    mutationFn: async (values: Record<string, string>) => {
      for (const field of LOG_RETENTION_FIELDS) {
        const raw = (values[field.key] ?? '').trim();
        const num = Number(raw);
        if (!raw || !Number.isInteger(num) || num < 1) {
          throw new Error(t('retentionInvalid', { label: field.label }));
        }
        const config = businessConfigs[field.key];
        if (!config) {
          throw new Error(`缺少配置项: ${field.key}`);
        }
        await services.adminSystemConfig.updateSystemConfig(field.key, {
          value: String(num),
          description: config.description,
        });
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'system-configs'],
      });
      toast.success(t('logRetentionUpdated'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('logRetentionUpdateFailed'));
    },
  });

  const updateWhitelistMutation = useMutation({
    mutationFn: async (newValue: string) => {
      const config = configs['file_access_whitelist'];
      if (!config) {
        throw new Error('缺少配置项: file_access_whitelist');
      }
      await services.adminSystemConfig.updateSystemConfig(
        'file_access_whitelist',
        {
          value: newValue,
          description: config.description,
        },
      );
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'system-configs'],
      });
      await queryClient.invalidateQueries({ queryKey: ['public-config'] });
      toast.success(t('whitelistUpdated'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('updateWhitelistFailed'));
    },
  });

  const whitelistConfig = configs['file_access_whitelist'];
  const currentWhitelist = useMemo<string[]>(() => {
    if (!whitelistConfig?.value) return ['avatar'];
    try {
      const parsed = JSON.parse(whitelistConfig.value);
      if (Array.isArray(parsed)) return parsed;
    } catch {
      // 降级支持逗号分隔解析
      return whitelistConfig.value
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean);
    }
    return ['avatar'];
  }, [whitelistConfig?.value]);

  const handleAddType = (type: string) => {
    if (!type || currentWhitelist.includes(type)) return;
    const newWhitelist = [...currentWhitelist, type];
    updateWhitelistMutation.mutate(JSON.stringify(newWhitelist));
  };

  const handleRemoveType = (typeToRemove: string) => {
    const newWhitelist = currentWhitelist.filter((t) => t !== typeToRemove);
    updateWhitelistMutation.mutate(JSON.stringify(newWhitelist));
  };

  const availableTypes = useMemo(() => {
    const types = uploadTypesQuery.data ?? [];
    return types.map((type) => {
      let label = type;
      if (type === 'avatar') label = t('typeAvatar');
      else if (type === 'attachment') label = t('typeAttachment');
      else if (type === 'doc') label = t('typeDoc');
      else if (type === 'generic') label = t('typeGeneric');
      return { value: type, label };
    });
  }, [t, uploadTypesQuery.data]);

  return (
    <div className='space-y-6'>
      {/* 文件访问白名单设置 */}
      <Card className='border border-dashed shadow-sm'>
        <CardHeader className='border-b border-dashed pb-4'>
          <div className='flex items-center gap-2'>
            <div className='p-1.5 rounded-lg bg-primary/10 text-primary'>
              <KeyRound className='size-4' />
            </div>
            <div>
              <CardTitle className='text-base font-semibold'>
                {t('fileAccessControl')}
              </CardTitle>
              <CardDescription className='text-xs'>
                {t('fileAccessControlDesc')}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className='pt-6 space-y-4'>
          <div className='flex flex-col gap-4'>
            <div className='flex items-center gap-3'>
              <span className='text-sm font-medium text-muted-foreground'>
                {t('addAuthFreeType')}
              </span>
              <Select
                value=''
                onValueChange={handleAddType}
                disabled={
                  updateWhitelistMutation.isPending ||
                  systemConfigsQuery.isPending ||
                  uploadTypesQuery.isPending
                }
              >
                <SelectTrigger className='w-[200px]' size='sm'>
                  <SelectValue placeholder={t('selectBusinessType')} />
                </SelectTrigger>
                <SelectContent>
                  {availableTypes
                    .filter((t) => !currentWhitelist.includes(t.value))
                    .map((t) => (
                      <SelectItem key={t.value} value={t.value}>
                        {t.label}
                      </SelectItem>
                    ))}
                  {availableTypes.filter(
                    (t) => !currentWhitelist.includes(t.value),
                  ).length === 0 && (
                    <div className='text-xs text-muted-foreground p-2 text-center'>
                      {t('allTypesAdded')}
                    </div>
                  )}
                </SelectContent>
              </Select>
            </div>

            {/* 当前白名单列表 */}
            <div className='rounded-xl border border-dashed p-4 bg-card hover:bg-muted/10 hover:border-primary/30 transition-all duration-300 shadow-sm space-y-3'>
              <div className='flex items-center gap-2'>
                <ShieldAlert className='size-4 text-primary' />
                <span className='font-medium text-sm text-foreground'>
                  {t('currentAuthFreeList')}
                </span>
              </div>

              {currentWhitelist.length > 0 ? (
                <div className='flex flex-wrap gap-2'>
                  {currentWhitelist.map((type) => (
                    <Badge
                      key={type}
                      variant='secondary'
                      className='px-2.5 py-1 text-xs gap-1.5 flex items-center bg-primary/10 text-primary dark:bg-primary/20 border border-primary/20'
                    >
                      {availableTypes.find((t) => t.value === type)?.label ||
                        type}
                      <button
                        type='button'
                        onClick={() => handleRemoveType(type)}
                        disabled={
                          updateWhitelistMutation.isPending ||
                          systemConfigsQuery.isPending
                        }
                        className='rounded-full outline-hidden hover:bg-primary/20 p-0.5 text-primary cursor-pointer disabled:cursor-not-allowed'
                      >
                        <X className='size-3' />
                      </button>
                    </Badge>
                  ))}
                </div>
              ) : (
                <p className='text-xs text-muted-foreground'>
                  {t('whitelistEmpty')}
                </p>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 日志保留时间设置 */}
      <Card className='border border-dashed shadow-sm'>
        <CardHeader className='border-b border-dashed pb-4'>
          <div className='flex items-center gap-2'>
            <div className='p-1.5 rounded-lg bg-primary/10 text-primary'>
              <Database className='size-4' />
            </div>
            <div>
              <CardTitle className='text-base font-semibold'>
                {t('logRetentionTitle')}
              </CardTitle>
              <CardDescription className='text-xs'>
                {t('logRetentionDesc')}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className='pt-6'>
          <div className='grid gap-4 sm:grid-cols-3'>
            {LOG_RETENTION_FIELDS.map((field) => (
              <div key={field.key} className='grid gap-2'>
                <Label htmlFor={field.key}>{field.label}</Label>
                <div className='flex items-center gap-2'>
                  <Input
                    id={field.key}
                    type='number'
                    min={1}
                    className='text-xs'
                    value={retentionValues[field.key] ?? ''}
                    disabled={
                      updateRetentionMutation.isPending ||
                      businessConfigsQuery.isPending
                    }
                    onChange={(e) =>
                      setRetentionValues((prev) => ({
                        ...prev,
                        [field.key]: e.target.value,
                      }))
                    }
                  />
                  <span className='text-xs text-muted-foreground whitespace-nowrap'>
                    {t('days')}
                  </span>
                </div>
                <p className='text-[10px] text-muted-foreground'>
                  {t(field.descKey)}
                </p>
              </div>
            ))}
          </div>
          <div className='mt-4 flex justify-end'>
            <Button
              type='button'
              size='sm'
              onClick={() => updateRetentionMutation.mutate(retentionValues)}
              disabled={
                updateRetentionMutation.isPending ||
                businessConfigsQuery.isPending
              }
            >
              {updateRetentionMutation.isPending ? (
                <Spinner className='size-3' />
              ) : (
                <Save className='size-3' />
              )}
              {t('save')}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* 通知模板管理 */}
      <TemplatesManager />
    </div>
  );
}
