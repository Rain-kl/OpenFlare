'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
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
} from '../../shared/utils';
import { MeasurementSourceBadge } from './measurement-source-badge';

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
                      {(row.geo_base_rtt_ms ?? 0) > 0 ||
                      (row.geo_confidence ?? 0) > 0
                        ? t('quality.columns.geoBase', {
                            value: formatNetToolsMs(row.geo_base_rtt_ms ?? 0),
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
                      <MeasurementSourceBadge source={row.measurement_source} />
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
