'use client';

import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
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
import { NetworkToolsService } from '@/lib/services/openflare';

import {
  formatNetToolsMs,
  formatNetToolsPercent,
  getNetToolsErrorMessage,
} from '../../shared/utils';
import { MeasurementSourceBadge } from './measurement-source-badge';

const PREDICTION_METHOD_KEYS = new Set([
  'exact_region',
  'country_rollup',
  'inferred_upstream',
  'similar_node',
  'geo_prior',
]);

export function QualityPredictorTab() {
  const t = useTranslations('networkTools');

  const [simIp, setSimIp] = useState('');
  const [simAsn, setSimAsn] = useState('');
  const [simCountry, setSimCountry] = useState('');
  const [simRegion, setSimRegion] = useState('');
  const [simFamily, setSimFamily] = useState('auto');

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

  return (
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
                  <TableHead>{t('quality.columns.predictionMethod')}</TableHead>
                  <TableHead>{t('quality.columns.effectiveRtt')}</TableHead>
                  <TableHead>{t('quality.columns.scoreConfidence')}</TableHead>
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
                        {node.geo_base_known || node.geo_base_rtt_ms > 0
                          ? ` · geo ${formatNetToolsMs(node.geo_base_rtt_ms)} ms`
                          : ''}
                      </div>
                    </TableCell>
                    <TableCell className='text-xs'>
                      <MeasurementSourceBadge
                        source={node.measurement_source}
                      />
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
  );
}
