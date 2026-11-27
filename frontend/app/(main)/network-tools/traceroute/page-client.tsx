'use client';

import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { History, Loader2, Play, Route } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
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
import type {
  NetToolsProbeTaskState,
  NetToolsTraceroutePayload,
  NetToolsTracerouteProtocol,
} from '@/lib/services/openflare';
import { NetworkToolsService } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import { NodeSelector } from '../shared/node-selector';
import {
  formatNetToolsMs,
  formatNetToolsPercent,
  formatNetToolsTime,
  getNetToolsErrorMessage,
  getTaskStatusVariant,
} from '../shared/utils';

export function NetworkToolsTraceroutePageClient() {
  const t = useTranslations('networkTools');

  const [tab, setTab] = useState<'live' | 'history'>('live');
  const [target, setTarget] = useState('');
  const [protocol, setProtocol] = useState<NetToolsTracerouteProtocol>('icmp');
  const [port, setPort] = useState('');
  const [maxHops, setMaxHops] = useState('30');
  const [probesPerHop, setProbesPerHop] = useState('3');
  const [timeoutMs, setTimeoutMs] = useState('1500');
  const [selectedNodes, setSelectedNodes] = useState<string[]>([]);
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);

  const [histTarget, setHistTarget] = useState('');
  const [histTriggerType, setHistTriggerType] = useState('all');
  const [histPage, setHistPage] = useState(1);

  const dispatchMutation = useMutation({
    mutationFn: (payload: NetToolsTraceroutePayload) =>
      NetworkToolsService.dispatchTraceroute(payload),
    onSuccess: (state) => {
      setActiveTaskId(state.task_id);
      setTab('live');
    },
    onError: (err) =>
      toast.error(getNetToolsErrorMessage(err, t('common.error'))),
  });

  const taskQuery = useQuery<NetToolsProbeTaskState>({
    queryKey: ['openflare', 'network-tools', 'task', activeTaskId],
    queryFn: () => NetworkToolsService.getTask(activeTaskId ?? ''),
    enabled: Boolean(activeTaskId),
    refetchInterval: (query) =>
      query.state.data?.status === 'running' ? 1500 : false,
  });

  const historyQuery = useQuery({
    queryKey: [
      'openflare',
      'network-tools',
      'traceroute-history',
      histPage,
      histTarget,
      histTriggerType,
    ],
    queryFn: () =>
      NetworkToolsService.listTracerouteHistory({
        p: histPage,
        page_size: 15,
        target: histTarget.trim() || undefined,
        trigger_type: histTriggerType === 'all' ? undefined : histTriggerType,
      }),
    enabled: tab === 'history',
  });

  const task = taskQuery.data ?? dispatchMutation.data ?? null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = target.trim();
    if (!trimmed) {
      return;
    }
    const parsedPort = port.trim() ? Number(port.trim()) : undefined;
    dispatchMutation.mutate({
      target: trimmed,
      protocol,
      port:
        parsedPort && Number.isInteger(parsedPort) && parsedPort > 0
          ? parsedPort
          : undefined,
      max_hops: Number(maxHops) || 30,
      probes_per_hop: Number(probesPerHop) || 3,
      timeout_ms: Number(timeoutMs) || 1500,
      node_ids: selectedNodes.length > 0 ? selectedNodes : undefined,
    });
  };

  const totalHistoryPages = Math.max(
    1,
    Math.ceil((historyQuery.data?.total ?? 0) / 15),
  );

  return (
    <div className='w-full py-6 px-1 space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div className='flex items-center gap-2'>
          <Route className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('traceroute.title')}
          </h1>
        </div>
        {task ? (
          <Badge variant={getTaskStatusVariant(task.status)}>
            {t('common.taskProgress', {
              completed: task.completed_nodes,
              total: task.total_nodes,
            })}
          </Badge>
        ) : null}
      </div>

      <p className='text-sm text-muted-foreground'>
        {t('traceroute.description')}
      </p>

      <Tabs
        value={tab}
        onValueChange={(v) => setTab(v as 'live' | 'history')}
        className='w-full gap-0'
      >
        <TabsList
          variant='line'
          className='h-auto w-full justify-start gap-6 bg-transparent p-0'
        >
          <TabsTrigger
            value='live'
            className={cn(
              'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
              'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
            )}
          >
            {t('traceroute.tabs.live')}
          </TabsTrigger>
          <TabsTrigger
            value='history'
            className={cn(
              'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
              'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
            )}
          >
            <History className='mr-1.5 size-4' />
            {t('traceroute.tabs.history')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value='live' className='mt-6 space-y-6 outline-none'>
          <Card>
            <CardContent className='pt-6'>
              <form onSubmit={handleSubmit} className='space-y-4'>
                <div className='grid grid-cols-1 gap-4 md:grid-cols-12'>
                  <div className='space-y-2 md:col-span-5'>
                    <Label htmlFor='trace-target'>{t('common.target')}</Label>
                    <Input
                      id='trace-target'
                      value={target}
                      onChange={(e) => setTarget(e.target.value)}
                      placeholder={t('common.targetPlaceholder')}
                    />
                  </div>
                  <div className='space-y-2 md:col-span-2'>
                    <Label>{t('common.protocol')}</Label>
                    <Select
                      value={protocol}
                      onValueChange={(v) =>
                        setProtocol(v as NetToolsTracerouteProtocol)
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='icmp'>ICMP</SelectItem>
                        <SelectItem value='udp'>UDP</SelectItem>
                        <SelectItem value='tcp'>TCP</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className='space-y-2 md:col-span-1'>
                    <Label htmlFor='trace-port'>{t('common.port')}</Label>
                    <Input
                      id='trace-port'
                      type='number'
                      min={1}
                      max={65535}
                      disabled={protocol === 'icmp'}
                      value={port}
                      onChange={(e) => setPort(e.target.value)}
                      placeholder={protocol === 'tcp' ? '443' : '33434'}
                    />
                  </div>
                  <div className='space-y-2 md:col-span-1'>
                    <Label htmlFor='trace-hops'>
                      {t('traceroute.maxHops')}
                    </Label>
                    <Input
                      id='trace-hops'
                      type='number'
                      min={1}
                      max={40}
                      value={maxHops}
                      onChange={(e) => setMaxHops(e.target.value)}
                    />
                  </div>
                  <div className='space-y-2 md:col-span-1'>
                    <Label htmlFor='trace-probes'>
                      {t('traceroute.probesPerHop')}
                    </Label>
                    <Input
                      id='trace-probes'
                      type='number'
                      min={1}
                      max={5}
                      value={probesPerHop}
                      onChange={(e) => setProbesPerHop(e.target.value)}
                    />
                  </div>
                  <div className='space-y-2 md:col-span-2'>
                    <Label htmlFor='trace-timeout'>
                      {t('traceroute.timeoutMs')}
                    </Label>
                    <Input
                      id='trace-timeout'
                      type='number'
                      min={500}
                      max={5000}
                      value={timeoutMs}
                      onChange={(e) => setTimeoutMs(e.target.value)}
                    />
                  </div>
                </div>
                <div className='flex justify-end'>
                  <Button
                    type='submit'
                    disabled={dispatchMutation.isPending || !target.trim()}
                  >
                    {dispatchMutation.isPending ||
                    task?.status === 'running' ? (
                      <Loader2 className='mr-2 size-4 animate-spin' />
                    ) : (
                      <Play className='mr-2 size-4' />
                    )}
                    {dispatchMutation.isPending
                      ? t('common.running')
                      : t('common.dispatch')}
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>

          <NodeSelector
            selectedIds={selectedNodes}
            onChange={setSelectedNodes}
          />

          {task ? (
            <div className='space-y-4'>
              {task.nodes.map((node) => {
                const tr = node.traceroute;
                return (
                  <Card key={node.node_id}>
                    <CardHeader className='pb-3'>
                      <div className='flex flex-wrap items-center justify-between gap-2'>
                        <CardTitle className='flex flex-wrap items-center gap-2 text-base font-medium'>
                          <span>{node.node_name || node.node_id}</span>
                          {node.node_geo ? (
                            <Badge variant='outline'>{node.node_geo}</Badge>
                          ) : null}
                          {node.node_asn ? (
                            <Badge variant='secondary' className='font-mono'>
                              AS{node.node_asn}
                            </Badge>
                          ) : null}
                        </CardTitle>
                        <div className='flex items-center gap-2'>
                          {tr ? (
                            <>
                              <Badge
                                variant={tr.reached ? 'default' : 'outline'}
                              >
                                {tr.reached
                                  ? t('traceroute.reached')
                                  : t('traceroute.unreached')}
                              </Badge>
                              <span className='text-xs text-muted-foreground'>
                                {t('traceroute.hopsSummary', {
                                  hops: tr.hop_count,
                                  rtt: formatNetToolsMs(tr.total_rtt_ms),
                                })}
                              </span>
                            </>
                          ) : null}
                          <Badge variant={getTaskStatusVariant(node.status)}>
                            {t(`common.statusLabels.${node.status}`)}
                          </Badge>
                        </div>
                      </div>
                    </CardHeader>
                    <CardContent>
                      {node.error || tr?.error ? (
                        <p className='mb-3 text-xs text-destructive'>
                          {node.error || tr?.error}
                        </p>
                      ) : null}
                      {tr?.hops?.length ? (
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead className='w-16'>
                                {t('traceroute.columns.ttl')}
                              </TableHead>
                              <TableHead>
                                {t('traceroute.columns.ip')}
                              </TableHead>
                              <TableHead>
                                {t('traceroute.columns.location')}
                              </TableHead>
                              <TableHead>
                                {t('traceroute.columns.asn')}
                              </TableHead>
                              <TableHead>
                                {t('traceroute.columns.loss')}
                              </TableHead>
                              <TableHead>
                                {t('traceroute.columns.minAvgMax')}
                              </TableHead>
                              <TableHead>
                                {t('traceroute.columns.jitter')}
                              </TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {tr.hops.map((hop) => (
                              <TableRow key={hop.ttl}>
                                <TableCell className='font-mono'>
                                  {hop.ttl}
                                </TableCell>
                                <TableCell className='font-mono text-xs'>
                                  {hop.ip || '*'}
                                </TableCell>
                                <TableCell className='text-xs'>
                                  {[hop.country_iso, hop.city_name]
                                    .filter(Boolean)
                                    .join(' / ') || '—'}
                                </TableCell>
                                <TableCell className='text-xs'>
                                  {hop.asn ? (
                                    <div className='flex flex-wrap items-center gap-1'>
                                      <Badge
                                        variant='outline'
                                        className='font-mono'
                                      >
                                        AS{hop.asn}
                                      </Badge>
                                      {hop.asn_name ? (
                                        <span>{hop.asn_name}</span>
                                      ) : null}
                                      {(hop.asn_tags ?? [])
                                        .slice(0, 3)
                                        .map((tag) => (
                                          <Badge
                                            key={tag}
                                            variant='secondary'
                                            className='text-[10px]'
                                          >
                                            {tag}
                                          </Badge>
                                        ))}
                                    </div>
                                  ) : (
                                    '—'
                                  )}
                                </TableCell>
                                <TableCell>
                                  {t('common.pctUnit', {
                                    value: formatNetToolsPercent(hop.loss_pct),
                                  })}
                                </TableCell>
                                <TableCell className='font-mono text-xs'>
                                  {formatNetToolsMs(hop.min_ms)} /{' '}
                                  {formatNetToolsMs(hop.avg_ms)} /{' '}
                                  {formatNetToolsMs(hop.max_ms)} ms
                                </TableCell>
                                <TableCell className='font-mono text-xs'>
                                  {formatNetToolsMs(hop.jitter_ms)}
                                </TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      ) : (
                        <p className='text-xs text-muted-foreground'>
                          {t(`common.statusLabels.${node.status}`)}
                        </p>
                      )}
                    </CardContent>
                  </Card>
                );
              })}
            </div>
          ) : (
            <div className='rounded-xl border border-dashed py-12 text-center text-sm text-muted-foreground'>
              {t('traceroute.emptyState')}
            </div>
          )}
        </TabsContent>

        <TabsContent value='history' className='mt-6 space-y-4 outline-none'>
          <div className='flex flex-wrap items-center gap-3'>
            <Input
              className='max-w-xs'
              value={histTarget}
              onChange={(e) => {
                setHistTarget(e.target.value);
                setHistPage(1);
              }}
              placeholder={t('traceroute.history.filterTarget')}
            />
            <Select
              value={histTriggerType}
              onValueChange={(v) => {
                setHistTriggerType(v);
                setHistPage(1);
              }}
            >
              <SelectTrigger className='w-44'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>
                  {t('traceroute.history.typeAll')}
                </SelectItem>
                <SelectItem value='manual'>
                  {t('traceroute.history.typeManual')}
                </SelectItem>
                <SelectItem value='auto'>
                  {t('traceroute.history.typeAuto')}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Card>
            <CardContent className='pt-6'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('traceroute.history.nodeId')}</TableHead>
                    <TableHead>{t('traceroute.history.target')}</TableHead>
                    <TableHead>{t('traceroute.history.filterType')}</TableHead>
                    <TableHead>{t('traceroute.history.hops')}</TableHead>
                    <TableHead>{t('traceroute.history.totalRtt')}</TableHead>
                    <TableHead>{t('traceroute.history.asPath')}</TableHead>
                    <TableHead>{t('traceroute.history.createdAt')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(historyQuery.data?.results ?? []).map((item) => (
                    <TableRow key={item.id}>
                      <TableCell className='font-mono text-xs'>
                        {item.node_id}
                      </TableCell>
                      <TableCell className='text-xs'>
                        <div className='font-medium'>{item.target}</div>
                        <div className='font-mono text-muted-foreground'>
                          {item.resolved_ip}
                          {item.target_asn ? ` · AS${item.target_asn}` : ''}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant='outline'>
                          {item.trigger_type === 'auto'
                            ? t('traceroute.history.typeAuto')
                            : t('traceroute.history.typeManual')}
                        </Badge>
                      </TableCell>
                      <TableCell>{item.hop_count}</TableCell>
                      <TableCell>
                        {t('common.msUnit', {
                          value: formatNetToolsMs(item.total_rtt_ms),
                        })}
                      </TableCell>
                      <TableCell className='font-mono text-xs'>
                        {(item.as_path ?? [])
                          .map((a) => `AS${a}`)
                          .join(' -> ') || '—'}
                      </TableCell>
                      <TableCell className='text-xs text-muted-foreground'>
                        {formatNetToolsTime(item.created_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                  {!historyQuery.isLoading &&
                  (historyQuery.data?.results?.length ?? 0) === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={7}
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
                    page: histPage,
                    totalPages: totalHistoryPages,
                    total: historyQuery.data?.total ?? 0,
                  })}
                </span>
                <div className='flex gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={histPage <= 1}
                    onClick={() => setHistPage((p) => Math.max(1, p - 1))}
                  >
                    {t('common.prevPage')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={histPage >= totalHistoryPages}
                    onClick={() => setHistPage((p) => p + 1)}
                  >
                    {t('common.nextPage')}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
