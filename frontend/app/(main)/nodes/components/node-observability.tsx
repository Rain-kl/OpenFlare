'use client';

import { getFormattingLocale } from '@/i18n/format';
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { formatDateTime } from '@/lib/utils';
import type { NodeHealthEvent, NodeItem } from '@/lib/services/openflare';
import { NodeService } from '@/lib/services/openflare';

import { TrafficTrendChart } from '../../components/dashboard/traffic-trend-chart';
import { DiskIOTrendChart } from './disk-io-trend-chart';
import { DistributionList } from './distribution-list';
import { NetworkTrendChart } from './network-trend-chart';
import { NodeHardwareOverview } from './node-hardware-overview';
import { NodeLoadDashboard } from './node-load-dashboard';
import { NodePingPanel } from './node-ping-panel';
import {
  NodeResourceSnapshot,
  NodeSystemDetails,
} from './node-resource-details';
import { NodeStatusBadge } from './node-status-badge';
import {
  formatMetricCount,
  formatPercent,
  formatUptime,
  formatUsageRatio,
  getErrorMessage,
  getHealthEventLabel,
  getHealthEventTone,
  isMeaningfulTime,
} from './node-utils';

type HealthEventFilter = 'all' | 'active' | 'resolved';
type NodeObservabilityVariant = 'edge' | 'compact';

function SummaryStat({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint: string;
}) {
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='pb-2'>
        <CardDescription>{label}</CardDescription>
        <CardTitle className='text-base font-semibold'>{value}</CardTitle>
        <p className='text-sm text-muted-foreground'>{hint}</p>
      </CardHeader>
    </Card>
  );
}

function HealthEventTimeline({
  events,
  allEventsCount,
  healthEventFilter,
  onFilterChange,
  onCleanup,
  cleanupPending,
}: {
  events: NodeHealthEvent[];
  allEventsCount: number;
  healthEventFilter: HealthEventFilter;
  onFilterChange: (filter: HealthEventFilter) => void;
  onCleanup: () => void;
  cleanupPending: boolean;
}) {
  const t = useTranslations('nodes');
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='flex-row items-center justify-between space-y-0'>
        <div>
          <CardTitle className='text-base font-semibold'>
            {t('obs.eventsTitle')}
          </CardTitle>
          <CardDescription>{t('obs.eventsDesc')}</CardDescription>
        </div>
        <Button
          variant='outline'
          size='sm'
          className='h-7 text-xs text-destructive hover:text-destructive'
          disabled={cleanupPending || allEventsCount === 0}
          onClick={onCleanup}
        >
          <Trash2 className='size-3.5 mr-1' />
          {t('obs.cleanupLogs')}
        </Button>
      </CardHeader>
      <CardContent className='space-y-4'>
        {allEventsCount > 0 ? (
          <>
            <div className='flex flex-wrap gap-2'>
              {(
                [
                  ['all', t('obs.filterAll')],
                  ['active', t('obs.filterActive')],
                  ['resolved', t('obs.filterResolved')],
                ] as const
              ).map(([filter, label]) => (
                <Button
                  key={filter}
                  type='button'
                  variant={
                    healthEventFilter === filter ? 'secondary' : 'outline'
                  }
                  size='sm'
                  className='h-7 text-xs rounded-full'
                  onClick={() => onFilterChange(filter)}
                >
                  {label}
                </Button>
              ))}
            </div>

            {events.length === 0 ? (
              <p className='text-sm text-muted-foreground'>
                {t('obs.filterEmpty')}
              </p>
            ) : null}

            {events.slice(0, 8).map((event) => (
              <div
                key={`${event.event_type}-${event.last_triggered_at}-${event.status}`}
                className='rounded-lg border px-4 py-3'
              >
                <div className='flex flex-wrap items-center gap-2'>
                  <NodeStatusBadge
                    label={getHealthEventLabel(event)}
                    tone={getHealthEventTone(event)}
                  />
                  <NodeStatusBadge
                    label={
                      event.status === 'active'
                        ? t('obs.filterActive')
                        : t('obs.filterResolved')
                    }
                    tone={event.status === 'active' ? 'warning' : 'success'}
                  />
                </div>
                <p className='mt-2 text-sm text-muted-foreground'>
                  {event.message || t('obs.noMessage')}
                </p>
                <div className='mt-2 grid gap-1 text-xs text-muted-foreground md:grid-cols-3'>
                  <p>
                    {t('obs.firstTriggered')}
                    {isMeaningfulTime(event.first_triggered_at)
                      ? ` ${formatDateTime(event.first_triggered_at)}`
                      : ' —'}
                  </p>
                  <p>
                    {t('obs.lastTriggered')}
                    {isMeaningfulTime(event.last_triggered_at)
                      ? ` ${formatDateTime(event.last_triggered_at)}`
                      : ' —'}
                  </p>
                  <p>
                    {t('obs.resolvedAt')}
                    {isMeaningfulTime(event.resolved_at) && event.resolved_at
                      ? ` ${formatDateTime(event.resolved_at)}`
                      : ' —'}
                  </p>
                </div>
              </div>
            ))}
          </>
        ) : (
          <p className='text-sm text-muted-foreground'>{t('obs.noEvents')}</p>
        )}
      </CardContent>
    </Card>
  );
}

