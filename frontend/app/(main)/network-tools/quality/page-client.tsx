'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Database, Gauge, Loader2, RefreshCw } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { NetworkToolsService } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import {
  formatNetToolsBytes,
  formatNetToolsMs,
  formatNetToolsPercent,
  formatNetToolsTime,
  getNetToolsErrorMessage,
} from '../shared/utils';

export function NetworkToolsQualityPageClient() {
  const t = useTranslations('networkTools');
  const queryClient = useQueryClient();

  const [tab, setTab] = useState<'matrix' | 'datasets'>('matrix');
  const [search, setSearch] = useState('');
  const [dataSource, setDataSource] = useState('all');
  const [page, setPage] = useState(1);

  const matrixQuery = useQuery({
    queryKey: [
      'openflare',
      'network-tools',
      'quality-matrix',
      page,
      search,
      dataSource,
    ],
    queryFn: () =>
      NetworkToolsService.listQualityMatrix({
        p: page,
        page_size: 20,
        search: search.trim() || undefined,
        data_source: dataSource === 'all' ? undefined : dataSource,
      }),
    enabled: tab === 'matrix',
  });

  const statusQuery = useQuery({
    queryKey: ['openflare', 'network-tools', 'netintel-status'],
    queryFn: () => NetworkToolsService.getNetIntelStatus(),
    enabled: tab === 'datasets',
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

  const totalPages = Math.max(
    1,
    Math.ceil((matrixQuery.data?.total ?? 0) / 20),
  );

  return (
    <div className='w-full py-6 px-1 space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div className='flex items-center gap-2'>
          <Gauge className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('quality.title')}
          </h1>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => {
            if (tab === 'matrix') {
              void matrixQuery.refetch();
            } else {
              void statusQuery.refetch();
            }
          }}
        >
          <RefreshCw className='mr-1.5 size-3.5' />
          {t('common.refresh')}
        </Button>
      </div>

      <p className='text-sm text-muted-foreground'>
        {t('quality.description')}
      </p>

      <Tabs
        value={tab}
        onValueChange={(v) => setTab(v as 'matrix' | 'datasets')}
        className='w-full gap-0'
      >
        <TabsList
          variant='line'
          className='h-auto w-full justify-start gap-6 bg-transparent p-0'
        >
          <TabsTrigger
            value='matrix'
            className={cn(
              'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
              'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
            )}
          >
            {t('quality.tabs.matrix')}
          </TabsTrigger>
          <TabsTrigger
            value='datasets'
            className={cn(
              'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
              'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
            )}
          >
            <Database className='mr-1.5 size-4' />
            {t('quality.tabs.datasets')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value='matrix' className='mt-6 space-y-4 outline-none'>
          <div className='flex flex-wrap items-center gap-3'>
            <Input
              className='max-w-sm'
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setPage(1);
              }}
              placeholder={t('quality.filters.searchPlaceholder')}
            />
            <Select
              value={dataSource}
              onValueChange={(v) => {
                setDataSource(v);
                setPage(1);
              }}
            >
              <SelectTrigger className='w-56'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>
                  {t('quality.filters.sourceAll')}
                </SelectItem>
                <SelectItem value='measured'>
                  {t('quality.filters.sourceMeasured')}
                </SelectItem>
                <SelectItem value='predicted'>
                  {t('quality.filters.sourcePredicted')}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Card>
            <CardContent className='pt-6'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('quality.columns.node')}</TableHead>
                    <TableHead>{t('quality.columns.clientAsn')}</TableHead>
                    <TableHead>{t('quality.columns.effectiveRtt')}</TableHead>
                    <TableHead>{t('quality.columns.tcpStats')}</TableHead>
                    <TableHead>{t('quality.columns.traceStats')}</TableHead>
                    <TableHead>{t('quality.columns.score')}</TableHead>
                    <TableHead>{t('quality.columns.confidence')}</TableHead>
                    <TableHead>{t('quality.columns.source')}</TableHead>
                    <TableHead>{t('quality.columns.updatedAt')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(matrixQuery.data?.results ?? []).map((row) => (
                    <TableRow key={row.id}>
                      <TableCell className='text-xs'>
                        <div className='font-medium'>
                          {row.node_name || row.node_id}
                        </div>
                        <div className='text-muted-foreground'>
                          {row.node_geo || '—'}
                          {row.node_asn ? ` · AS${row.node_asn}` : ''}
                        </div>
                      </TableCell>
                      <TableCell className='text-xs'>
                        <div className='font-mono font-medium'>
                          AS{row.client_asn}
                          {row.client_country ? ` · ${row.client_country}` : ''}
                        </div>
                        <div className='text-muted-foreground'>
                          {row.client_asn_name || '—'}
                        </div>
                      </TableCell>
                      <TableCell className='font-mono text-xs font-semibold'>
                        {t('common.msUnit', {
                          value: formatNetToolsMs(row.effective_rtt_ms),
                        })}
                      </TableCell>
                      <TableCell className='font-mono text-xs'>
                        {row.tcp_sample_count > 0
                          ? `${formatNetToolsMs(row.tcp_rtt_p50_ms)} / ${formatNetToolsMs(row.tcp_rtt_p95_ms)} ms · ${formatNetToolsPercent(row.tcp_retrans_rate * 100)}%`
                          : '—'}
                      </TableCell>
                      <TableCell className='font-mono text-xs'>
                        {row.trace_sample_count > 0 || row.trace_hop_count > 0
                          ? `${formatNetToolsMs(row.trace_rtt_p50_ms)} ms · ${formatNetToolsPercent(row.trace_loss_rate * 100)}% · ${row.trace_hop_count}`
                          : '—'}
                      </TableCell>
                      <TableCell className='font-mono text-xs font-semibold'>
                        {row.quality_score.toFixed(1)}
                      </TableCell>
                      <TableCell className='font-mono text-xs'>
                        {t('common.pctUnit', {
                          value: formatNetToolsPercent(row.confidence * 100),
                        })}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            row.data_source === 'measured'
                              ? 'default'
                              : 'secondary'
                          }
                        >
                          {row.data_source}
                        </Badge>
                      </TableCell>
                      <TableCell className='text-xs text-muted-foreground'>
                        {formatNetToolsTime(row.updated_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                  {!matrixQuery.isLoading &&
                  (matrixQuery.data?.results?.length ?? 0) === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={9}
                        className='py-8 text-center text-muted-foreground'
                      >
                        {t('common.noData')}
                      </TableCell>
                    </TableRow>
                  ) : null}
                </TableBody>
              </Table>

              <div className='mt-4 flex items-center justify-between text-xs text-muted-foreground'>
                <span>
                  {t('common.pageSummary', {
                    page,
                    totalPages,
                    total: matrixQuery.data?.total ?? 0,
                  })}
                </span>
                <div className='flex gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={page <= 1}
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                  >
                    {t('common.prevPage')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={page >= totalPages}
                    onClick={() => setPage((p) => p + 1)}
                  >
                    {t('common.nextPage')}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value='datasets' className='mt-6 space-y-6 outline-none'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='flex flex-wrap items-center gap-2 text-xs'>
              <Badge variant='outline' className='font-mono'>
                {t('quality.datasets.dataDir', {
                  dir: statusQuery.data?.data_dir || 'data/netintel',
                })}
              </Badge>
              <Badge
                variant={
                  statusQuery.data?.caida_enabled ? 'default' : 'secondary'
                }
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
                    {t('quality.datasets.records', { count: ds.record_count })}{' '}
                    ·{' '}
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
                  {ds.error ? (
                    <p className='text-destructive'>{ds.error}</p>
                  ) : null}
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
        </TabsContent>
      </Tabs>
    </div>
  );
}
