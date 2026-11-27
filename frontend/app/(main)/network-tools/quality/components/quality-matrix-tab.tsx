'use client';

import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Calculator, Loader2, ShieldAlert } from 'lucide-react';
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
import type { NetToolsQualityMatrixItem } from '@/lib/services/openflare';
import { NetworkToolsService } from '@/lib/services/openflare';

import {
  formatNetToolsMs,
  formatNetToolsPercent,
  formatNetToolsTime,
  getNetToolsErrorMessage,
} from '../../shared/utils';

const PREDICTION_METHOD_KEYS = new Set([
  'exact_region',
  'country_rollup',
  'inferred_upstream',
  'similar_node',
  'geo_prior',
]);

const MEASUREMENT_SOURCE_KEYS = new Set([
  'tcp_passive',
  'tcp_active',
  'icmp_active',
  'traceroute',
  'none',
]);

interface QualityMatrixTabProps {
  refreshNonce: number;
}

export function QualityMatrixTab({ refreshNonce }: QualityMatrixTabProps) {
  const t = useTranslations('networkTools');

  const [search, setSearch] = useState('');
  const [clientCountry, setClientCountry] = useState('');
  const [clientRegion, setClientRegion] = useState('');
  const [ipFamily, setIpFamily] = useState('all');
  const [aggLevel, setAggLevel] = useState('all');
  const [dataSource, setDataSource] = useState('all');
  const [measureSource, setMeasureSource] = useState('all');
  const [page, setPage] = useState(1);

  // On-demand 5-level predictor simulator state
  const [simIp, setSimIp] = useState('');
  const [simAsn, setSimAsn] = useState('');
  const [simCountry, setSimCountry] = useState('');
  const [simRegion, setSimRegion] = useState('');
  const [simFamily, setSimFamily] = useState('auto');

  const matrixQuery = useQuery({
    queryKey: [
      'openflare',
      'network-tools',
      'quality-matrix',
      refreshNonce,
      page,
      search,
      clientCountry,
      clientRegion,
      ipFamily,
      aggLevel,
      dataSource,
      measureSource,
    ],
    queryFn: () =>
      NetworkToolsService.listQualityMatrix({
        p: page,
        page_size: 20,
        search: search.trim() || undefined,
        client_country: clientCountry.trim().toUpperCase() || undefined,
        client_region: clientRegion.trim().toUpperCase() || undefined,
        ip_family: ipFamily === 'all' ? undefined : ipFamily,
        aggregation_level: aggLevel === 'all' ? undefined : aggLevel,
        data_source: dataSource === 'all' ? undefined : dataSource,
        measurement_source: measureSource === 'all' ? undefined : measureSource,
      }),
  });

  const predictMutation = useMutation({
    mutationFn: () => {
      const parsedAsn = Number(simAsn.trim());
      return NetworkToolsService.predictQualityMatrix({
        target_ip: simIp.trim() || undefined,
        target_asn:
          Number.isInteger(parsedAsn) && parsedAsn > 0 ? parsedAsn : undefined,
        target_country: simCountry.trim().toUpperCase() || undefined,
        target_region: simRegion.trim().toUpperCase() || undefined,
        ip_family: simFamily === 'auto' ? undefined : simFamily,
      });
    },
    onError: (err) =>
      toast.error(getNetToolsErrorMessage(err, t('common.error'))),
  });

  const renderMeasurementBadge = (source?: string) => {
    const clean = source || 'none';
    const label = MEASUREMENT_SOURCE_KEYS.has(clean)
      ? t(`quality.sources.${clean}` as Parameters<typeof t>[0])
      : clean;
    return (
      <Badge variant={clean === 'none' ? 'outline' : 'secondary'}>
        {label}
      </Badge>
    );
  };

  const renderPredictionMethod = (method?: string) => {
    if (!method) return '—';
    const label = PREDICTION_METHOD_KEYS.has(method)
      ? t(`quality.predictor.methods.${method}` as Parameters<typeof t>[0])
      : method;
    return (
      <Badge
        variant={
          method === 'exact_region' || method === 'country_rollup'
            ? 'default'
            : 'outline'
        }
      >
        {label}
      </Badge>
    );
  };

  const formatActiveStats = (row: NetToolsQualityMatrixItem) => {
    const parts: string[] = [];
    if ((row.probe_sample_count ?? 0) > 0 && (row.probe_rtt_p50_ms ?? 0) > 0) {
      parts.push(
        `${(row.probe_proto || 'probe').toUpperCase()} ${formatNetToolsMs(row.probe_rtt_p50_ms)} ms · ${formatNetToolsPercent((row.probe_loss_rate ?? 0) * 100)}%`,
      );
    }
    if (row.trace_sample_count > 0 || row.trace_hop_count > 0) {
      parts.push(
        `TR ${formatNetToolsMs(row.trace_rtt_p50_ms)} ms · ${row.trace_hop_count}h`,
      );
    }
    return parts.length > 0 ? parts.join(' | ') : '—';
  };

  const totalPages = Math.max(
    1,
    Math.ceil((matrixQuery.data?.total ?? 0) / 20),
  );

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader className='pb-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div className='space-y-1'>
              <CardTitle className='flex items-center gap-2 text-base font-medium'>
                <Calculator className='size-4 text-primary' />
                {t('quality.predictor.title')}
              </CardTitle>
              <p className='text-xs text-muted-foreground'>
                {t('quality.predictor.description')}
              </p>
            </div>
          </div>
        </CardHeader>
        <CardContent className='space-y-4'>
          <form
            className='grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-6'
            onSubmit={(e) => {
              e.preventDefault();
              predictMutation.mutate();
            }}
          >
            <Input
              value={simIp}
              onChange={(e) => setSimIp(e.target.value)}
              placeholder={t('quality.predictor.targetIpPlaceholder')}
            />
            <Input
              value={simAsn}
              onChange={(e) => setSimAsn(e.target.value)}
              placeholder={t('quality.predictor.targetAsnPlaceholder')}
            />
            <Input
              value={simCountry}
              onChange={(e) => setSimCountry(e.target.value)}
              placeholder={t('quality.predictor.targetCountryPlaceholder')}
            />
            <Input
              value={simRegion}
              onChange={(e) => setSimRegion(e.target.value)}
              placeholder={t('quality.predictor.targetRegionPlaceholder')}
            />
            <Select value={simFamily} onValueChange={setSimFamily}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='auto'>
                  {t('quality.filters.ipFamilyAll')}
                </SelectItem>
                <SelectItem value='ipv4'>IPv4</SelectItem>
                <SelectItem value='ipv6'>IPv6</SelectItem>
              </SelectContent>
            </Select>
            <Button
              type='submit'
              disabled={predictMutation.isPending}
              className='w-full'
            >
              {predictMutation.isPending ? (
                <Loader2 className='mr-1.5 size-3.5 animate-spin' />
              ) : (
                <Calculator className='mr-1.5 size-3.5' />
              )}
              {t('quality.predictor.run')}
            </Button>
          </form>

          {predictMutation.data ? (
            <div className='space-y-2 rounded-md border border-dashed p-3'>
              <div className='flex flex-wrap items-center gap-2 text-xs text-muted-foreground'>
                {predictMutation.data.target_asn > 0 ? (
                  <Badge variant='outline' className='font-mono'>
                    AS{predictMutation.data.target_asn}
                    {predictMutation.data.target_asn_info?.name
                      ? ` · ${predictMutation.data.target_asn_info.name}`
                      : ''}
                  </Badge>
                ) : null}
                {predictMutation.data.target_country ? (
                  <Badge variant='outline'>
                    {predictMutation.data.target_country}
                    {predictMutation.data.target_region
                      ? ` / ${predictMutation.data.target_region}`
                      : ''}
                  </Badge>
                ) : null}
                <Badge variant='outline' className='uppercase'>
                  {predictMutation.data.ip_family}
                </Badge>
                {predictMutation.data.anycast_status === 'confirmed' ? (
                  <Badge variant='destructive' className='gap-1'>
                    <ShieldAlert className='size-3' />
                    {t('quality.predictor.anycastIsolatedBadge')}
                  </Badge>
                ) : null}
              </div>

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('quality.columns.node')}</TableHead>
                    <TableHead>
                      {t('quality.columns.predictionMethod')}
                    </TableHead>
                    <TableHead>{t('quality.columns.effectiveRtt')}</TableHead>
                    <TableHead>
                      {t('quality.columns.scoreConfidence')}
                    </TableHead>
                    <TableHead>{t('quality.columns.source')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {predictMutation.data.nodes.map((node) => (
                    <TableRow key={node.node_id}>
                      <TableCell className='text-xs'>
                        <div className='flex flex-wrap items-center gap-1.5 font-medium'>
                          <span>{node.node_name || node.node_id}</span>
                          {node.network_group ? (
                            <Badge variant='secondary' className='font-mono'>
                              {node.network_group}
                            </Badge>
                          ) : null}
                        </div>
                        <div className='text-muted-foreground'>
                          {node.node_geo || node.node_ip}
                          {node.geo_source ? ` · ${node.geo_source}` : ''}
                        </div>
                      </TableCell>
                      <TableCell className='text-xs'>
                        <div className='flex flex-wrap items-center gap-1'>
                          {renderPredictionMethod(node.prediction_method)}
                          {node.anycast_isolated ? (
                            <Badge variant='outline' className='text-[11px]'>
                              {t('quality.predictor.anycastIsolatedBadge')}
                            </Badge>
                          ) : null}
                        </div>
                      </TableCell>
                      <TableCell className='font-mono text-xs'>
                        <div className='font-semibold'>
                          {formatNetToolsMs(node.effective_rtt_ms)} /{' '}
                          {formatNetToolsMs(node.rtt_p95_ms)} ms
                        </div>
                        <div className='text-muted-foreground'>
                          {t('quality.predictor.dispersionError', {
                            dispersion: formatNetToolsMs(
                              node.candidate_dispersion_ms,
                            ),
                            error:
                              node.prediction_error_ms !== undefined &&
                              node.prediction_error_ms !== null
                                ? `${formatNetToolsMs(node.prediction_error_ms)} ms`
                                : '—',
                          })}
                        </div>
                      </TableCell>
                      <TableCell className='font-mono text-xs'>
                        <div className='font-semibold'>
                          {node.quality_score.toFixed(1)}
                        </div>
                        <div className='text-muted-foreground'>
                          {t('common.pctUnit', {
                            value: formatNetToolsPercent(node.confidence * 100),
                          })}
                          {node.geo_base_rtt_ms > 0
                            ? ` · geo ${formatNetToolsMs(node.geo_base_rtt_ms)} ms`
                            : ''}
                        </div>
                      </TableCell>
                      <TableCell className='text-xs'>
                        {renderMeasurementBadge(node.measurement_source)}
                      </TableCell>
                    </TableRow>
                  ))}
                  {predictMutation.data.nodes.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={5}
                        className='py-6 text-center text-xs text-muted-foreground'
                      >
                        {t('common.noData')}
                      </TableCell>
                    </TableRow>
                  ) : null}
                </TableBody>
              </Table>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <div className='flex flex-wrap items-center gap-2'>
        <Input
          className='w-60'
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(1);
          }}
          placeholder={t('quality.filters.searchPlaceholder')}
        />
        <Input
          className='w-28 uppercase'
          value={clientCountry}
          onChange={(e) => {
            setClientCountry(e.target.value);
            setPage(1);
          }}
          placeholder={t('quality.filters.countryPlaceholder')}
        />
        <Input
          className='w-32 uppercase'
          value={clientRegion}
          onChange={(e) => {
            setClientRegion(e.target.value);
            setPage(1);
          }}
          placeholder={t('quality.filters.regionPlaceholder')}
        />
        <Select
          value={ipFamily}
          onValueChange={(v) => {
            setIpFamily(v);
            setPage(1);
          }}
        >
          <SelectTrigger className='w-32'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>
              {t('quality.filters.ipFamilyAll')}
            </SelectItem>
            <SelectItem value='ipv4'>IPv4</SelectItem>
            <SelectItem value='ipv6'>IPv6</SelectItem>
            <SelectItem value='unknown'>Unknown</SelectItem>
          </SelectContent>
        </Select>
        <Select
          value={aggLevel}
          onValueChange={(v) => {
            setAggLevel(v);
            setPage(1);
          }}
        >
          <SelectTrigger className='w-36'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('quality.filters.levelAll')}</SelectItem>
            <SelectItem value='region'>
              {t('quality.filters.levelRegion')}
            </SelectItem>
            <SelectItem value='country'>
              {t('quality.filters.levelCountry')}
            </SelectItem>
          </SelectContent>
        </Select>
        <Select
          value={measureSource}
          onValueChange={(v) => {
            setMeasureSource(v);
            setPage(1);
          }}
        >
          <SelectTrigger className='w-44'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>
              {t('quality.filters.measureAll')}
            </SelectItem>
            <SelectItem value='tcp_passive'>
              {t('quality.filters.measureTcpPassive')}
            </SelectItem>
            <SelectItem value='tcp_active'>
              {t('quality.filters.measureTcpActive')}
            </SelectItem>
            <SelectItem value='icmp_active'>
              {t('quality.filters.measureIcmpActive')}
            </SelectItem>
            <SelectItem value='traceroute'>
              {t('quality.filters.measureTraceroute')}
            </SelectItem>
          </SelectContent>
        </Select>
        <Select
          value={dataSource}
          onValueChange={(v) => {
            setDataSource(v);
            setPage(1);
          }}
        >
          <SelectTrigger className='w-44'>
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
                <TableHead>{t('quality.columns.clientNetwork')}</TableHead>
                <TableHead>{t('quality.columns.effectiveRtt')}</TableHead>
                <TableHead>{t('quality.columns.tcpStats')}</TableHead>
                <TableHead>{t('quality.columns.activeStats')}</TableHead>
                <TableHead>{t('quality.columns.scoreConfidence')}</TableHead>
                <TableHead>{t('quality.columns.source')}</TableHead>
                <TableHead>{t('quality.columns.updatedAt')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(matrixQuery.data?.results ?? []).map((row) => (
                <TableRow key={row.id}>
                  <TableCell className='text-xs'>
                    <div className='flex flex-wrap items-center gap-1 font-medium'>
                      <span>{row.node_name || row.node_id}</span>
                      {row.node_network_group ? (
                        <Badge variant='secondary' className='font-mono'>
                          {row.node_network_group}
                        </Badge>
                      ) : null}
                    </div>
                    <div className='text-muted-foreground'>
                      {row.node_geo || '—'}
                      {row.node_asn ? ` · AS${row.node_asn}` : ''}
                      {row.node_geo_source ? ` · ${row.node_geo_source}` : ''}
                    </div>
                  </TableCell>
                  <TableCell className='text-xs'>
                    <div className='flex flex-wrap items-center gap-1 font-mono font-medium'>
                      <span>AS{row.client_asn}</span>
                      {row.client_country ? (
                        <span>· {row.client_country}</span>
                      ) : null}
                      {row.client_region ? (
                        <span>/ {row.client_region}</span>
                      ) : null}
                      {row.ip_family ? (
                        <Badge variant='outline' className='text-[10px]'>
                          {row.ip_family}
                        </Badge>
                      ) : null}
                      {row.aggregation_level ? (
                        <Badge variant='secondary' className='text-[10px]'>
                          {row.aggregation_level}
                        </Badge>
                      ) : null}
                    </div>
                    <div className='text-muted-foreground'>
                      {row.client_asn_name || '—'}
                    </div>
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    <div className='font-semibold'>
                      {t('common.msUnit', {
                        value: formatNetToolsMs(row.effective_rtt_ms),
                      })}
                    </div>
                    <div className='text-muted-foreground'>
                      {(row.geo_base_rtt_ms ?? 0) > 0
                        ? t('quality.columns.geoBase', {
                            value: formatNetToolsMs(row.geo_base_rtt_ms),
                          })
                        : '—'}
                    </div>
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    {row.tcp_sample_count > 0 ? (
                      <>
                        <div>
                          {formatNetToolsMs(row.tcp_rtt_p50_ms)} /{' '}
                          {formatNetToolsMs(row.tcp_rtt_p95_ms)} ms
                        </div>
                        <div className='text-muted-foreground'>
                          n={row.tcp_sample_count} ·{' '}
                          {formatNetToolsPercent(row.tcp_retrans_rate * 100)}%
                        </div>
                      </>
                    ) : (
                      '—'
                    )}
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    {formatActiveStats(row)}
                  </TableCell>
                  <TableCell className='font-mono text-xs'>
                    <div className='font-semibold'>
                      {row.quality_score.toFixed(1)} ·{' '}
                      {t('common.pctUnit', {
                        value: formatNetToolsPercent(row.confidence * 100),
                      })}
                    </div>
                    <div className='text-muted-foreground'>
                      {t('quality.columns.prefixes', {
                        count: row.distinct_prefix_count ?? 0,
                      })}
                    </div>
                  </TableCell>
                  <TableCell className='text-xs'>
                    <div className='flex flex-col gap-1'>
                      {renderMeasurementBadge(row.measurement_source)}
                      <span className='font-mono text-[11px] text-muted-foreground'>
                        {row.data_source}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className='text-xs text-muted-foreground'>
                    {formatNetToolsTime(row.last_observed_at || row.updated_at)}
                  </TableCell>
                </TableRow>
              ))}
              {!matrixQuery.isLoading &&
              (matrixQuery.data?.results?.length ?? 0) === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={8}
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
    </div>
  );
}
