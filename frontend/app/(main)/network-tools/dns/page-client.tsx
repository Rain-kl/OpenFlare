'use client';

import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Globe, Loader2, Play } from 'lucide-react';
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
  NetToolsDNSQueryPayload,
  NetToolsDNSQueryResponse,
  NetToolsProbeTaskState,
} from '@/lib/services/openflare';
import { NetworkToolsService } from '@/lib/services/openflare';

import { NodeSelector } from '../shared/node-selector';
import {
  formatNetToolsMs,
  getNetToolsErrorMessage,
  getTaskStatusVariant,
} from '../shared/utils';

const DNS_RECORD_TYPES = [
  'A',
  'AAAA',
  'CNAME',
  'MX',
  'TXT',
  'NS',
  'SOA',
  'CAA',
  'HTTPS',
  'SRV',
  'PTR',
] as const;

const RESOLVER_PRESETS = [
  '1.1.1.1:53',
  '8.8.8.8:53',
  '223.5.5.5:53',
  '119.29.29.29:53',
  'custom',
] as const;

export function NetworkToolsDnsPageClient() {
  const t = useTranslations('networkTools');

  const [domain, setDomain] = useState('');
  const [recordType, setRecordType] = useState('A');
  const [resolverPreset, setResolverPreset] = useState('1.1.1.1:53');
  const [customResolver, setCustomResolver] = useState('');
  const [ecsSubnet, setEcsSubnet] = useState('');
  const [mode, setMode] = useState<'direct' | 'nodes'>('direct');
  const [selectedNodes, setSelectedNodes] = useState<string[]>([]);
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [lastDirectResult, setLastDirectResult] =
    useState<NetToolsDNSQueryResponse['direct_result']>();

  const queryMutation = useMutation({
    mutationFn: (payload: NetToolsDNSQueryPayload) =>
      NetworkToolsService.queryDNS(payload),
    onSuccess: (res) => {
      if (res.mode === 'direct' && res.direct_result) {
        setLastDirectResult(res.direct_result);
        setActiveTaskId(null);
      } else if (res.task) {
        setLastDirectResult(undefined);
        setActiveTaskId(res.task.task_id);
      }
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

  const task = taskQuery.data ?? queryMutation.data?.task ?? null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = domain.trim();
    if (!trimmed) return;
    const resolver =
      resolverPreset === 'custom' ? customResolver.trim() : resolverPreset;
    queryMutation.mutate({
      domain: trimmed,
      record_type: recordType,
      resolver: resolver || undefined,
      ecs_subnet: ecsSubnet.trim() || undefined,
      node_ids:
        mode === 'nodes' && selectedNodes.length > 0
          ? selectedNodes
          : undefined,
    });
  };

  return (
    <div className='w-full py-6 px-1 space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div className='flex items-center gap-2'>
          <Globe className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('dns.title')}
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

      <p className='text-sm text-muted-foreground'>{t('dns.description')}</p>

      <Card>
        <CardContent className='pt-6'>
          <form onSubmit={handleSubmit} className='space-y-4'>
            <div className='grid grid-cols-1 gap-4 md:grid-cols-12'>
              <div className='space-y-2 md:col-span-4'>
                <Label htmlFor='dns-domain'>{t('dns.domain')}</Label>
                <Input
                  id='dns-domain'
                  value={domain}
                  onChange={(e) => setDomain(e.target.value)}
                  placeholder={t('dns.domainPlaceholder')}
                />
              </div>
              <div className='space-y-2 md:col-span-2'>
                <Label>{t('dns.recordType')}</Label>
                <Select value={recordType} onValueChange={setRecordType}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {DNS_RECORD_TYPES.map((rt) => (
                      <SelectItem key={rt} value={rt}>
                        {rt}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-2 md:col-span-2'>
                <Label>{t('dns.resolver')}</Label>
                <Select
                  value={resolverPreset}
                  onValueChange={setResolverPreset}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {RESOLVER_PRESETS.map((preset) => (
                      <SelectItem key={preset} value={preset}>
                        {preset === 'custom' ? t('dns.customResolver') : preset}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {resolverPreset === 'custom' ? (
                <div className='space-y-2 md:col-span-2'>
                  <Label htmlFor='dns-custom-resolver'>
                    {t('dns.customResolver')}
                  </Label>
                  <Input
                    id='dns-custom-resolver'
                    value={customResolver}
                    onChange={(e) => setCustomResolver(e.target.value)}
                    placeholder={t('dns.customResolverPlaceholder')}
                  />
                </div>
              ) : null}
              <div
                className={
                  resolverPreset === 'custom'
                    ? 'space-y-2 md:col-span-2'
                    : 'space-y-2 md:col-span-2'
                }
              >
                <Label htmlFor='dns-ecs'>{t('dns.ecsSubnet')}</Label>
                <Input
                  id='dns-ecs'
                  value={ecsSubnet}
                  onChange={(e) => setEcsSubnet(e.target.value)}
                  placeholder={t('dns.ecsPlaceholder')}
                />
              </div>
              <div className='space-y-2 md:col-span-2'>
                <Label>{t('dns.mode')}</Label>
                <Select
                  value={mode}
                  onValueChange={(v) => setMode(v as 'direct' | 'nodes')}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='direct'>
                      {t('dns.modeDirect')}
                    </SelectItem>
                    <SelectItem value='nodes'>{t('dns.modeNodes')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className='flex justify-end'>
              <Button
                type='submit'
                disabled={queryMutation.isPending || !domain.trim()}
              >
                {queryMutation.isPending || task?.status === 'running' ? (
                  <Loader2 className='mr-2 size-4 animate-spin' />
                ) : (
                  <Play className='mr-2 size-4' />
                )}
                {t('dns.runQuery')}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {mode === 'nodes' ? (
        <NodeSelector selectedIds={selectedNodes} onChange={setSelectedNodes} />
      ) : null}

      {lastDirectResult ? (
        <Card>
          <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-2 pb-3'>
            <CardTitle className='text-base font-medium'>
              {lastDirectResult.domain} ({lastDirectResult.record_type}) ·{' '}
              {lastDirectResult.resolver}
            </CardTitle>
            <div className='flex flex-wrap items-center gap-2'>
              <Badge variant='default'>
                {t('dns.flags.rcode', { rcode: lastDirectResult.rcode })}
              </Badge>
              <Badge variant='outline'>
                {lastDirectResult.authoritative
                  ? t('dns.flags.authoritative')
                  : t('dns.flags.nonAuthoritative')}
              </Badge>
              <Badge variant='outline'>
                {lastDirectResult.used_tcp
                  ? t('dns.flags.tcpFallback')
                  : t('dns.flags.udp')}
              </Badge>
              <Badge variant='secondary'>
                {t('dns.flags.latency', {
                  ms: formatNetToolsMs(lastDirectResult.latency_ms),
                })}
              </Badge>
            </div>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('dns.columns.name')}</TableHead>
                  <TableHead>{t('dns.columns.type')}</TableHead>
                  <TableHead>{t('dns.columns.ttl')}</TableHead>
                  <TableHead>{t('dns.columns.value')}</TableHead>
                  <TableHead>{t('dns.columns.geoAsn')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {lastDirectResult.records.map((rec, idx) => (
                  <TableRow key={`${rec.name}-${rec.value}-${idx}`}>
                    <TableCell className='font-mono text-xs'>
                      {rec.name}
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline'>{rec.record_type}</Badge>
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      {rec.ttl}
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      {rec.value}
                    </TableCell>
                    <TableCell className='text-xs'>
                      {rec.asn || rec.country_iso ? (
                        <div className='flex flex-wrap items-center gap-1'>
                          {rec.country_iso ? (
                            <Badge variant='outline'>{rec.country_iso}</Badge>
                          ) : null}
                          {rec.city_name ? <span>{rec.city_name}</span> : null}
                          {rec.asn ? (
                            <Badge variant='secondary' className='font-mono'>
                              AS{rec.asn}
                            </Badge>
                          ) : null}
                          {rec.asn_name ? (
                            <span className='text-muted-foreground'>
                              {rec.asn_name}
                            </span>
                          ) : null}
                        </div>
                      ) : (
                        '—'
                      )}
                    </TableCell>
                  </TableRow>
                ))}
                {lastDirectResult.records.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={5}
                      className='py-6 text-center text-muted-foreground'
                    >
                      {t('common.noData')}
                    </TableCell>
                  </TableRow>
                ) : null}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : task ? (
        <div className='space-y-4'>
          {task.nodes.map((node) => {
            const dnsRes = node.dns;
            return (
              <Card key={node.node_id}>
                <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-2 pb-3'>
                  <CardTitle className='text-base font-medium'>
                    {node.node_name || node.node_id}{' '}
                    {node.node_geo ? `(${node.node_geo})` : ''}
                  </CardTitle>
                  <div className='flex flex-wrap items-center gap-2'>
                    {dnsRes ? (
                      <>
                        <Badge variant='outline'>
                          {t('dns.flags.rcode', { rcode: dnsRes.rcode })}
                        </Badge>
                        <Badge variant='secondary'>
                          {t('dns.flags.latency', {
                            ms: formatNetToolsMs(dnsRes.latency_ms),
                          })}
                        </Badge>
                      </>
                    ) : null}
                    <Badge variant={getTaskStatusVariant(node.status)}>
                      {t(`common.statusLabels.${node.status}`)}
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent>
                  {node.error || dnsRes?.error ? (
                    <p className='text-xs text-destructive'>
                      {node.error || dnsRes?.error}
                    </p>
                  ) : dnsRes?.records?.length ? (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t('dns.columns.name')}</TableHead>
                          <TableHead>{t('dns.columns.type')}</TableHead>
                          <TableHead>{t('dns.columns.ttl')}</TableHead>
                          <TableHead>{t('dns.columns.value')}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {dnsRes.records.map((rec, idx) => (
                          <TableRow key={`${rec.name}-${rec.value}-${idx}`}>
                            <TableCell className='font-mono text-xs'>
                              {rec.name}
                            </TableCell>
                            <TableCell>{rec.record_type}</TableCell>
                            <TableCell className='font-mono text-xs'>
                              {rec.ttl}
                            </TableCell>
                            <TableCell className='font-mono text-xs'>
                              {rec.value}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  ) : (
                    <p className='text-xs text-muted-foreground'>
                      {t('common.noData')}
                    </p>
                  )}
                </CardContent>
              </Card>
            );
          })}
        </div>
      ) : (
        <div className='rounded-xl border border-dashed py-12 text-center text-sm text-muted-foreground'>
          {t('dns.emptyState')}
        </div>
      )}
    </div>
  );
}
