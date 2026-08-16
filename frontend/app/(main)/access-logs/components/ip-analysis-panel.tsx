'use client';

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts';
import { Loader2, ShieldPlus, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { RankChart } from '@/components/data/rank-chart';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  AccessLogService,
  type DistributionItem,
  type WAFIPGroup,
  WafService,
} from '@/lib/services/openflare';
import { formatBytes, formatCompactNumber } from '@/lib/utils/metrics';

import { buildIPGroupPayloadFromGroup } from '../../waf/components/helpers';
import {
  formatOverviewRangeHint,
  formatOverviewTrendLabel,
  OVERVIEW_RANGE_OPTIONS,
  type OverviewRangeHours,
} from './access-log-utils';

function useTrendChartConfig() {
  const t = useTranslations('accessLogs');
  return {
    requests: { label: t('ip.requests'), color: 'hsl(var(--primary))' },
  } satisfies ChartConfig;
}

function resolveBucketMinutes(hours: number) {
  if (hours <= 24) return 30;
  return 60;
}

function groupsContainingIp(groups: WAFIPGroup[], ip: string) {
  const target = ip.trim();
  if (!target) return [];
  return groups.filter((group) =>
    (group.ip_list ?? []).some((entry) => entry.trim() === target),
  );
}

function toRankItems(items: DistributionItem[] | undefined) {
  return (items ?? []).map((item) => ({
    label: item.key,
    value: item.value,
  }));
}

/** Clamp analysis/trend window to API limits (1–720 hours). */
function clampAnalysisHours(hours: number): number {
  if (!Number.isFinite(hours) || hours <= 0) return 24;
  return Math.min(720, Math.max(1, Math.round(hours)));
}

function isOverviewPreset(hours: number): hours is OverviewRangeHours {
  return hours === 24 || hours === 168 || hours === 360 || hours === 720;
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className='rounded-lg border border-dashed px-3 py-2.5'>
      <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
        {label}
      </p>
      <p className='mt-1 text-lg font-semibold tracking-tight'>{value}</p>
    </div>
  );
}

function MiniRankCard({
  title,
  items,
  color,
}: {
  title: string;
  items: { label: string; value: number }[];
  color: string;
}) {
  const t = useTranslations('accessLogs.analysis');
  return (
    <div className='rounded-lg border border-dashed p-3'>
      <p className='mb-2 text-sm font-medium'>{title}</p>
      <RankChart
        items={items}
        color={color}
        className='!h-[220px]'
        emptyMessage={t('emptySeries', { title })}
      />
    </div>
  );
}

