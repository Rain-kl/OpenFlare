'use client';

import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { EChartsOption } from 'echarts';
import ReactECharts from 'echarts-for-react';
import { ChevronDown, Filter, Loader2, X } from 'lucide-react';
import { Cell, Pie, PieChart } from 'recharts';

import { RankCard } from '@/components/data/rank-card';
import { TrendChart } from '@/components/data/trend-chart';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import { Input } from '@/components/ui/input';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  ZoneService,
  zoneQueryKey,
  type AccessLogOverview,
  type DistributionItem,
} from '@/lib/services/openflare';
import { cn } from '@/lib/utils';
import { formatBytes, formatCompactNumber } from '@/lib/utils/metrics';

import {
  formatOverviewRangeHint,
  formatOverviewTrendLabel,
  OVERVIEW_RANGE_OPTIONS,
  type OverviewRangeHours,
} from './access-log-utils';

const DEVICE_COLORS = [
  '#38bdf8',
  '#34d399',
  '#f59e0b',
  '#a78bfa',
  '#f472b6',
  '#94a3b8',
];

function SparklineMetricCard({
  title,
  value,
  hint,
  color,
  fillColor,
  labels,
  values,
  valueFormatter,
}: {
  title: string;
  value: string;
  hint: string;
  color: string;
  fillColor: string;
  labels: string[];
  values: number[];
  valueFormatter?: (value: number) => string;
}) {
  const option = useMemo<EChartsOption>(
    () => ({
      animationDuration: 400,
      grid: {
        left: 0,
        right: 0,
        top: 8,
        bottom: 0,
      },
      xAxis: {
        type: 'category',
        show: false,
        boundaryGap: false,
        data: labels,
      },
      yAxis: {
        type: 'value',
        show: false,
        min: 0,
      },
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(15, 23, 42, 0.92)',
        borderWidth: 0,
        textStyle: {
          color: '#e2e8f0',
          fontSize: 12,
        },
        formatter: (params: unknown) => {
          const items = Array.isArray(params) ? params : [];
          const item = items[0] as
            { axisValueLabel?: string; value?: number } | undefined;
          if (!item) return '';
          const raw = typeof item.value === 'number' ? item.value : 0;
          const formatted = valueFormatter
            ? valueFormatter(raw)
            : formatCompactNumber(raw);
          return `${item.axisValueLabel ?? ''}<br/>${formatted}`;
        },
      },
      series: [
        {
          type: 'line',
          data: values,
          smooth: true,
          showSymbol: false,
          lineStyle: {
            color,
            width: 2,
          },
          areaStyle: {
            color: fillColor,
          },
        },
      ],
    }),
    [color, fillColor, labels, valueFormatter, values],
  );

  return (
    <Card className='border-dashed shadow-none'>
      <CardContent className='p-4'>
        <div className='flex items-start justify-between gap-3'>
          <div className='min-w-0 space-y-1'>
            <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
              {title}
            </p>
            <p className='text-2xl font-semibold tracking-tight'>{value}</p>
            <p className='text-[11px] text-muted-foreground'>{hint}</p>
          </div>
          <div className='h-16 w-28 shrink-0 sm:w-36'>
            {labels.length > 0 ? (
              <ReactECharts
                option={option}
                notMerge
                lazyUpdate
                style={{ height: '100%', width: '100%' }}
              />
            ) : null}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function toRankItems(items: DistributionItem[] | undefined) {
  return (items ?? []).map((item) => ({
    label: item.key,
    value: item.value,
  }));
}

function PieDistributionCard({
  title,
  description,
  items,
  emptyMessage,
}: {
  title: string;
  description: string;
  items: DistributionItem[];
  emptyMessage: string;
}) {
  const chartData = useMemo(
    () =>
      items.map((item, index) => ({
        name: item.key,
        value: item.value,
        fill: DEVICE_COLORS[index % DEVICE_COLORS.length],
      })),
    [items],
  );

  const chartConfig = useMemo(() => {
    const config: ChartConfig = {};
    chartData.forEach((item) => {
      config[item.name] = {
        label: item.name,
        color: item.fill,
      };
    });
    return config;
  }, [chartData]);

  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='pb-3'>
        <CardTitle className='text-sm font-semibold text-foreground'>
          {title}
        </CardTitle>
        <CardDescription className='text-xs text-muted-foreground'>
          {description}
        </CardDescription>
      </CardHeader>
      <CardContent className='pt-0'>
        {chartData.length === 0 ? (
          <div className='flex h-[300px] items-center justify-center rounded-md border border-dashed bg-muted/20 text-sm text-muted-foreground'>
            {emptyMessage}
          </div>
        ) : (
          <ChartContainer
            config={chartConfig}
            className='mx-auto h-[300px] w-full'
          >
            <PieChart>
              <Pie
                data={chartData}
                dataKey='value'
                nameKey='name'
                cx='50%'
                cy='46%'
                innerRadius={50}
                outerRadius={80}
                paddingAngle={2}
              >
                {chartData.map((entry) => (
                  <Cell key={entry.name} fill={entry.fill} />
                ))}
              </Pie>
              <ChartTooltip
                cursor={false}
                content={
                  <ChartTooltipContent
                    hideLabel
                    formatter={(value, name) => (
                      <>
                        <span className='text-muted-foreground'>{name}</span>
                        <span className='ml-auto font-mono font-medium tabular-nums text-foreground'>
                          {formatCompactNumber(Number(value ?? 0))}
                        </span>
                      </>
                    )}
                  />
                }
              />
              <ChartLegend
                content={<ChartLegendContent nameKey='name' />}
                className='flex-wrap justify-center gap-x-4 gap-y-1 pt-2 text-[11px]'
              />
            </PieChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}

type ManagedZoneDomains = {
  zoneId: number;
  zoneName: string;
  domains: string[];
};

function useManagedZoneDomains(enabled: boolean) {
  return useQuery({
    queryKey: [...zoneQueryKey, 'zone-domain-tree'],
    enabled,
    staleTime: 60_000,
    queryFn: async (): Promise<ManagedZoneDomains[]> => {
      const zones = await ZoneService.list();
      const overviews = await Promise.all(
        zones.map((zone) => ZoneService.getOverview(zone.id)),
      );
      return overviews
        .map((overview) => {
          const domainSet = new Set<string>();
          for (const item of overview.domains ?? []) {
            const domain = item.domain?.trim();
            if (domain) domainSet.add(domain);
          }
          return {
            zoneId: overview.zone.id,
            zoneName: overview.zone.domain || `Zone #${overview.zone.id}`,
            domains: Array.from(domainSet).sort((a, b) => a.localeCompare(b)),
          };
        })
        .filter((zone) => zone.domains.length > 0)
        .sort((a, b) => a.zoneName.localeCompare(b.zoneName));
    },
  });
}

function OverviewHostFilter({
  hosts,
  onHostsChange,
}: {
  hosts: string[];
  onHostsChange: (hosts: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [expandedZones, setExpandedZones] = useState<Record<number, boolean>>(
    {},
  );
  const zonesQuery = useManagedZoneDomains(open);
  const zones = useMemo(() => zonesQuery.data ?? [], [zonesQuery.data]);
  const selectedSet = useMemo(() => new Set(hosts), [hosts]);
  const filteredZones = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return zones;
    return zones
      .map((zone) => {
        const zoneMatched = zone.zoneName.toLowerCase().includes(q);
        const domains = zoneMatched
          ? zone.domains
          : zone.domains.filter((domain) => domain.toLowerCase().includes(q));
        return { ...zone, domains };
      })
      .filter((zone) => zone.domains.length > 0);
  }, [query, zones]);

  useEffect(() => {
    if (!open || zones.length === 0) return;
    setExpandedZones((prev) => {
      const next = { ...prev };
      let changed = false;
      for (const zone of zones) {
        if (next[zone.zoneId] === undefined) {
          next[zone.zoneId] = true;
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [open, zones]);

  useEffect(() => {
    const q = query.trim();
    if (!q || filteredZones.length === 0) return;
    setExpandedZones((prev) => {
      const next = { ...prev };
      for (const zone of filteredZones) {
        next[zone.zoneId] = true;
      }
      return next;
    });
  }, [filteredZones, query]);

  const toggleHost = (domain: string, checked: boolean | 'indeterminate') => {
    if (checked === true) {
      if (selectedSet.has(domain)) return;
      onHostsChange([...hosts, domain]);
      return;
    }
    onHostsChange(hosts.filter((item) => item !== domain));
  };

  const toggleZone = (
    zoneDomains: string[],
    checked: boolean | 'indeterminate',
  ) => {
    if (checked === true) {
      const next = new Set(hosts);
      for (const domain of zoneDomains) next.add(domain);
      onHostsChange(Array.from(next));
      return;
    }
    onHostsChange(hosts.filter((item) => !zoneDomains.includes(item)));
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery('');
      }}
    >
      <PopoverTrigger asChild>
        <Button
          type='button'
          variant='outline'
          size='icon'
          className={cn(
            'size-8 shrink-0',
            hosts.length > 0 ? 'border-primary text-primary' : undefined,
          )}
          title={
            hosts.length > 0 ? `已筛选 ${hosts.length} 个域名` : '按域名筛选'
          }
          aria-label={
            hosts.length > 0 ? `已筛选 ${hosts.length} 个域名` : '按域名筛选'
          }
        >
          <Filter className='size-3.5' />
        </Button>
      </PopoverTrigger>
      <PopoverContent align='end' className='w-96 space-y-3 p-3'>
        <div className='flex items-center justify-between gap-2'>
          <p className='text-sm font-medium'>筛选域名</p>
          {hosts.length > 0 ? (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='h-7 px-2 text-xs'
              onClick={() => onHostsChange([])}
            >
              <X className='mr-1 size-3' />
              清除
            </Button>
          ) : null}
        </div>
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder='搜索 Zone 或域名'
          className='h-8 text-xs'
        />
        <div className='max-h-72 space-y-2 overflow-y-auto hide-scrollbar'>
          {zonesQuery.isLoading ? (
            <div className='flex items-center justify-center gap-2 py-6 text-xs text-muted-foreground'>
              <Loader2 className='size-3.5 animate-spin' />
              加载域名…
            </div>
          ) : zonesQuery.isError ? (
            <div className='space-y-2 py-2'>
              <p className='text-xs text-destructive'>加载域名失败</p>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='h-7 text-xs'
                onClick={() => void zonesQuery.refetch()}
              >
                重试
              </Button>
            </div>
          ) : filteredZones.length === 0 ? (
            <p className='py-6 text-center text-xs text-muted-foreground'>
              {zones.length === 0 ? '暂无已登记域名' : '没有匹配的域名'}
            </p>
          ) : (
            filteredZones.map((zone) => {
              const selectedCount = zone.domains.filter((domain) =>
                selectedSet.has(domain),
              ).length;
              const allSelected = selectedCount === zone.domains.length;
              const partialSelected =
                selectedCount > 0 && selectedCount < zone.domains.length;
              const expanded = expandedZones[zone.zoneId] ?? true;
              return (
                <Collapsible
                  key={zone.zoneId}
                  open={expanded}
                  onOpenChange={(next) =>
                    setExpandedZones((prev) => ({
                      ...prev,
                      [zone.zoneId]: next,
                    }))
                  }
                  className='rounded-md border border-dashed'
                >
                  <div className='flex items-center gap-1 px-2 py-1.5'>
                    <Checkbox
                      checked={
                        allSelected
                          ? true
                          : partialSelected
                            ? 'indeterminate'
                            : false
                      }
                      onCheckedChange={(checked) =>
                        toggleZone(zone.domains, checked)
                      }
                      aria-label={`选择 Zone ${zone.zoneName}`}
                    />
                    <CollapsibleTrigger asChild>
                      <button
                        type='button'
                        className='flex min-w-0 flex-1 items-center gap-1 rounded-md px-1 py-0.5 text-left text-xs font-medium hover:bg-accent'
                      >
                        <ChevronDown
                          className={cn(
                            'size-3.5 shrink-0 text-muted-foreground transition-transform',
                            expanded ? 'rotate-0' : '-rotate-90',
                          )}
                        />
                        <span className='min-w-0 flex-1 truncate'>
                          {zone.zoneName}
                        </span>
                        <span className='text-[10px] text-muted-foreground'>
                          {selectedCount}/{zone.domains.length}
                        </span>
                      </button>
                    </CollapsibleTrigger>
                  </div>
                  <CollapsibleContent>
                    <div className='space-y-0.5 border-t border-dashed px-2 py-1.5'>
                      {zone.domains.map((domain) => {
                        const selected = selectedSet.has(domain);
                        return (
                          <label
                            key={domain}
                            className={cn(
                              'flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-accent',
                              selected ? 'bg-accent/50' : undefined,
                            )}
                          >
                            <Checkbox
                              checked={selected}
                              onCheckedChange={(checked) =>
                                toggleHost(domain, checked)
                              }
                              aria-label={`选择域名 ${domain}`}
                            />
                            <span className='min-w-0 flex-1 truncate font-mono'>
                              {domain}
                            </span>
                          </label>
                        );
                      })}
                    </div>
                  </CollapsibleContent>
                </Collapsible>
              );
            })
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

function OverviewToolbar({
  hours,
  hosts,
  onHoursChange,
  onHostsChange,
}: {
  hours: OverviewRangeHours;
  hosts: string[];
  onHoursChange: (hours: OverviewRangeHours) => void;
  onHostsChange: (hosts: string[]) => void;
}) {
  return (
    <div className='flex flex-wrap items-center justify-end gap-2'>
      {hosts.length > 0 ? (
        <Badge variant='secondary' className='max-w-[260px] truncate'>
          {hosts.length === 1 ? hosts[0] : `已选 ${hosts.length} 个域名`}
        </Badge>
      ) : null}
      <OverviewHostFilter hosts={hosts} onHostsChange={onHostsChange} />
      <ToggleGroup
        type='single'
        value={String(hours)}
        onValueChange={(value) => {
          if (!value) return;
          onHoursChange(Number.parseInt(value, 10) as OverviewRangeHours);
        }}
        variant='outline'
        size='sm'
        className='justify-end'
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
  );
}

export function OverviewTab({
  data,
  loading,
  error,
  hours,
  hosts,
  onHoursChange,
  onHostsChange,
  onRetry,
}: {
  data?: AccessLogOverview;
  loading: boolean;
  error: Error | null;
  hours: OverviewRangeHours;
  hosts: string[];
  onHoursChange: (hours: OverviewRangeHours) => void;
  onHostsChange: (hosts: string[]) => void;
  onRetry: () => void;
}) {
  return (
    <div className='space-y-6'>
      <OverviewToolbar
        hours={hours}
        hosts={hosts}
        onHoursChange={onHoursChange}
        onHostsChange={onHostsChange}
      />

      {loading ? (
        <LoadingStateWithBorder
          title='加载访问概览'
          description='正在聚合请求量、访问量与带宽趋势...'
        />
      ) : error ? (
        <ErrorInline
          message={error.message || '加载访问概览失败'}
          onRetry={onRetry}
        />
      ) : !data ? (
        <EmptyStateWithBorder
          title='暂无概览数据'
          description='当前时间范围内没有可展示的访问统计。'
        />
      ) : (
        <OverviewContent data={data} hours={hours} />
      )}
    </div>
  );
}

function OverviewContent({
  data,
  hours,
}: {
  data: AccessLogOverview;
  hours: number;
}) {
  const requestLabels = data.trends.requests.map((point) =>
    formatOverviewTrendLabel(point.bucket_started_at, hours),
  );
  const requestValues = data.trends.requests.map((point) => point.value);
  const visitValues = data.trends.visits.map((point) => point.value);
  const bandwidthValues = data.trends.bandwidth.map((point) => point.value);
  const hint = formatOverviewRangeHint(hours);

  return (
    <>
      <div className='grid gap-4 lg:grid-cols-3'>
        <SparklineMetricCard
          title='Total Requests'
          value={formatCompactNumber(data.summary.total_requests)}
          hint={hint}
          color='#f59e0b'
          fillColor='rgba(245, 158, 11, 0.18)'
          labels={requestLabels}
          values={requestValues}
        />
        <SparklineMetricCard
          title='Total Visits'
          value={formatCompactNumber(data.summary.total_visits)}
          hint={`${hint} · 独立访客`}
          color='#38bdf8'
          fillColor='rgba(56, 189, 248, 0.16)'
          labels={requestLabels}
          values={visitValues}
        />
        <SparklineMetricCard
          title='Bandwidth Served'
          value={formatBytes(data.summary.bandwidth_served)}
          hint={`${hint} · 已提供数据`}
          color='#34d399'
          fillColor='rgba(52, 211, 153, 0.16)'
          labels={requestLabels}
          values={bandwidthValues}
          valueFormatter={formatBytes}
        />
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader>
          <CardTitle className='text-sm font-semibold'>
            Requests over time
          </CardTitle>
          <CardDescription className='text-xs'>
            观察请求量是否出现异常抬升或回落。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <TrendChart
            labels={requestLabels}
            height={280}
            showSummary={false}
            series={[
              {
                label: '请求量',
                color: '#f59e0b',
                fillColor: 'rgba(245, 158, 11, 0.18)',
                variant: 'area',
                values: requestValues,
              },
            ]}
          />
        </CardContent>
      </Card>

      <div className='grid gap-6 xl:grid-cols-2'>
        <PieDistributionCard
          title='Requests by device type'
          description='按设备类型统计请求占比。'
          items={data.device_types ?? []}
          emptyMessage='暂无设备类型数据'
        />
        <PieDistributionCard
          title='Status code'
          description='按 HTTP 状态码统计请求占比。'
          items={data.status_codes ?? []}
          emptyMessage='暂无状态码分布数据'
        />
      </div>

      <div className='grid gap-6 xl:grid-cols-3'>
        <RankCard
          title='Top Paths'
          description='访问量最高的请求路径。'
          items={toRankItems(data.top_paths)}
        />
        <RankCard
          title='Top Hosts'
          description='流量集中的访问域名。'
          items={toRankItems(data.top_hosts)}
        />
        <RankCard
          title='Top IPs'
          description='请求次数最多的来源 IP。'
          items={toRankItems(data.top_ips)}
        />
      </div>

      <div className='grid gap-6 xl:grid-cols-3'>
        <RankCard
          title='Top browsers'
          description='按浏览器聚合的请求排行。'
          items={toRankItems(data.top_browsers)}
        />
        <RankCard
          title='Top Operating System'
          description='按操作系统聚合的请求排行。'
          items={toRankItems(data.top_operating_systems)}
        />
        <RankCard
          title='Top User-Agent'
          description='原始 User-Agent 请求排行。'
          items={toRankItems(data.top_user_agents)}
        />
      </div>
    </>
  );
}
