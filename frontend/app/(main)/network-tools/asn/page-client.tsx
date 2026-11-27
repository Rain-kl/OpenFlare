'use client';

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { GitFork, Loader2, Network, Search } from 'lucide-react';
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
  NetToolsASNDetailResponse,
  NetToolsIPLookupResponse,
} from '@/lib/services/openflare';
import { NetworkToolsService } from '@/lib/services/openflare';

import { formatNetToolsMs, getNetToolsErrorMessage } from '../shared/utils';
import { UpstreamRoutingGraph } from './components/upstream-routing-graph';

type QueryState =
  | { kind: 'asn'; asn: number }
  | { kind: 'ip'; query: string }
  | null;

function parseAsnOrIpQuery(raw: string): QueryState {
  const trimmed = raw.trim();
  if (!trimmed) return null;
  const asnMatch = /^(?:AS)?([1-9][0-9]{0,9})$/i.exec(trimmed);
  if (asnMatch) {
    const num = Number(asnMatch[1]);
    if (Number.isInteger(num) && num > 0 && num <= 4294967295) {
      return { kind: 'asn', asn: num };
    }
  }
  return { kind: 'ip', query: trimmed };
}

export function NetworkToolsAsnPageClient() {
  const t = useTranslations('networkTools');
  const queryClient = useQueryClient();

  const [input, setInput] = useState('');
  const [submitted, setSubmitted] = useState<QueryState>(null);
  const [family, setFamily] = useState<'all' | 'ipv4' | 'ipv6'>('all');
  const [prefixPage, setPrefixPage] = useState(1);

  const ipQuery = useQuery<NetToolsIPLookupResponse>({
    queryKey: [
      'openflare',
      'network-tools',
      'lookup-ip',
      submitted?.kind === 'ip' ? submitted.query : '',
    ],
    queryFn: () =>
      NetworkToolsService.lookupIP(
        submitted?.kind === 'ip' ? submitted.query : '',
      ),
    enabled: submitted?.kind === 'ip',
  });

  const asnDirectQuery = useQuery<NetToolsASNDetailResponse>({
    queryKey: [
      'openflare',
      'network-tools',
      'lookup-asn',
      submitted?.kind === 'asn' ? submitted.asn : 0,
    ],
    queryFn: () =>
      NetworkToolsService.lookupASN(
        submitted?.kind === 'asn' ? submitted.asn : 0,
      ),
    enabled: submitted?.kind === 'asn',
  });

  const resolvedAsn = useMemo(() => {
    if (submitted?.kind === 'asn') return submitted.asn;
    return ipQuery.data?.primary?.asn ?? ipQuery.data?.asn_info?.asn ?? 0;
  }, [submitted, ipQuery.data]);

  const prefixesQuery = useQuery({
    queryKey: [
      'openflare',
      'network-tools',
      'asn-prefixes',
      resolvedAsn,
      family,
      prefixPage,
    ],
    queryFn: () =>
      NetworkToolsService.listASNPrefixes(resolvedAsn, {
        family,
        p: prefixPage,
        page_size: 15,
      }),
    enabled: resolvedAsn > 0,
  });

  const pathMutation = useMutation({
    mutationFn: (targetAsn: number) =>
      NetworkToolsService.traceASPath({ target_asn: targetAsn }),
    onError: (err) =>
      toast.error(getNetToolsErrorMessage(err, t('common.error'))),
  });

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const parsed = parseAsnOrIpQuery(input);
    if (!parsed) return;
    setPrefixPage(1);
    setSubmitted(parsed);
    pathMutation.reset();
  };

  const asnInfo =
    submitted?.kind === 'asn'
      ? asnDirectQuery.data?.asn_info
      : ipQuery.data?.asn_info;
  const neighbors =
    (submitted?.kind === 'asn'
      ? asnDirectQuery.data?.neighbors
      : ipQuery.data?.neighbors) ?? [];
  const primaryIp =
    submitted?.kind === 'ip' ? ipQuery.data?.primary : undefined;
  const upstreamQueryKey = useMemo(
    () =>
      [
        'openflare',
        'network-tools',
        'asn-upstream-graph',
        resolvedAsn,
        primaryIp?.prefix ?? '',
      ] as const,
    [resolvedAsn, primaryIp?.prefix],
  );

  const upstreamGraphQuery = useQuery({
    queryKey: upstreamQueryKey,
    queryFn: () =>
      NetworkToolsService.getASNUpstreamGraph(resolvedAsn, {
        prefix: primaryIp?.prefix || undefined,
      }),
    enabled: resolvedAsn > 0,
  });

  const refreshUpstreamMutation = useMutation({
    mutationFn: () =>
      NetworkToolsService.getASNUpstreamGraph(resolvedAsn, {
        prefix: primaryIp?.prefix || undefined,
        refresh: true,
      }),
    onSuccess: (fresh) => {
      queryClient.setQueryData(upstreamQueryKey, fresh);
    },
    onError: (err) =>
      toast.error(getNetToolsErrorMessage(err, t('common.error'))),
  });

  const handleSelectUpstreamAsn = (asn: number) => {
    setInput(`AS${asn}`);
    setPrefixPage(1);
    setSubmitted({ kind: 'asn', asn });
    pathMutation.reset();
  };

  const isSearching = ipQuery.isFetching || asnDirectQuery.isFetching;
  const searchError = ipQuery.error || asnDirectQuery.error;
  const totalPrefixPages = Math.max(
    1,
    Math.ceil((prefixesQuery.data?.total ?? 0) / 15),
  );

  return (
    <div className='w-full py-6 px-1 space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div className='flex items-center gap-2'>
          <Network className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('asn.title')}
          </h1>
        </div>
        <span className='text-xs text-muted-foreground'>
          {t('common.attributionPrefix')}
          <a
            href='https://bgp.tools'
            target='_blank'
            rel='noreferrer'
            className='underline underline-offset-4 hover:text-foreground'
          >
            bgp.tools
          </a>
        </span>
      </div>

      <p className='text-sm text-muted-foreground'>{t('asn.description')}</p>

      <Card>
        <CardContent className='pt-6'>
          <form
            onSubmit={handleSearch}
            className='flex flex-wrap items-end gap-3'
          >
            <div className='min-w-[260px] flex-1 space-y-2'>
              <Label htmlFor='asn-query'>{t('asn.queryLabel')}</Label>
              <Input
                id='asn-query'
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder={t('asn.queryPlaceholder')}
              />
            </div>
            <Button type='submit' disabled={isSearching || !input.trim()}>
              {isSearching ? (
                <Loader2 className='mr-2 size-4 animate-spin' />
              ) : (
                <Search className='mr-2 size-4' />
              )}
              {t('asn.search')}
            </Button>
          </form>
        </CardContent>
      </Card>

      {searchError ? (
        <div className='rounded-xl border border-dashed p-4 text-sm text-destructive'>
          {getNetToolsErrorMessage(searchError, t('common.error'))}
        </div>
      ) : null}

      {submitted && (primaryIp || asnInfo) ? (
        <div className='space-y-6'>
          <Card>
            <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-2 pb-3'>
              <CardTitle className='text-base font-medium'>
                {t('asn.summaryCard')}
              </CardTitle>
              {resolvedAsn > 0 ? (
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={pathMutation.isPending}
                  onClick={() => pathMutation.mutate(resolvedAsn)}
                >
                  {pathMutation.isPending ? (
                    <Loader2 className='mr-2 size-3.5 animate-spin' />
                  ) : (
                    <GitFork className='mr-2 size-3.5' />
                  )}
                  {t('asn.pathCompare')}
                </Button>
              ) : null}
            </CardHeader>
            <CardContent className='grid grid-cols-1 gap-4 text-sm md:grid-cols-2 lg:grid-cols-4'>
              {primaryIp ? (
                <>
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      {t('asn.geoLocation')}
                    </div>
                    <div className='font-medium'>
                      {[
                        primaryIp.country_name || primaryIp.country_iso,
                        primaryIp.city_name,
                      ]
                        .filter(Boolean)
                        .join(' · ') || '—'}
                    </div>
                  </div>
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      {t('asn.bgpPrefix')}
                    </div>
                    <div className='font-mono font-medium'>
                      {primaryIp.prefix || '—'}
                    </div>
                  </div>
                </>
              ) : null}
              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground'>
                  {t('asn.asnOrg')}
                </div>
                <div className='font-medium'>
                  {resolvedAsn ? `AS${resolvedAsn}` : '—'}{' '}
                  {asnInfo?.name || primaryIp?.asn_name || ''}
                </div>
              </div>
              <div className='space-y-1'>
                <div className='text-xs text-muted-foreground'>
                  {t('asn.asnClass')}
                </div>
                <div className='flex flex-wrap items-center gap-1'>
                  <span>{asnInfo?.class || primaryIp?.asn_class || '—'}</span>
                  {(asnInfo?.tags ?? primaryIp?.asn_tags ?? []).map((tag) => (
                    <Badge key={tag} variant='secondary' className='text-xs'>
                      {tag}
                    </Badge>
                  ))}
                </div>
              </div>
              {asnInfo ? (
                <>
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      {t('asn.ipv4Prefixes')} / {t('asn.ipv6Prefixes')}
                    </div>
                    <div className='font-mono font-medium'>
                      {asnInfo.ipv4_prefixes} / {asnInfo.ipv6_prefixes}
                    </div>
                  </div>
                  <div className='space-y-1'>
                    <div className='text-xs text-muted-foreground'>
                      {t('asn.neighborCount')}
                    </div>
                    <div className='font-mono font-medium'>
                      {asnInfo.neighbor_count}
                    </div>
                  </div>
                </>
              ) : null}
            </CardContent>
          </Card>

          {resolvedAsn > 0 ? (
            <UpstreamRoutingGraph
              data={upstreamGraphQuery.data}
              isLoading={upstreamGraphQuery.isLoading}
              isFetching={
                upstreamGraphQuery.isFetching ||
                refreshUpstreamMutation.isPending
              }
              onRefresh={() => refreshUpstreamMutation.mutate()}
              onSelectAsn={handleSelectUpstreamAsn}
            />
          ) : null}

          {pathMutation.data ? (
            <Card>
              <CardHeader className='pb-3'>
                <CardTitle className='text-base font-medium'>
                  {t('asn.pathCard')} (AS{pathMutation.data.target_asn})
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('asn.columns.node')}</TableHead>
                      <TableHead>{t('asn.columns.nodeAsn')}</TableHead>
                      <TableHead>{t('asn.columns.asHops')}</TableHead>
                      <TableHead>{t('asn.columns.asPath')}</TableHead>
                      <TableHead>{t('asn.columns.effectiveRtt')}</TableHead>
                      <TableHead>{t('asn.columns.qualityScore')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {pathMutation.data.nodes.map((row) => (
                      <TableRow key={row.node_id}>
                        <TableCell>
                          <div className='font-medium'>{row.node_name}</div>
                          <div className='text-xs text-muted-foreground'>
                            {row.node_geo || row.node_ip}
                          </div>
                        </TableCell>
                        <TableCell className='font-mono text-xs'>
                          {row.node_asn ? `AS${row.node_asn}` : '—'}
                        </TableCell>
                        <TableCell>{row.as_hops}</TableCell>
                        <TableCell className='text-xs'>
                          <div className='flex flex-wrap items-center gap-1'>
                            {row.path.map((hop, idx) => (
                              <Badge
                                key={`${row.node_id}-${hop.asn}-${idx}`}
                                variant='outline'
                                className='font-mono text-[11px]'
                              >
                                AS{hop.asn}
                                {hop.country ? ` (${hop.country})` : ''}
                              </Badge>
                            ))}
                            {row.path.length === 0 ? '—' : null}
                          </div>
                        </TableCell>
                        <TableCell>
                          {row.effective_rtt_ms > 0
                            ? t('common.msUnit', {
                                value: formatNetToolsMs(row.effective_rtt_ms),
                              })
                            : '—'}
                        </TableCell>
                        <TableCell className='font-semibold'>
                          {row.quality_score > 0
                            ? row.quality_score.toFixed(1)
                            : '—'}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          ) : null}

          <div className='grid grid-cols-1 gap-6 lg:grid-cols-2'>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between gap-2 pb-3'>
                <CardTitle className='text-base font-medium'>
                  {t('asn.prefixesCard')}
                </CardTitle>
                <Select
                  value={family}
                  onValueChange={(v) => {
                    setFamily(v as 'all' | 'ipv4' | 'ipv6');
                    setPrefixPage(1);
                  }}
                >
                  <SelectTrigger className='w-36'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='all'>{t('asn.familyAll')}</SelectItem>
                    <SelectItem value='ipv4'>IPv4</SelectItem>
                    <SelectItem value='ipv6'>IPv6</SelectItem>
                  </SelectContent>
                </Select>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('asn.columns.cidr')}</TableHead>
                      <TableHead>{t('asn.columns.family')}</TableHead>
                      <TableHead>{t('asn.columns.hits')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(prefixesQuery.data?.results ?? []).map((p) => (
                      <TableRow key={p.cidr}>
                        <TableCell className='font-mono text-xs'>
                          {p.cidr}
                        </TableCell>
                        <TableCell className='uppercase text-xs'>
                          {p.family}
                        </TableCell>
                        <TableCell className='font-mono text-xs'>
                          {p.hits}
                        </TableCell>
                      </TableRow>
                    ))}
                    {(prefixesQuery.data?.results?.length ?? 0) === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={3}
                          className='py-6 text-center text-muted-foreground'
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
                      page: prefixPage,
                      totalPages: totalPrefixPages,
                      total: prefixesQuery.data?.total ?? 0,
                    })}
                  </span>
                  <div className='flex gap-2'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      disabled={prefixPage <= 1}
                      onClick={() => setPrefixPage((p) => Math.max(1, p - 1))}
                    >
                      {t('common.prevPage')}
                    </Button>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      disabled={prefixPage >= totalPrefixPages}
                      onClick={() => setPrefixPage((p) => p + 1)}
                    >
                      {t('common.nextPage')}
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-3'>
                <CardTitle className='text-base font-medium'>
                  {t('asn.neighborsCard')}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('asn.columns.peerAsn')}</TableHead>
                      <TableHead>{t('asn.columns.peerName')}</TableHead>
                      <TableHead>{t('asn.columns.relation')}</TableHead>
                      <TableHead>{t('asn.columns.observedRtt')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {neighbors.map((nb) => (
                      <TableRow key={`${nb.peer_asn}-${nb.relation}`}>
                        <TableCell className='font-mono text-xs'>
                          <button
                            type='button'
                            onClick={() => handleSelectUpstreamAsn(nb.peer_asn)}
                            className='font-mono text-xs text-primary underline-offset-4 hover:underline'
                          >
                            {`AS${nb.peer_asn}`}
                          </button>
                        </TableCell>
                        <TableCell className='text-xs'>
                          <div className='font-medium'>
                            {nb.peer_name || '—'}
                          </div>
                          <div className='text-muted-foreground'>
                            {[nb.peer_country, nb.peer_class]
                              .filter(Boolean)
                              .join(' · ')}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant='outline'>
                            {nb.relation === 'provider' ||
                            nb.relation === 'customer' ||
                            nb.relation === 'peer' ||
                            nb.relation === 'observed'
                              ? t(`asn.relations.${nb.relation}`)
                              : nb.relation}
                          </Badge>
                        </TableCell>
                        <TableCell className='font-mono text-xs'>
                          {nb.observed_rtt_ms
                            ? `${formatNetToolsMs(nb.observed_rtt_ms)} ms (${nb.sample_count ?? 0})`
                            : '—'}
                        </TableCell>
                      </TableRow>
                    ))}
                    {neighbors.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={4}
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
          </div>
        </div>
      ) : !isSearching ? (
        <div className='rounded-xl border border-dashed py-12 text-center text-sm text-muted-foreground'>
          {t('asn.emptyState')}
        </div>
      ) : null}
    </div>
  );
}
