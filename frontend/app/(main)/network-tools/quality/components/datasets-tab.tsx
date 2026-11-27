'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, RefreshCw } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { NetworkToolsService } from '@/lib/services/openflare';

import {
  formatNetToolsBytes,
  formatNetToolsTime,
  getNetToolsErrorMessage,
} from '../../shared/utils';

interface DatasetsTabProps {
  refreshNonce: number;
}

export function DatasetsTab({ refreshNonce }: DatasetsTabProps) {
  const t = useTranslations('networkTools');
  const queryClient = useQueryClient();

  const statusQuery = useQuery({
    queryKey: ['openflare', 'network-tools', 'netintel-status', refreshNonce],
    queryFn: () => NetworkToolsService.getNetIntelStatus(),
  });

  const syncMutation = useMutation({
    mutationFn: (force: boolean) =>
      NetworkToolsService.triggerNetIntelSync(force),
    onSuccess: async (res) => {
      if (res.mode === 'queued' && res.task_id) {
        toast.success(
          t('quality.datasets.syncSuccessQueued', { taskId: res.task_id }),
        );
      } else if (res.summary) {
        toast.success(
          t('quality.datasets.syncSuccessInline', {
            updated: res.summary.updated_files.length,
            backfilled: res.summary.backfilled_nodes,
          }),
        );
      }
      await queryClient.invalidateQueries({
        queryKey: ['openflare', 'network-tools', 'netintel-status'],
      });
    },
    onError: (err) =>
      toast.error(getNetToolsErrorMessage(err, t('common.error'))),
  });

  return (
    <div className='space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex flex-wrap items-center gap-2 text-xs'>
          <Badge variant='outline' className='font-mono'>
            {t('quality.datasets.dataDir', {
              dir: statusQuery.data?.data_dir || 'data/netintel',
            })}
          </Badge>
          <Badge
            variant={statusQuery.data?.caida_enabled ? 'default' : 'secondary'}
          >
            {statusQuery.data?.caida_enabled
              ? t('quality.datasets.caidaEnabled')
              : t('quality.datasets.caidaDisabled')}
          </Badge>
          <Badge
            variant={
              statusQuery.data?.auto_probe_enabled ? 'default' : 'outline'
            }
          >
            {statusQuery.data?.auto_probe_enabled
              ? t('quality.datasets.autoProbeEnabled')
              : t('quality.datasets.autoProbeDisabled')}
          </Badge>
        </div>
        <div className='flex items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={syncMutation.isPending}
            onClick={() => syncMutation.mutate(false)}
          >
            {syncMutation.isPending ? (
              <Loader2 className='mr-1.5 size-3.5 animate-spin' />
            ) : (
              <RefreshCw className='mr-1.5 size-3.5' />
            )}
            {t('quality.datasets.syncNow')}
          </Button>
          <Button
            type='button'
            size='sm'
            disabled={syncMutation.isPending}
            onClick={() => syncMutation.mutate(true)}
          >
            {t('quality.datasets.forceSync')}
          </Button>
        </div>
      </div>

      <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3'>
        {(statusQuery.data?.datasets ?? []).map((ds) => (
          <Card key={ds.key}>
            <CardHeader className='flex flex-row items-center justify-between gap-2 pb-2'>
              <CardTitle className='font-mono text-sm font-medium'>
                {ds.filename}
              </CardTitle>
              <Badge variant={ds.loaded ? 'default' : 'outline'}>
                {ds.loaded
                  ? t('quality.datasets.loaded')
                  : t('quality.datasets.notLoaded')}
              </Badge>
            </CardHeader>
            <CardContent className='space-y-1.5 text-xs text-muted-foreground'>
              <div>
                {t('quality.datasets.records', { count: ds.record_count })} ·{' '}
                {t('quality.datasets.size', {
                  size: formatNetToolsBytes(ds.size_bytes),
                })}
              </div>
              <div className='truncate' title={ds.license}>
                {t('quality.datasets.license', {
                  license: ds.license || '—',
                })}
              </div>
              <div>
                {t('quality.datasets.updated', {
                  time: formatNetToolsTime(ds.updated_at),
                })}
              </div>
              {ds.error ? <p className='text-destructive'>{ds.error}</p> : null}
            </CardContent>
          </Card>
        ))}
      </div>

      <p className='text-xs text-muted-foreground'>
        {t('common.attributionPrefix')}
        <a
          href='https://bgp.tools'
          target='_blank'
          rel='noreferrer'
          className='underline underline-offset-4 hover:text-foreground'
        >
          Data powered by bgp.tools
        </a>
      </p>
    </div>
  );
}