export function NodeObservability({
  nodeId,
  node,
  variant = 'edge',
  connectionHint,
}: {
  nodeId: number;
  node?: NodeItem;
  variant?: NodeObservabilityVariant;
  connectionHint?: string;
}) {
  const t = useTranslations('nodes');
  const tc = useTranslations('common');
  const resolvedConnectionHint = connectionHint ?? t('edge.connectionHint');
  const queryClient = useQueryClient();
  const [healthEventFilter, setHealthEventFilter] =
    useState<HealthEventFilter>('all');
  const [cleanupOpen, setCleanupOpen] = useState(false);

  const observabilityQuery = useQuery({
    queryKey: ['openflare', 'node-observability', nodeId],
    queryFn: () =>
      NodeService.getObservability(nodeId, { hours: 24, limit: 48 }),
    refetchInterval: 30000,
  });

  const cleanupMutation = useMutation({
    mutationFn: () => NodeService.cleanupHealthEvents(nodeId),
    onSuccess: async (result) => {
      toast.success(
        result.deleted_count > 0
          ? t('obs.cleaned', { count: result.deleted_count })
          : t('obs.nothingToClean'),
      );
      setCleanupOpen(false);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'node-observability', nodeId],
        }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'dashboard', 'overview'],
        }),
      ]);
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const observability = observabilityQuery.data ?? null;
  const profile = observability?.profile ?? null;
  const latestMetric =
    observability?.metric_snapshots && observability.metric_snapshots.length > 0
      ? observability.metric_snapshots[
          observability.metric_snapshots.length - 1
        ]
      : null;
  const activeHealthEvents = useMemo(
    () =>
      observability?.health_events.filter(
        (event) => event.status === 'active',
      ) ?? [],
    [observability?.health_events],
  );
  const resolvedHealthEvents = useMemo(
    () =>
      observability?.health_events.filter(
        (event) => event.status === 'resolved',
      ) ?? [],
    [observability?.health_events],
  );
  const filteredHealthEvents = useMemo(() => {
    switch (healthEventFilter) {
      case 'active':
        return activeHealthEvents;
      case 'resolved':
        return resolvedHealthEvents;
      default:
        return observability?.health_events ?? [];
    }
  }, [
    activeHealthEvents,
    healthEventFilter,
    observability?.health_events,
    resolvedHealthEvents,
  ]);

  const trafficSummary = observability?.analytics?.traffic ?? null;
  const healthSummary = observability?.analytics?.health ?? null;
  const distributions = observability?.analytics?.distributions;
  const statusCodeDistribution = useMemo(
    () =>
      (distributions?.status_codes ?? []).map((item) => ({
        label: item.key,
        value: item.value,
      })),
    [distributions?.status_codes],
  );
  const topDomains = useMemo(
    () =>
      (distributions?.top_domains ?? []).map((item) => ({
        label: item.key,
        value: item.value,
      })),
    [distributions?.top_domains],
  );
  const topSourceCountry = distributions?.source_countries?.[0] ?? null;
  const latestHealthEvent = activeHealthEvents[0] ?? null;
  const dominantStatusCode = statusCodeDistribution[0] ?? null;
  const dominantDomain = topDomains[0] ?? null;
  const storageUsageRatio = formatUsageRatio(
    latestMetric?.storage_used_bytes,
    latestMetric?.storage_total_bytes,
  );
  const trends = observability?.trends;
  const nodeName = node?.name ?? observability?.node_id ?? String(nodeId);

  if (observabilityQuery.isLoading) {
    return (
      <Card className='border-dashed shadow-none'>
        <CardContent className='flex items-center justify-center py-10 text-sm text-muted-foreground'>
          <Loader2 className='size-4 mr-2 animate-spin' />
          {t('obs.loading')}
        </CardContent>
      </Card>
    );
  }

  if (observabilityQuery.isError) {
    return (
      <Card className='border-dashed shadow-none'>
        <CardContent className='py-6'>
          <p className='text-sm text-destructive'>
            {getErrorMessage(observabilityQuery.error, t('requestFailed'))}
          </p>
        </CardContent>
      </Card>
    );
  }

  if (variant === 'compact') {
    return (
      <div className='flex flex-col gap-6'>
        <div className='grid gap-4 md:grid-cols-3'>
          <SummaryStat
            label={t('obs.diag')}
            value={
              activeHealthEvents.length
                ? t('obs.activeIssues', { count: activeHealthEvents.length })
                : t('obs.stable')
            }
            hint={
              latestHealthEvent
                ? getHealthEventLabel(latestHealthEvent)
                : t('obs.noActiveEvents')
            }
          />
          <SummaryStat
            label={t('obs.systemCore')}
            value={profile?.hostname || '—'}
            hint={
              profile
                ? `${profile.os_name || '—'} · ${profile.architecture || '—'}`
                : '—'
            }
          />
          <SummaryStat
            label={t('obs.uptime')}
            value={formatUptime(profile?.uptime_seconds, t)}
            hint={t('obs.uptimeFromProfile')}
          />
        </div>

        <div className='grid gap-6 xl:grid-cols-2'>
          <Card className='border-dashed shadow-none'>
            <CardHeader>
              <CardTitle className='text-base font-semibold'>
                {t('obs.profileTitle')}
              </CardTitle>
              <CardDescription>{t('obs.profileDesc')}</CardDescription>
            </CardHeader>
            <CardContent>
              {profile ? (
                <NodeSystemDetails profile={profile} nodeName={nodeName} />
              ) : (
                <p className='text-sm text-muted-foreground'>
                  {t('obs.noProfile')}
                </p>
              )}
            </CardContent>
          </Card>

          <Card className='border-dashed shadow-none'>
            <CardHeader>
              <CardTitle className='text-base font-semibold'>
                {t('obs.snapshotTitle')}
              </CardTitle>
              <CardDescription>{t('obs.snapshotDesc')}</CardDescription>
            </CardHeader>
            <CardContent>
              {latestMetric ? (
                <NodeResourceSnapshot
                  metric={latestMetric}
                  connectionHint={resolvedConnectionHint}
                />
              ) : (
                <p className='text-sm text-muted-foreground'>
                  {t('obs.noSnapshot')}
                </p>
              )}
            </CardContent>
          </Card>
        </div>

        <HealthEventTimeline
          events={filteredHealthEvents}
          allEventsCount={observability?.health_events.length ?? 0}
          healthEventFilter={healthEventFilter}
          onFilterChange={setHealthEventFilter}
          onCleanup={() => setCleanupOpen(true)}
          cleanupPending={cleanupMutation.isPending}
        />

        <AlertDialog open={cleanupOpen} onOpenChange={setCleanupOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t('obs.cleanupTitle')}</AlertDialogTitle>
              <AlertDialogDescription>
                {t('obs.cleanupDesc')}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={cleanupMutation.isPending}>
                {tc('cancel')}
              </AlertDialogCancel>
              <AlertDialogAction
                className='bg-destructive text-white hover:bg-destructive/90'
                disabled={cleanupMutation.isPending}
                onClick={() => cleanupMutation.mutate()}
              >
                {cleanupMutation.isPending
                  ? t('obs.cleaning')
                  : t('obs.confirmCleanup')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    );
  }

  return (
    <div className='flex flex-col gap-6'>
      <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
        <SummaryStat
          label={t('obs.diag')}
          value={
            activeHealthEvents.length
              ? t('obs.activeIssues', { count: activeHealthEvents.length })
              : t('obs.stable')
          }
          hint={
            latestHealthEvent
              ? getHealthEventLabel(latestHealthEvent)
              : t('obs.noActiveEvents')
          }
        />
        <SummaryStat
          label={t('obs.windowRequests')}
          value={formatMetricCount(trafficSummary?.request_count)}
          hint={
            trafficSummary
              ? t('obs.windowVisitorsHint', {
                  visitors: formatMetricCount(
                    trafficSummary.unique_visitor_count,
                  ),
                  rate: trafficSummary.error_rate_percent.toFixed(1),
                })
              : t('obs.noWindowSummary')
          }
        />
        <SummaryStat
          label={t('obs.capacityPressure')}
          value={
            healthSummary?.has_capacity_risk
              ? t('obs.needsAttention')
              : t('obs.normalRange')
          }
          hint={
            latestMetric
              ? t('obs.cpuStorage', {
                  cpu: formatPercent(latestMetric.cpu_usage_percent),
                  storage: formatPercent(storageUsageRatio),
                })
              : t('obs.noResourceSnapshot')
          }
        />
        <SummaryStat
          label={t('obs.sourceSignal')}
          value={topSourceCountry?.key ?? '—'}
          hint={
            topSourceCountry
              ? t('obs.requestTimes', {
                  count: topSourceCountry.value.toLocaleString(
                    getFormattingLocale(),
                  ),
                })
              : t('obs.noSourceDist')
          }
        />
      </div>

      <NodeHardwareOverview profile={profile} latestMetric={latestMetric} />

      <NodeLoadDashboard nodeId={nodeId} profile={profile} />

      <NodePingPanel nodeId={nodeId} />

      <div className='grid gap-6 xl:grid-cols-2'>
        <TrafficTrendChart
          points={trends?.traffic_24h ?? []}
          description={t('obs.trafficTrendDesc')}
          compactSummary
        />
        <NetworkTrendChart points={trends?.network_24h ?? []} />
      </div>

      <DiskIOTrendChart points={trends?.disk_io_24h ?? []} />

      <Card className='border-dashed shadow-none'>
        <CardHeader>
          <CardTitle className='text-base font-semibold'>
            {t('obs.structureTitle')}
          </CardTitle>
          <CardDescription>{t('obs.structureDesc')}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className='mb-6 grid gap-4 md:grid-cols-3'>
            <div className='rounded-lg border px-4 py-4'>
              <p className='text-xs text-muted-foreground uppercase tracking-wide'>
                {t('obs.mainStatus')}
              </p>
              <p className='mt-3 text-2xl font-semibold'>
                {dominantStatusCode?.label ?? '—'}
              </p>
              <p className='mt-2 text-sm text-muted-foreground'>
                {dominantStatusCode
                  ? t('obs.times', { count: dominantStatusCode.value })
                  : t('obs.noStatusDist')}
              </p>
            </div>
            <div className='rounded-lg border px-4 py-4'>
              <p className='text-xs text-muted-foreground uppercase tracking-wide'>
                {t('obs.topDomain')}
              </p>
              <p className='mt-3 truncate text-2xl font-semibold'>
                {dominantDomain?.label ?? '—'}
              </p>
              <p className='mt-2 text-sm text-muted-foreground'>
                {dominantDomain
                  ? t('obs.times', { count: dominantDomain.value })
                  : t('obs.noDomainDist')}
              </p>
            </div>
            <div className='rounded-lg border px-4 py-4'>
              <p className='text-xs text-muted-foreground uppercase tracking-wide'>
                {t('obs.resolvedEvents')}
              </p>
              <p className='mt-3 text-2xl font-semibold'>
                {resolvedHealthEvents.length}
              </p>
              <p className='mt-2 text-sm text-muted-foreground'>
                {t('obs.resolvedHint')}
              </p>
            </div>
          </div>

          <div className='grid gap-6 xl:grid-cols-2'>
            <div>
              <p className='mb-4 text-xs text-muted-foreground uppercase tracking-wide'>
                {t('obs.statusDist')}
              </p>
              <DistributionList
                items={statusCodeDistribution}
                emptyMessage={t('obs.noStatusDist')}
              />
            </div>
            <div>
              <p className='mb-4 text-xs text-muted-foreground uppercase tracking-wide'>
                {t('obs.topDomain')}
              </p>
              <DistributionList
                items={topDomains}
                emptyMessage={t('obs.noDomainDist')}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      <HealthEventTimeline
        events={filteredHealthEvents}
        allEventsCount={observability?.health_events.length ?? 0}
        healthEventFilter={healthEventFilter}
        onFilterChange={setHealthEventFilter}
        onCleanup={() => setCleanupOpen(true)}
        cleanupPending={cleanupMutation.isPending}
      />

      <AlertDialog open={cleanupOpen} onOpenChange={setCleanupOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('obs.cleanupTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('obs.cleanupDesc')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cleanupMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={cleanupMutation.isPending}
              onClick={() => cleanupMutation.mutate()}
            >
              {cleanupMutation.isPending
                ? t('obs.cleaning')
                : t('obs.confirmCleanup')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
