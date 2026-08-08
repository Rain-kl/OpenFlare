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

const LOG_RETENTION_FIELDS = [
  {
    key: 'log_retention_days_postgres',
    label: 'PostgreSQL',
    description: '访问日志与可观测指标统一保留天数',
  },
  {
    key: 'log_retention_days_sqlite',
    label: 'SQLite',
    description: 'SQLite 日志保留天数',
  },
  {
    key: 'log_retention_days_clickhouse',
    label: 'ClickHouse',
    description: 'ClickHouse 日志保留天数',
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
        const config = businessConfigs[field.key];
        if (!config) {
          throw new Error(`缺少配置项: ${field.key}`);
        }
        await services.adminSystemConfig.updateSystemConfig(field.key, {
          value: values[field.key] ?? '90',
          description: config.description,
        });
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'system-configs'],
      });
      toast.success('日志保留时间已更新');
    },
    onError: (error: Error) => {
      toast.error(error.message || '更新日志保留时间失败');
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
      toast.success('文件访问白名单已更新');
    },
    onError: (error: Error) => {
      toast.error(error.message || '更新白名单失败');
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
    return types.map((t) => {
      let label = t;
      if (t === 'avatar') label = '头像 (avatar)';
      else if (t === 'attachment') label = '附件 (attachment)';
      else if (t === 'doc') label = '文档 (doc)';
      else if (t === 'generic') label = '通用 (generic)';
      return { value: t, label };
    });
  }, [uploadTypesQuery.data]);

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
                文件访问权限控制
              </CardTitle>
              <CardDescription className='text-xs'>
                配置免登录直接访问的文件业务类型。不在白名单内的文件将要求登录鉴权。
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className='pt-6 space-y-4'>
          <div className='flex flex-col gap-4'>
            <div className='flex items-center gap-3'>
              <span className='text-sm font-medium text-muted-foreground'>
                添加免鉴权类型:
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
                  <SelectValue placeholder='选择业务类型...' />
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
                      所有类型已添加
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
                  当前免鉴权列表
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
                  白名单已空，所有类型文件的访问都将需要登录。
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
                日志保留时间
              </CardTitle>
              <CardDescription className='text-xs'>
                配置各日志数据库的日志保留天数，切换日志数据库后自动按对应配置清理过期日志。
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
                    天
                  </span>
                </div>
                <p className='text-[10px] text-muted-foreground'>
                  {field.description}
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
              保存
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* 通知模板管理 */}
      <TemplatesManager />
    </div>
  );
}
