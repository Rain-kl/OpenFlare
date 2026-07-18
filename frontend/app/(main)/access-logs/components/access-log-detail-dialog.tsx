'use client';

import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts';
import { Loader2, ShieldPlus, Trash2 } from 'lucide-react';
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
  type AccessLogItem,
  type DistributionItem,
  type WAFIPGroup,
  WafService,
} from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';
import { formatBytes, formatCompactNumber } from '@/lib/utils/metrics';

import { buildIPGroupPayloadFromGroup } from '../../waf/components/helpers';
import {
  cacheOutcomeLabel,
  formatOverviewRangeHint,
  formatOverviewTrendLabel,
  OVERVIEW_RANGE_OPTIONS,
  resolveCacheOutcome,
  type OverviewRangeHours,
} from './access-log-utils';

const trendChartConfig = {
  requests: { label: '请求数', color: 'hsl(var(--primary))' },
} satisfies ChartConfig;

function resolveBucketMinutes(hours: OverviewRangeHours) {
  if (hours <= 24) return 30;
  if (hours <= 168) return 60;
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

function DetailField({
  label,
  value,
  mono,
  full,
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
  full?: boolean;
}) {
  return (
    <div className={full ? 'sm:col-span-2 space-y-1' : 'space-y-1'}>
      <p className='text-[11px] uppercase tracking-wider text-muted-foreground'>
        {label}
      </p>
      <div
        className={`text-sm break-all ${mono ? 'font-mono text-xs' : 'text-foreground'}`}
      >
        {value}
      </div>
    </div>
  );
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
  return (
    <div className='rounded-lg border border-dashed p-3'>
      <p className='mb-2 text-sm font-medium'>{title}</p>
      <RankChart
        items={items}
        color={color}
        className='!h-[220px]'
        emptyMessage={`暂无 ${title} 数据`}
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
      toast.error('请选择要加入的 IP 组');
      return;
    }
    try {
      await updateMutation.mutateAsync({
        group,
        nextList: [...(group.ip_list ?? []), ip.trim()],
      });
      toast.success(`已将 ${ip} 加入 IP 组「${group.name}」`);
      setSelectedGroupId('');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '加入 IP 组失败');
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
      toast.success(`已从 IP 组「${group.name}」移除 ${ip}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '移除失败');
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
          <DialogTitle>将 IP 加入 IP 组</DialogTitle>
          <DialogDescription>
            目标 IP：
            <span className='font-mono text-foreground'>{ip}</span>
          </DialogDescription>
        </DialogHeader>

        {groupsQuery.isLoading ? (
          <LoadingStateWithBorder title='加载 IP 组' />
        ) : groupsQuery.isError ? (
          <ErrorInline
            message={
              groupsQuery.error instanceof Error
                ? groupsQuery.error.message
                : '加载 IP 组失败'
            }
            onRetry={() => void groupsQuery.refetch()}
          />
        ) : (
          <div className='space-y-4'>
            {matchedGroups.length > 0 ? (
              <div className='space-y-2 rounded-lg border border-dashed p-3'>
                <p className='text-sm font-medium text-foreground'>
                  该 IP 已存在于以下 IP 组
                </p>
                <p className='text-xs text-muted-foreground'>
                  可选择删除，或继续添加到其他 IP 组。
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
                          {group.type} · {group.ip_list?.length ?? 0} 条
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
                              删除
                            </>
                          )}
                        </Button>
                      ) : (
                        <Badge variant='outline' className='text-[10px]'>
                          不可手动删除
                        </Badge>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <p className='text-sm text-muted-foreground'>
                该 IP 尚未加入任何 IP 组。
              </p>
            )}

            <div className='space-y-2'>
              <p className='text-sm font-medium'>添加到其他 IP 组</p>
              {addableGroups.length === 0 ? (
                <p className='text-xs text-muted-foreground'>
                  没有可写入的手动 IP 组（或已全部包含该 IP）。
                </p>
              ) : (
                <Select
                  value={selectedGroupId}
                  onValueChange={setSelectedGroupId}
                >
                  <SelectTrigger className='h-9 text-xs'>
                    <SelectValue placeholder='选择手动 IP 组' />
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
            关闭
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
                处理中...
              </>
            ) : (
              '加入所选 IP 组'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function AccessLogDetailDialog({
  open,
  item,
  onOpenChange,
}: {
  open: boolean;
  item: AccessLogItem | null;
  onOpenChange: (open: boolean) => void;
}) {
  const [ipGroupOpen, setIpGroupOpen] = useState(false);
  const [rangeHours, setRangeHours] = useState<OverviewRangeHours>(24);
  // Keep last selected item while the dialog closes to avoid empty-state flash.
  const [displayItem, setDisplayItem] = useState<AccessLogItem | null>(item);
  if (item && item !== displayItem) {
    setDisplayItem(item);
  }
  const activeItem = item ?? displayItem;
  const ip = activeItem?.remote_addr?.trim() ?? '';
  const bucketMinutes = resolveBucketMinutes(rangeHours);
  const rangeHint = formatOverviewRangeHint(rangeHours);

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
    enabled: open && ip !== '',
  });

  const analysisQuery = useQuery({
    queryKey: ['openflare', 'access-logs', 'ip-analysis', ip, rangeHours],
    queryFn: () =>
      AccessLogService.getIPAnalysis({
        remote_addr: ip,
        hours: rangeHours,
      }),
    enabled: open && ip !== '',
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

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='max-h-[90vh] sm:max-w-6xl md:max-w-6xl overflow-y-auto hide-scrollbar'>
          <DialogHeader>
            <DialogTitle>访问日志详情</DialogTitle>
            <DialogDescription>
              查看请求字段、User-Agent 与该 IP 的访问趋势及分析数据。
            </DialogDescription>
          </DialogHeader>

          {activeItem && (
            <div className='space-y-5'>
              <div className='grid gap-4 sm:grid-cols-2'>
                <DetailField
                  label='时间'
                  value={formatDateTime(activeItem.logged_at)}
                />
                <DetailField
                  label='节点'
                  value={activeItem.node_name || activeItem.node_id || '—'}
                />
                <DetailField
                  label='IP'
                  value={activeItem.remote_addr || '—'}
                  mono
                />
                <DetailField label='地区' value={activeItem.region || '—'} />
                <DetailField label='域名' value={activeItem.host || '—'} />
                <DetailField
                  label='状态码'
                  value={
                    <Badge variant='outline' className='text-[10px]'>
                      {activeItem.status_code}
                    </Badge>
                  }
                />
                <DetailField
                  label='缓存'
                  value={
                    <div className='flex flex-wrap items-center gap-2'>
                      <Badge variant='outline' className='text-[10px]'>
                        {cacheOutcomeLabel(
                          resolveCacheOutcome(activeItem.cache_status),
                        )}
                      </Badge>
                      <span className='font-mono text-xs text-muted-foreground'>
                        {activeItem.cache_status || '—'}
                      </span>
                    </div>
                  }
                />
                <DetailField
                  label='路径'
                  value={activeItem.path || '—'}
                  full
                  mono
                />
                <DetailField
                  label='User-Agent'
                  value={activeItem.user_agent || '—'}
                  full
                  mono
                />
              </div>

              <div className='flex flex-wrap items-center justify-between gap-2'>
                <Button
                  size='sm'
                  variant='outline'
                  disabled={!ip}
                  onClick={() => setIpGroupOpen(true)}
                >
                  <ShieldPlus className='mr-1 size-3.5' />将 IP 加入到 IP 组
                </Button>
                <ToggleGroup
                  type='single'
                  value={String(rangeHours)}
                  onValueChange={(value) => {
                    if (!value) return;
                    setRangeHours(
                      Number.parseInt(value, 10) as OverviewRangeHours,
                    );
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
                      {option.label}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
              </div>

              {!ip ? (
                <EmptyStateWithBorder description='该日志没有有效 IP。' />
              ) : isLoadingIP ? (
                <LoadingStateWithBorder title='加载 IP 分析' />
              ) : (
                <div className='space-y-4'>
                  {analysisQuery.isError ? (
                    <ErrorInline
                      message={
                        analysisQuery.error instanceof Error
                          ? analysisQuery.error.message
                          : '加载 IP 分析失败'
                      }
                      onRetry={() => void analysisQuery.refetch()}
                    />
                  ) : analysis ? (
                    <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
                      <MetricCard
                        label='总请求'
                        value={formatCompactNumber(
                          analysis.summary.total_requests,
                        )}
                      />
                      <MetricCard
                        label='错误数'
                        value={formatCompactNumber(
                          analysis.summary.error_count,
                        )}
                      />
                      <MetricCard
                        label='已提供带宽'
                        value={formatBytes(analysis.summary.bandwidth_served)}
                      />
                      <MetricCard
                        label='接收数据'
                        value={formatBytes(analysis.summary.bytes_received)}
                      />
                      <MetricCard
                        label='独立域名'
                        value={formatCompactNumber(
                          analysis.summary.unique_hosts,
                        )}
                      />
                      <MetricCard
                        label='独立路径'
                        value={formatCompactNumber(
                          analysis.summary.unique_paths,
                        )}
                      />
                    </div>
                  ) : null}

                  <div className='space-y-3 rounded-lg border border-dashed p-4'>
                    <div className='flex items-center justify-between gap-2'>
                      <div>
                        <p className='text-sm font-medium'>IP 请求趋势</p>
                        <p className='text-xs text-muted-foreground'>
                          {ip} · {rangeHint} · {bucketMinutes} 分钟桶
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
                        刷新
                      </Button>
                    </div>

                    {trendQuery.isError ? (
                      <ErrorInline
                        message={
                          trendQuery.error instanceof Error
                            ? trendQuery.error.message
                            : '加载趋势失败'
                        }
                        onRetry={() => void trendQuery.refetch()}
                      />
                    ) : trendChartData.every(
                        (point) => point.requests === 0,
                      ) ? (
                      <EmptyStateWithBorder
                        description={`该 IP 在${rangeHint}内没有访问记录。`}
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
          )}
        </DialogContent>
      </Dialog>

      {ip ? (
        <AddToIPGroupPanel
          ip={ip}
          open={ipGroupOpen}
          onClose={() => setIpGroupOpen(false)}
        />
      ) : null}
    </>
  );
}
