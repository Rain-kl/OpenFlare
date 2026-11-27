'use client';

import { Fragment, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import {
  Activity,
  ChevronDown,
  ChevronRight,
  Loader2,
  Play,
} from 'lucide-react';
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
import type {
  NetToolsPingPayload,
  NetToolsPingProtocol,
  NetToolsProbeTaskState,
} from '@/lib/services/openflare';
import { NetworkToolsService } from '@/lib/services/openflare';

import { NodeSelector } from '../shared/node-selector';
import {
  formatNetToolsMs,
  formatNetToolsPercent,
  getNetToolsErrorMessage,
  getTaskStatusVariant,
} from '../shared/utils';

export function NetworkToolsPingPageClient() {
  const t = useTranslations('networkTools');

  const [target, setTarget] = useState('');
  const [protocol, setProtocol] = useState<NetToolsPingProtocol>('icmp');
  const [port, setPort] = useState('');
  const [count, setCount] = useState('4');
  const [intervalMs, setIntervalMs] = useState('200');
  const [timeoutMs, setTimeoutMs] = useState('3000');
  const [selectedNodes, setSelectedNodes] = useState<string[]>([]);
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [expandedNodes, setExpandedNodes] = useState<Record<string, boolean>>(
    {},
  );

  const dispatchMutation = useMutation({
    mutationFn: (payload: NetToolsPingPayload) =>
      NetworkToolsService.dispatchPing(payload),
    onSuccess: (state) => {
      setActiveTaskId(state.task_id);
      setExpandedNodes({});
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

  const task = taskQuery.data ?? dispatchMutation.data ?? null;

  const summary = useMemo(() => {
    const nodes = task?.nodes ?? [];
    const pings = nodes
      .map((n) => n.ping)
      .filter((p): p is NonNullable<typeof p> => Boolean(p && p.received > 0));
    if (pings.length === 0) {
      return {
        responded: nodes.filter((n) => n.status === 'completed').length,
        total: nodes.length,
        minAvg: null as number | null,
        meanAvg: null as number | null,
        worstP95: null as number | null,
      };
    }
    const minAvg = Math.min(...pings.map((p) => p.avg_ms));
    const meanAvg = pings.reduce((acc, p) => acc + p.avg_ms, 0) / pings.length;
    const worstP95 = Math.max(...pings.map((p) => p.p95_ms));
    return {
      responded: nodes.filter((n) => n.status === 'completed').length,
      total: nodes.length,
      minAvg,
      meanAvg,
      worstP95,
    };
  }, [task]);

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
      count: Number(count) || 4,
      interval_ms: Number(intervalMs) || 200,
      timeout_ms: Number(timeoutMs) || 3000,
      node_ids: selectedNodes.length > 0 ? selectedNodes : undefined,
    });
  };

  return (
    <div className='w-full py-6 px-1 space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div className='flex items-center gap-2'>
          <Activity className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('ping.title')}
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

      <p className='text-sm text-muted-foreground'>{t('ping.description')}</p>

      <Card>
        <CardContent className='pt-6'>
          <form onSubmit={handleSubmit} className='space-y-4'>
            <div className='grid grid-cols-1 gap-4 md:grid-cols-12'>
              <div className='space-y-2 md:col-span-5'>
                <Label htmlFor='ping-target'>{t('common.target')}</Label>
                <Input
                  id='ping-target'
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  placeholder={t('common.targetPlaceholder')}
                />
                <p className='text-xs text-muted-foreground'>
                  {t('ping.targetHint')}
                </p>
              </div>
              <div className='space-y-2 md:col-span-2'>
                <Label>{t('common.protocol')}</Label>
                <Select
                  value={protocol}
                  onValueChange={(v) => setProtocol(v as NetToolsPingProtocol)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='icmp'>ICMP</SelectItem>
                    <SelectItem value='tcp'>TCPing</SelectItem>
                    <SelectItem value='http'>HTTP</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-2 md:col-span-1'>
                <Label htmlFor='ping-port'>{t('common.port')}</Label>
                <Input
                  id='ping-port'
                  type='number'
                  min={1}
                  max={65535}
                  disabled={protocol === 'icmp'}
                  value={port}
                  onChange={(e) => setPort(e.target.value)}
                  placeholder={protocol === 'tcp' ? '443' : '80'}
                />
              </div>
              <div className='space-y-2 md:col-span-1'>
                <Label htmlFor='ping-count'>{t('ping.count')}</Label>
                <Input
                  id='ping-count'
                  type='number'
                  min={1}
                  max={10}
                  value={count}
                  onChange={(e) => setCount(e.target.value)}
                />
              </div>
              <div className='space-y-2 md:col-span-1'>
                <Label htmlFor='ping-interval'>{t('ping.intervalMs')}</Label>
                <Input
                  id='ping-interval'
                  type='number'
                  min={100}
                  max={2000}
                  value={intervalMs}
                  onChange={(e) => setIntervalMs(e.target.value)}
                />
              </div>
              <div className='space-y-2 md:col-span-2'>
                <Label htmlFor='ping-timeout'>{t('ping.timeoutMs')}</Label>
                <Input
                  id='ping-timeout'
                  type='number'
                  min={500}
                  max={10000}
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
                {dispatchMutation.isPending || task?.status === 'running' ? (
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

      <NodeSelector selectedIds={selectedNodes} onChange={setSelectedNodes} />

      {task ? (
        <>
          <div className='grid grid-cols-2 gap-4 lg:grid-cols-4'>
            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-xs font-medium text-muted-foreground'>
                  {t('ping.summary.responded')}
                </CardTitle>
              </CardHeader>
              <CardContent className='text-xl font-semibold'>
                {summary.responded} / {summary.total}
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-xs font-medium text-muted-foreground'>
                  {t('ping.summary.minLatency')}
                </CardTitle>
              </CardHeader>
              <CardContent className='text-xl font-semibold'>
                {summary.minAvg !== null
                  ? t('common.msUnit', {
                      value: formatNetToolsMs(summary.minAvg),
                    })
                  : '—'}
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-xs font-medium text-muted-foreground'>
                  {t('ping.summary.overallAvg')}
                </CardTitle>
              </CardHeader>
              <CardContent className='text-xl font-semibold'>
                {summary.meanAvg !== null
                  ? t('common.msUnit', {
                      value: formatNetToolsMs(summary.meanAvg),
                    })
                  : '—'}
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-xs font-medium text-muted-foreground'>
                  {t('ping.summary.worstP95')}
                </CardTitle>
              </CardHeader>
              <CardContent className='text-xl font-semibold'>
                {summary.worstP95 !== null
                  ? t('common.msUnit', {
                      value: formatNetToolsMs(summary.worstP95),
                    })
                  : '—'}
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardContent className='pt-6'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-8' />
                    <TableHead>{t('ping.columns.node')}</TableHead>
                    <TableHead>{t('ping.columns.status')}</TableHead>
                    <TableHead>{t('ping.columns.resolvedIp')}</TableHead>
                    <TableHead>{t('ping.columns.sentRecv')}</TableHead>
                    <TableHead>{t('ping.columns.loss')}</TableHead>
                    <TableHead>{t('ping.columns.min')}</TableHead>
                    <TableHead>{t('ping.columns.avg')}</TableHead>
                    <TableHead>{t('ping.columns.p95')}</TableHead>
                    <TableHead>{t('ping.columns.jitter')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {task.nodes.map((node) => {
                    const p = node.ping;
                    const expanded = Boolean(expandedNodes[node.node_id]);
                    return (
                      <Fragment key={node.node_id}>
                        <TableRow
                          className='cursor-pointer'
                          onClick={() =>
                            setExpandedNodes((prev) => ({
                              ...prev,
                              [node.node_id]: !prev[node.node_id],
                            }))
                          }
                        >
                          <TableCell>
                            {p?.samples?.length ? (
                              expanded ? (
                                <ChevronDown className='size-4' />
                              ) : (
                                <ChevronRight className='size-4' />
                              )
                            ) : null}
                          </TableCell>
                          <TableCell className='font-medium'>
                            <div>{node.node_name || node.node_id}</div>
                            <div className='text-xs text-muted-foreground'>
                              {node.node_geo || '—'}
                              {node.node_asn ? ` · AS${node.node_asn}` : ''}
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant={getTaskStatusVariant(node.status)}>
                              {t(`common.statusLabels.${node.status}`)}
                            </Badge>
                          </TableCell>
                          <TableCell className='font-mono text-xs'>
                            {p?.resolved_ip || '—'}
                          </TableCell>
                          <TableCell>
                            {p ? `${p.received} / ${p.sent}` : '—'}
                          </TableCell>
                          <TableCell>
                            {p
                              ? t('common.pctUnit', {
                                  value: formatNetToolsPercent(p.loss_rate),
                                })
                              : '—'}
                          </TableCell>
                          <TableCell>{formatNetToolsMs(p?.min_ms)}</TableCell>
                          <TableCell className='font-semibold'>
                            {formatNetToolsMs(p?.avg_ms)}
                          </TableCell>
                          <TableCell>{formatNetToolsMs(p?.p95_ms)}</TableCell>
                          <TableCell>
                            {formatNetToolsMs(p?.jitter_ms)}
                          </TableCell>
                        </TableRow>
                        {expanded && (p?.samples?.length || node.error) ? (
                          <TableRow>
                            <TableCell colSpan={10} className='bg-muted/30'>
                              {node.error || p?.error ? (
                                <p className='text-xs text-destructive'>
                                  {node.error || p?.error}
                                </p>
                              ) : null}
                              {p?.samples?.length ? (
                                <div className='flex flex-wrap gap-2 py-1'>
                                  {p.samples.map((s) => (
                                    <Badge
                                      key={s.seq}
                                      variant={
                                        s.success ? 'outline' : 'destructive'
                                      }
                                      className='font-mono text-xs'
                                    >
                                      {s.success
                                        ? t('ping.sampleSeq', {
                                            seq: s.seq,
                                            rtt: s.rtt_ms,
                                          })
                                        : t('ping.sampleTimeout', {
                                            seq: s.seq,
                                          })}
                                    </Badge>
                                  ))}
                                </div>
                              ) : null}
                            </TableCell>
                          </TableRow>
                        ) : null}
                      </Fragment>
                    );
                  })}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </>
      ) : (
        <div className='rounded-xl border border-dashed py-12 text-center text-sm text-muted-foreground'>
          {t('ping.emptyState')}
        </div>
      )}
    </div>
  );
}