function AddToIPGroupPanel({
  ip,
  open,
  onClose,
}: {
  ip: string;
  open: boolean;
  onClose: () => void;
}) {
  const t = useTranslations('accessLogs.analysis');
  const queryClient = useQueryClient();
  const [selectedGroupId, setSelectedGroupId] = useState<string>('');

  const groupsQuery = useQuery({
    queryKey: ['openflare', 'waf', 'ip-groups'],
    queryFn: () => WafService.listIPGroups(),
    enabled: open,
  });

  const groups = useMemo(() => groupsQuery.data ?? [], [groupsQuery.data]);
  const matchedGroups = useMemo(
    () => groupsContainingIp(groups, ip),
    [groups, ip],
  );
  const manualGroups = useMemo(
    () =>
      groups.filter(
        (group) => group.type === 'manual' && group.enabled !== false,
      ),
    [groups],
  );
  const addableGroups = useMemo(
    () =>
      manualGroups.filter(
        (group) =>
          !(group.ip_list ?? []).some((entry) => entry.trim() === ip.trim()),
      ),
    [manualGroups, ip],
  );

  const updateMutation = useMutation({
    mutationFn: async ({
      group,
      nextList,
    }: {
      group: WAFIPGroup;
      nextList: string[];
    }) =>
      WafService.updateIPGroup(
        group.id,
        buildIPGroupPayloadFromGroup(group, nextList),
      ),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['openflare', 'waf', 'ip-groups'],
      });
    },
  });

  const handleAdd = async () => {
    const groupId = Number.parseInt(selectedGroupId, 10);
    const group = addableGroups.find((item) => item.id === groupId);
    if (!group) {
      toast.error(t('selectGroup'));
      return;
    }
    try {
      await updateMutation.mutateAsync({
        group,
        nextList: [...(group.ip_list ?? []), ip.trim()],
      });
      toast.success(t('added', { ip, name: group.name }));
      setSelectedGroupId('');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('addFailed'));
    }
  };

  const handleRemove = async (group: WAFIPGroup) => {
    try {
      await updateMutation.mutateAsync({
        group,
        nextList: (group.ip_list ?? []).filter(
          (entry) => entry.trim() !== ip.trim(),
        ),
      });
      toast.success(t('removed', { name: group.name, ip }));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('removeFailed'));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setSelectedGroupId('');
          onClose();
        }
      }}
    >
      <DialogContent className='max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('joinTitle')}</DialogTitle>
          <DialogDescription>
            {t('targetIp')}
            <span className='font-mono text-foreground'>{ip}</span>
          </DialogDescription>
        </DialogHeader>

        {groupsQuery.isLoading ? (
          <LoadingStateWithBorder title={t('loadingGroups')} />
        ) : groupsQuery.isError ? (
          <ErrorInline
            message={
              groupsQuery.error instanceof Error
                ? groupsQuery.error.message
                : t('loadGroupsFailed')
            }
            onRetry={() => void groupsQuery.refetch()}
          />
        ) : (
          <div className='space-y-4'>
            {matchedGroups.length > 0 ? (
              <div className='space-y-2 rounded-lg border border-dashed p-3'>
                <p className='text-sm font-medium text-foreground'>
                  {t('alreadyIn')}
                </p>
                <p className='text-xs text-muted-foreground'>
                  {t('alreadyInHint')}
                </p>
                <div className='space-y-2'>
                  {matchedGroups.map((group) => (
                    <div
                      key={group.id}
                      className='flex items-center justify-between gap-2 rounded-md border px-3 py-2'
                    >
                      <div className='min-w-0'>
                        <p className='truncate text-sm font-medium'>
                          {group.name}
                        </p>
                        <p className='text-[11px] text-muted-foreground'>
                          {t('entryCount', {
                            type: group.type,
                            count: group.ip_list?.length ?? 0,
                          })}
                        </p>
                      </div>
                      {group.type === 'manual' ? (
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-8 shrink-0 text-destructive'
                          disabled={updateMutation.isPending}
                          onClick={() => void handleRemove(group)}
                        >
                          {updateMutation.isPending ? (
                            <Loader2 className='size-3.5 animate-spin' />
                          ) : (
                            <>
                              <Trash2 className='mr-1 size-3.5' />
                              {t('delete')}
                            </>
                          )}
                        </Button>
                      ) : (
                        <Badge variant='outline' className='text-[10px]'>
                          {t('cannotDelete')}
                        </Badge>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <p className='text-sm text-muted-foreground'>{t('notInAny')}</p>
            )}

            <div className='space-y-2'>
              <p className='text-sm font-medium'>{t('addToOther')}</p>
              {addableGroups.length === 0 ? (
                <p className='text-xs text-muted-foreground'>
                  {t('noWritable')}
                </p>
              ) : (
                <Select
                  value={selectedGroupId}
                  onValueChange={setSelectedGroupId}
                >
                  <SelectTrigger className='h-9 text-xs'>
                    <SelectValue placeholder={t('selectManual')} />
                  </SelectTrigger>
                  <SelectContent>
                    {addableGroups.map((group) => (
                      <SelectItem key={group.id} value={String(group.id)}>
                        {group.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant='outline' onClick={onClose}>
            {t('close')}
          </Button>
          <Button
            onClick={() => void handleAdd()}
            disabled={
              !selectedGroupId ||
              updateMutation.isPending ||
              addableGroups.length === 0
            }
          >
            {updateMutation.isPending ? (
              <>
                <Loader2 className='mr-1 size-3.5 animate-spin' />
                {t('processing')}
              </>
            ) : (
              t('joinSelected')
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function IpAnalysisPanel({
  ip,
  enabled,
  initialHours = 24,
}: {
  ip: string;
  enabled: boolean;
  /**
   * Exact analysis window in hours (1–720), aligned with list filter duration.
   * Remount with key={ip} when switching IPs so this re-initializes.
   */
  initialHours?: number;
}) {
  const t = useTranslations('accessLogs');
  const trendChartConfig = useTrendChartConfig();
  const [ipGroupOpen, setIpGroupOpen] = useState(false);
  const [rangeHours, setRangeHours] = useState(() =>
    clampAnalysisHours(initialHours),
  );

  const bucketMinutes = resolveBucketMinutes(rangeHours);
  const rangeHint = isOverviewPreset(rangeHours)
    ? formatOverviewRangeHint(rangeHours, (key, values) => t(key, values))
    : t('analysis.recentHours', { hours: rangeHours });

  const trendQuery = useQuery({
    queryKey: [
      'openflare',
      'access-logs',
      'ip-trend',
      ip,
      rangeHours,
      bucketMinutes,
    ],
    queryFn: () =>
      AccessLogService.getIPTrend({
        remote_addr: ip,
        hours: rangeHours,
        bucket_minutes: bucketMinutes,
      }),
    enabled: enabled && ip !== '',
  });

  const analysisQuery = useQuery({
    queryKey: ['openflare', 'access-logs', 'ip-analysis', ip, rangeHours],
    queryFn: () =>
      AccessLogService.getIPAnalysis({
        remote_addr: ip,
        hours: rangeHours,
      }),
    enabled: enabled && ip !== '',
  });

  const trendChartData = useMemo(() => {
    return (trendQuery.data?.points ?? []).map((point) => ({
      label: formatOverviewTrendLabel(point.bucket_started_at, rangeHours),
      requests: point.request_count,
    }));
  }, [rangeHours, trendQuery.data?.points]);

  const analysis = analysisQuery.data;
  const isLoadingIP = trendQuery.isLoading || analysisQuery.isLoading;
  const isFetchingIP = trendQuery.isFetching || analysisQuery.isFetching;

  if (!ip) {
    return <EmptyStateWithBorder description={t('analysis.invalidIp')} />;
  }

  return (
    <>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <Button
            size='sm'
            variant='outline'
            onClick={() => setIpGroupOpen(true)}
          >
            <ShieldPlus className='mr-1 size-3.5' />
            {t('analysis.addToGroup')}
          </Button>
          <ToggleGroup
            type='single'
            value={isOverviewPreset(rangeHours) ? String(rangeHours) : ''}
            onValueChange={(value) => {
              if (!value) return;
              setRangeHours(clampAnalysisHours(Number.parseInt(value, 10)));
            }}
            variant='outline'
            size='sm'
          >
            {OVERVIEW_RANGE_OPTIONS.map((option) => (
              <ToggleGroupItem
                key={option.value}
                value={String(option.value)}
                className='px-2.5 text-xs'
              >
                {t(option.labelKey)}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        </div>

        {isLoadingIP ? (
          <LoadingStateWithBorder title={t('analysis.loading')} />
        ) : (
          <div className='space-y-4'>
            {analysisQuery.isError ? (
              <ErrorInline
                message={
                  analysisQuery.error instanceof Error
                    ? analysisQuery.error.message
                    : t('analysis.loadFailed')
                }
                onRetry={() => void analysisQuery.refetch()}
              />
            ) : analysis ? (
              <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
                <MetricCard
                  label={t('analysis.totalRequests')}
                  value={formatCompactNumber(analysis.summary.total_requests)}
                />
                <MetricCard
                  label={t('analysis.errors')}
                  value={formatCompactNumber(analysis.summary.error_count)}
                />
                <MetricCard
                  label={t('analysis.bandwidth')}
                  value={formatBytes(analysis.summary.bandwidth_served)}
                />
                <MetricCard
                  label={t('analysis.received')}
                  value={formatBytes(analysis.summary.bytes_received)}
                />
                <MetricCard
                  label={t('analysis.uniqueHosts')}
                  value={formatCompactNumber(analysis.summary.unique_hosts)}
                />
                <MetricCard
                  label={t('analysis.uniquePaths')}
                  value={formatCompactNumber(analysis.summary.unique_paths)}
                />
              </div>
            ) : null}

            <div className='space-y-3 rounded-lg border border-dashed p-4'>
              <div className='flex items-center justify-between gap-2'>
                <div>
                  <p className='text-sm font-medium'>{t('analysis.trend')}</p>
                  <p className='text-xs text-muted-foreground'>
                    {t('analysis.trendMeta', {
                      ip,
                      range: rangeHint,
                      minutes: bucketMinutes,
                    })}
                  </p>
                </div>
                <Button
                  size='sm'
                  variant='ghost'
                  disabled={isFetchingIP}
                  onClick={() => {
                    void trendQuery.refetch();
                    void analysisQuery.refetch();
                  }}
                >
                  {t('analysis.refresh')}
                </Button>
              </div>

              {trendQuery.isError ? (
                <ErrorInline
                  message={
                    trendQuery.error instanceof Error
                      ? trendQuery.error.message
                      : t('analysis.trendFailed')
                  }
                  onRetry={() => void trendQuery.refetch()}
                />
              ) : trendChartData.every((point) => point.requests === 0) ? (
                <EmptyStateWithBorder
                  description={t('analysis.trendEmpty', { range: rangeHint })}
                />
              ) : (
                <ChartContainer
                  config={trendChartConfig}
                  className='h-56 w-full'
                >
                  <AreaChart data={trendChartData}>
                    <CartesianGrid vertical={false} />
                    <XAxis
                      dataKey='label'
                      tickLine={false}
                      axisLine={false}
                      fontSize={10}
                      minTickGap={24}
                    />
                    <YAxis
                      tickLine={false}
                      axisLine={false}
                      fontSize={10}
                      width={40}
                      tickFormatter={(value) =>
                        formatCompactNumber(Number(value))
                      }
                    />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <Area
                      type='monotone'
                      dataKey='requests'
                      stroke='var(--color-requests)'
                      fill='var(--color-requests)'
                      fillOpacity={0.2}
                    />
                  </AreaChart>
                </ChartContainer>
              )}
            </div>

            {analysis ? (
              <div className='grid gap-3 md:grid-cols-2'>
                <MiniRankCard
                  title='Top Paths'
                  color='#a78bfa'
                  items={toRankItems(analysis.top_paths)}
                />
                <MiniRankCard
                  title='Top Hosts'
                  color='#34d399'
                  items={toRankItems(analysis.top_hosts)}
                />
                <MiniRankCard
                  title='Status Codes'
                  color='#f59e0b'
                  items={toRankItems(analysis.status_codes)}
                />
                <MiniRankCard
                  title='Top User-Agents'
                  color='#818cf8'
                  items={toRankItems(analysis.top_user_agents)}
                />
                <MiniRankCard
                  title='Device Types'
                  color='#38bdf8'
                  items={toRankItems(analysis.device_types)}
                />
                <MiniRankCard
                  title='Top Browsers'
                  color='#22c55e'
                  items={toRankItems(analysis.top_browsers)}
                />
              </div>
            ) : null}
          </div>
        )}
      </div>

      <AddToIPGroupPanel
        ip={ip}
        open={ipGroupOpen}
        onClose={() => setIpGroupOpen(false)}
      />
    </>
  );
}
