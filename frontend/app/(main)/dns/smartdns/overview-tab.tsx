'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Activity,
  CheckCircle2,
  Gauge,
  Globe2,
  RefreshCw,
  Save,
  Server,
  ShieldCheck,
  XCircle,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';

import { useChartThemePalette } from '@/components/data/chart-theme';
import { TrendChart } from '@/components/data/trend-chart';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  SmartDNSService,
  smartdnsQueryKey,
  type SmartDNSDelegationVerifyResult,
  type SmartDNSOverviewWindow,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';
import { getErrorMessage } from '../../websites/components/website-utils';

const WINDOWS: SmartDNSOverviewWindow[] = ['1h', '6h', '24h', '7d'];

export function SmartDNSOverviewTab() {
  const t = useTranslations('smartdns');
  const queryClient = useQueryClient();
  const palette = useChartThemePalette();
  const [windowRange, setWindowRange] = useState<SmartDNSOverviewWindow>('24h');

  const [enabled, setEnabled] = useState(true);
  const [delegationDomain, setDelegationDomain] = useState('');
  const [nsHostsText, setNsHostsText] = useState('');
  const [defaultTTL, setDefaultTTL] = useState('60');
  const [snapshotInterval, setSnapshotInterval] = useState('10');
  const [updateRepo, setUpdateRepo] = useState('');
  const [verifyResult, setVerifyResult] =
    useState<SmartDNSDelegationVerifyResult | null>(null);

  const overviewQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'overview', windowRange],
    queryFn: () => SmartDNSService.getOverview(windowRange),
    refetchInterval: 15000,
  });

  const settingsQuery = useQuery({
    queryKey: [...smartdnsQueryKey, 'settings'],
    queryFn: () => SmartDNSService.getSettings(),
  });

  useEffect(() => {
    const s = settingsQuery.data;
    if (!s) return;
    setEnabled(s.enabled);
    setDelegationDomain(s.delegation_domain ?? '');
    setNsHostsText((s.ns_hosts ?? []).join('\n'));
    setDefaultTTL(String(s.default_ttl ?? 60));
    setSnapshotInterval(String(s.snapshot_interval_seconds ?? 10));
    setUpdateRepo(s.update_repo ?? '');
  }, [settingsQuery.data]);

  const saveMutation = useMutation({
    mutationFn: () => {
      const nsHosts = nsHostsText
        .split(/[\n,]+/)
        .map((s) => s.trim())
        .filter(Boolean);
      const ttl = Number(defaultTTL);
      const interval = Number(snapshotInterval);
      return SmartDNSService.saveSettings({
        enabled,
        delegation_domain: delegationDomain.trim(),
        ns_hosts: nsHosts,
        default_ttl: Number.isFinite(ttl) && ttl > 0 ? Math.trunc(ttl) : 60,
        snapshot_interval_seconds:
          Number.isFinite(interval) && interval > 0 ? Math.trunc(interval) : 10,
        update_repo: updateRepo.trim(),
      });
    },
    onSuccess: async () => {
      toast.success(t('settingsSaved'));
      await queryClient.invalidateQueries({ queryKey: smartdnsQueryKey });
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const verifyMutation = useMutation({
    mutationFn: () => SmartDNSService.verifyDelegation(),
    onSuccess: (res) => {
      setVerifyResult(res);
      if (res.verified) {
        toast.success(t('delegationVerifiedToast'));
      } else {
        toast.warning(t('delegationMismatchToast'));
      }
    },
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const overview = overviewQuery.data;
  const chartData = useMemo(() => {
    const series = overview?.series ?? [];
    const labels = series.map((pt) => formatDateTime(pt.timestamp));
    return {
      labels,
      qps: series.map((pt) => Number(pt.qps.toFixed(2))),
      p50: series.map((pt) => Number(pt.latency_p50_ms.toFixed(2))),
      p95: series.map((pt) => Number(pt.latency_p95_ms.toFixed(2))),
      p99: series.map((pt) => Number(pt.latency_p99_ms.toFixed(2))),
    };
  }, [overview?.series]);

  return (
    <div className='space-y-6'>
      {overviewQuery.isLoading ? (
        <LoadingStateWithBorder
          icon={Globe2}
          description={t('loadingOverview')}
        />
      ) : overviewQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(overviewQuery.error)}
          onRetry={() => void overviewQuery.refetch()}
        />
      ) : overview ? (
        <>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='flex flex-wrap items-center gap-2'>
              <Badge variant={overview.enabled ? 'default' : 'secondary'}>
                {overview.enabled ? t('serviceEnabled') : t('serviceDisabled')}
              </Badge>
              {overview.delegation_domain ? (
                <Badge variant='outline' className='font-mono'>
                  {overview.delegation_domain}
                </Badge>
              ) : null}
              <Badge variant='outline' className='font-mono'>
                {t('snapshotVersion', { version: overview.snapshot_version })}
              </Badge>
            </div>
            <div className='flex items-center gap-2'>
              <ToggleGroup
                type='single'
                variant='outline'
                size='sm'
                value={windowRange}
                onValueChange={(v) => {
                  if (v) setWindowRange(v as SmartDNSOverviewWindow);
                }}
              >
                {WINDOWS.map((w) => (
                  <ToggleGroupItem key={w} value={w}>
                    {w}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
              <Button
                variant='outline'
                size='sm'
                onClick={() => void overviewQuery.refetch()}
                disabled={overviewQuery.isFetching}
              >
                <RefreshCw
                  data-icon='inline-start'
                  className={overviewQuery.isFetching ? 'animate-spin' : ''}
                />
                {t('refresh')}
              </Button>
            </div>
          </div>

          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
            <Card>
              <CardHeader className='pb-2'>
                <CardDescription className='flex items-center gap-1.5'>
                  <Activity className='size-3.5 text-primary' />
                  {t('kpiQps')}
                </CardDescription>
                <CardTitle className='text-2xl tabular-nums'>
                  {overview.total_qps.toFixed(1)}
                </CardTitle>
              </CardHeader>
              <CardContent className='text-xs text-muted-foreground'>
                {t('kpiInstancesOnline', {
                  online: overview.online_instances,
                  total: overview.total_instances,
                })}
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardDescription className='flex items-center gap-1.5'>
                  <Gauge className='size-3.5 text-primary' />
                  {t('kpiLatency')}
                </CardDescription>
                <CardTitle className='text-2xl tabular-nums'>
                  {overview.avg_p50_ms.toFixed(2)} ms
                </CardTitle>
              </CardHeader>
              <CardContent className='text-xs text-muted-foreground tabular-nums'>
                P95 {overview.avg_p95_ms.toFixed(2)} ms · P99{' '}
                {overview.avg_p99_ms.toFixed(2)} ms
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardDescription className='flex items-center gap-1.5'>
                  <ShieldCheck className='size-3.5 text-primary' />
                  {t('kpiCacheHit')}
                </CardDescription>
                <CardTitle className='text-2xl tabular-nums'>
                  {(overview.avg_cache_hit_ratio * 100).toFixed(1)}%
                </CardTitle>
              </CardHeader>
              <CardContent className='text-xs text-muted-foreground'>
                {t('kpiCacheSub')}
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardDescription className='flex items-center gap-1.5'>
                  <Server className='size-3.5 text-primary' />
                  {t('kpiCoverage')}
                </CardDescription>
                <CardTitle className='text-2xl tabular-nums'>
                  {overview.managed_domains}
                </CardTitle>
              </CardHeader>
              <CardContent className='text-xs text-muted-foreground'>
                {t('kpiPoliciesCount', { count: overview.active_policies })}
              </CardContent>
            </Card>
          </div>

          <div className='grid gap-4 lg:grid-cols-2'>
            <Card>
              <CardHeader>
                <CardTitle className='text-base'>
                  {t('chartQpsTitle')}
                </CardTitle>
                <CardDescription>{t('chartQpsDesc')}</CardDescription>
              </CardHeader>
              <CardContent>
                {chartData.labels.length === 0 ? (
                  <p className='py-12 text-center text-sm text-muted-foreground'>
                    {t('chartNoSeries')}
                  </p>
                ) : (
                  <TrendChart
                    labels={chartData.labels}
                    summaryScope='average'
                    yAxisValueFormatter={(v) => `${v.toFixed(1)} qps`}
                    series={[
                      {
                        label: 'QPS',
                        color: palette.primary,
                        values: chartData.qps,
                        variant: 'area',
                      },
                    ]}
                  />
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className='text-base'>
                  {t('chartLatencyTitle')}
                </CardTitle>
                <CardDescription>{t('chartLatencyDesc')}</CardDescription>
              </CardHeader>
              <CardContent>
                {chartData.labels.length === 0 ? (
                  <p className='py-12 text-center text-sm text-muted-foreground'>
                    {t('chartNoSeries')}
                  </p>
                ) : (
                  <TrendChart
                    labels={chartData.labels}
                    summaryScope='average'
                    yAxisValueFormatter={(v) => `${v.toFixed(1)} ms`}
                    series={[
                      {
                        label: 'P50',
                        color: palette.series[1],
                        values: chartData.p50,
                        variant: 'line',
                      },
                      {
                        label: 'P95',
                        color: palette.series[2],
                        values: chartData.p95,
                        variant: 'line',
                      },
                      {
                        label: 'P99',
                        color: palette.destructive,
                        values: chartData.p99,
                        variant: 'line',
                      },
                    ]}
                  />
                )}
              </CardContent>
            </Card>
          </div>
        </>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle className='text-base'>{t('settingsTitle')}</CardTitle>
          <CardDescription>{t('settingsDesc')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex items-center justify-between rounded-lg border border-dashed px-3 py-2.5'>
            <div>
              <p className='text-sm font-medium'>{t('enableLabel')}</p>
              <p className='text-xs text-muted-foreground'>{t('enableDesc')}</p>
            </div>
            <Switch checked={enabled} onCheckedChange={setEnabled} />
          </div>

          <div className='grid gap-4 md:grid-cols-2'>
            <div className='space-y-1.5'>
              <Label htmlFor='sdns-delegation-domain'>
                {t('delegationDomain')}
              </Label>
              <Input
                id='sdns-delegation-domain'
                value={delegationDomain}
                placeholder='sdns.example.com'
                onChange={(e) => setDelegationDomain(e.target.value)}
              />
              <p className='text-xs text-muted-foreground'>
                {t('delegationDomainDesc')}
              </p>
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='sdns-ns-hosts'>{t('nsHosts')}</Label>
              <Textarea
                id='sdns-ns-hosts'
                rows={3}
                value={nsHostsText}
                placeholder={'ns1.sdns.example.com\nns2.sdns.example.com'}
                onChange={(e) => setNsHostsText(e.target.value)}
              />
              <p className='text-xs text-muted-foreground'>
                {t('nsHostsDesc')}
              </p>
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='sdns-default-ttl'>{t('defaultTTL')}</Label>
              <Input
                id='sdns-default-ttl'
                type='number'
                min={10}
                max={86400}
                value={defaultTTL}
                onChange={(e) => setDefaultTTL(e.target.value)}
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='sdns-snap-interval'>
                {t('snapshotInterval')}
              </Label>
              <Input
                id='sdns-snap-interval'
                type='number'
                min={3}
                max={300}
                value={snapshotInterval}
                onChange={(e) => setSnapshotInterval(e.target.value)}
              />
            </div>

            <div className='space-y-1.5 md:col-span-2'>
              <Label htmlFor='sdns-update-repo'>{t('updateRepo')}</Label>
              <Input
                id='sdns-update-repo'
                value={updateRepo}
                placeholder='Rain-kl/OpenFlare'
                onChange={(e) => setUpdateRepo(e.target.value)}
              />
            </div>
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <Button
              onClick={() => saveMutation.mutate()}
              disabled={saveMutation.isPending}
            >
              <Save data-icon='inline-start' />
              {t('saveSettings')}
            </Button>
            <Button
              variant='outline'
              onClick={() => verifyMutation.mutate()}
              disabled={verifyMutation.isPending || !delegationDomain.trim()}
            >
              <ShieldCheck data-icon='inline-start' />
              {t('verifyDelegation')}
            </Button>
          </div>

          {verifyResult ? (
            <Alert>
              {verifyResult.verified ? (
                <CheckCircle2 className='text-primary' />
              ) : (
                <XCircle className='text-destructive' />
              )}
              <AlertTitle>
                {verifyResult.verified
                  ? t('verifyPassTitle')
                  : t('verifyFailTitle')}
              </AlertTitle>
              <AlertDescription className='space-y-1 text-xs'>
                <p>
                  {t('verifyObserved', {
                    records: verifyResult.observed_ns_records.join(', ') || '-',
                  })}
                </p>
                {verifyResult.missing_ns_hosts.length > 0 ? (
                  <p className='text-destructive'>
                    {t('verifyMissing', {
                      hosts: verifyResult.missing_ns_hosts.join(', '),
                    })}
                  </p>
                ) : null}
                {verifyResult.error ? (
                  <p className='text-destructive'>{verifyResult.error}</p>
                ) : null}
              </AlertDescription>
            </Alert>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
