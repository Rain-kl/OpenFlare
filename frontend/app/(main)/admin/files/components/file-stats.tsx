'use client';

import * as React from 'react';
import { useTranslations } from 'next-intl';
import { useQuery } from '@tanstack/react-query';
import {
  Database,
  Files,
  HardDrive,
  Info,
  Loader2,
  TrendingUp,
  Upload,
} from 'lucide-react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  XAxis,
  YAxis,
} from 'recharts';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart';
import services, { formatFileSize } from '@/lib/services';

const categoryMap: Record<string, string> = {
  图片: 'images',
  视频: 'videos',
  音频: 'audio',
  文档: 'documents',
  压缩包: 'archives',
  其他: 'others',
};

function useStatsChartConfig() {
  const t = useTranslations('admin.files');
  return {
    images: {
      label: t('stats.catImages'),
      color: 'hsl(var(--chart-1))',
    },
    videos: {
      label: t('stats.catVideos'),
      color: 'hsl(var(--chart-2))',
    },
    audio: {
      label: t('stats.catAudio'),
      color: 'hsl(var(--chart-3))',
    },
    documents: {
      label: t('stats.catDocuments'),
      color: 'hsl(var(--chart-4))',
    },
    archives: {
      label: t('stats.catArchives'),
      color: 'hsl(var(--chart-5))',
    },
    others: {
      label: t('stats.catOthers'),
      color: 'hsl(var(--chart-6))',
    },
    count: {
      label: t('stats.chartLabelCount'),
      color: 'hsl(var(--primary))',
    },
    size: {
      label: t('stats.chartLabelSize'),
      color: 'hsl(var(--chart-2))',
    },
  } satisfies ChartConfig;
}

export function FileStats() {
  const t = useTranslations('admin.files');
  const statsChartConfig = useStatsChartConfig();
  const [trendMetric, setTrendMetric] = React.useState<'count' | 'size'>(
    'count',
  );

  const statsQuery = useQuery({
    queryKey: ['files', 'stats'],
    queryFn: () => services.adminUpload.getFileStats(),
    staleTime: 0,
    refetchOnMount: 'always',
  });

  const stats = statsQuery.data;

  const trendData = React.useMemo(() => {
    if (!stats?.trend) return [];
    return stats.trend.map((item) => ({
      ...item,
      formattedDate: item.date.substring(5), // YYYY-MM-DD -> MM-DD
      value: trendMetric === 'count' ? item.count : item.size,
    }));
  }, [stats?.trend, trendMetric]);

  const categoryCountData = React.useMemo(() => {
    if (!stats?.categories) return [];
    return stats.categories
      .filter((c) => c.count > 0)
      .map((c) => {
        const key = categoryMap[c.name] || 'others';
        return {
          name: key,
          value: c.count,
          fill: `var(--color-${key})`,
        };
      });
  }, [stats?.categories]);

  const categorySizeData = React.useMemo(() => {
    if (!stats?.categories) return [];
    return stats.categories
      .filter((c) => c.size > 0)
      .map((c) => {
        const key = categoryMap[c.name] || 'others';
        return {
          name: key,
          value: c.size,
          fill: `var(--color-${key})`,
        };
      });
  }, [stats?.categories]);

  const maxStats = React.useMemo(() => {
    if (!stats?.categories || stats.categories.length === 0) {
      return {
        maxCountName: t('stats.none'),
        maxCount: 0,
        maxSizeName: t('stats.none'),
        maxSize: 0,
      };
    }
    let maxCountName = t('stats.none');
    let maxCount = 0;
    let maxSizeName = t('stats.none');
    let maxSize = 0;

    stats.categories.forEach((cat) => {
      if (cat.count > maxCount) {
        maxCount = cat.count;
        maxCountName = cat.name;
      }
      if (cat.size > maxSize) {
        maxSize = cat.size;
        maxSizeName = cat.name;
      }
    });

    return { maxCountName, maxCount, maxSizeName, maxSize };
  }, [stats?.categories, t]);

  const trendSummary = React.useMemo(() => {
    if (!stats?.trend) return { count: 0, size: 0 };
    return stats.trend.reduce(
      (acc, curr) => ({
        count: acc.count + curr.count,
        size: acc.size + curr.size,
      }),
      { count: 0, size: 0 },
    );
  }, [stats?.trend]);

  if (statsQuery.isLoading) {
    return (
      <div className='flex items-center justify-center py-32'>
        <Loader2 className='size-8 animate-spin text-sky-500' />
      </div>
    );
  }

  if (!stats || stats.total_count === 0) {
    return (
      <div className='flex flex-col items-center justify-center py-24 border border-dashed rounded-xl bg-card text-muted-foreground gap-4'>
        <Upload className='size-14 text-muted-foreground/25' />
        <div className='text-center space-y-1'>
          <p className='font-medium'>{t('stats.noData')}</p>
          <p className='text-xs text-muted-foreground'>
            {t('stats.noDataDesc')}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className='space-y-6'>
      {/* 四个指标卡片 */}
      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
        <Card className='bg-card/25 border-border/40 hover:shadow-md transition-shadow'>
          <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
            <CardTitle className='text-xs font-semibold tracking-tight text-muted-foreground'>
              {t('stats.totalFiles')}
            </CardTitle>
            <Files className='size-4 text-sky-500' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold font-mono text-sky-500'>
              {stats.total_count.toLocaleString()}{' '}
              <span className='text-xs font-normal text-muted-foreground'>
                {t('stats.countUnit')}
              </span>
            </div>
            <p className='text-[10px] text-muted-foreground mt-1'>
              {t('stats.totalFilesDesc')}
            </p>
          </CardContent>
        </Card>

        <Card className='bg-card/25 border-border/40 hover:shadow-md transition-shadow'>
          <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
            <CardTitle className='text-xs font-semibold tracking-tight text-muted-foreground'>
              {t('stats.totalSize')}
            </CardTitle>
            <Database className='size-4 text-emerald-500' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold font-mono text-emerald-500'>
              {formatFileSize(stats.total_size)}
            </div>
            <p className='text-[10px] text-muted-foreground mt-1'>
              {t('stats.totalSizeDesc')}
            </p>
          </CardContent>
        </Card>

        <Card className='bg-card/25 border-border/40 hover:shadow-md transition-shadow'>
          <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
            <CardTitle className='text-xs font-semibold tracking-tight text-muted-foreground'>
              {t('stats.weeklyNewFiles')}
            </CardTitle>
            <TrendingUp className='size-4 text-indigo-500' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold font-mono text-indigo-500'>
              +{trendSummary.count.toLocaleString()}{' '}
              <span className='text-xs font-normal text-muted-foreground'>
                {t('stats.countUnit')}
              </span>
            </div>
            <p className='text-[10px] text-muted-foreground mt-1'>
              {t('stats.weeklyNewFilesDesc')}
            </p>
          </CardContent>
        </Card>

        <Card className='bg-card/25 border-border/40 hover:shadow-md transition-shadow'>
          <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
            <CardTitle className='text-xs font-semibold tracking-tight text-muted-foreground'>
              {t('stats.weeklyNewSize')}
            </CardTitle>
            <HardDrive className='size-4 text-amber-500' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold font-mono text-amber-500'>
              +{formatFileSize(trendSummary.size)}
            </div>
            <p className='text-[10px] text-muted-foreground mt-1'>
              {t('stats.weeklyNewSizeDesc')}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* 7 天新增趋势折线图 */}
      <Card className='bg-card/20 border-border/40'>
        <CardHeader className='flex flex-row items-center justify-between'>
          <div>
            <CardTitle className='text-sm font-bold'>
              {t('stats.trendTitle')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('stats.trendDesc')}
            </CardDescription>
          </div>
          <div className='flex items-center gap-1 border rounded-lg p-0.5 bg-muted/30'>
            <Button
              size='sm'
              variant={trendMetric === 'count' ? 'secondary' : 'ghost'}
              className='h-7 text-xs px-2.5 rounded-md'
              onClick={() => setTrendMetric('count')}
            >
              {t('stats.trendCount')}
            </Button>
            <Button
              size='sm'
              variant={trendMetric === 'size' ? 'secondary' : 'ghost'}
              className='h-7 text-xs px-2.5 rounded-md'
              onClick={() => setTrendMetric('size')}
            >
              {t('stats.trendSize')}
            </Button>
          </div>
        </CardHeader>
        <CardContent className='pl-2 pr-4 pt-2'>
          <div className='h-[280px] w-full'>
            <ChartContainer config={statsChartConfig} className='w-full h-full'>
              <AreaChart
                data={trendData}
                margin={{ top: 10, right: 10, left: 10, bottom: 0 }}
              >
                <defs>
                  <linearGradient id='colorCount' x1='0' y1='0' x2='0' y2='1'>
                    <stop
                      offset='5%'
                      stopColor='var(--color-count)'
                      stopOpacity={0.3}
                    />
                    <stop
                      offset='95%'
                      stopColor='var(--color-count)'
                      stopOpacity={0.01}
                    />
                  </linearGradient>
                  <linearGradient id='colorSize' x1='0' y1='0' x2='0' y2='1'>
                    <stop
                      offset='5%'
                      stopColor='var(--color-size)'
                      stopOpacity={0.3}
                    />
                    <stop
                      offset='95%'
                      stopColor='var(--color-size)'
                      stopOpacity={0.01}
                    />
                  </linearGradient>
                </defs>
                <CartesianGrid
                  strokeDasharray='3 3'
                  vertical={false}
                  stroke='hsl(var(--border)/40)'
                />
                <XAxis
                  dataKey='formattedDate'
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  style={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }}
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  style={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }}
                  tickFormatter={(v) =>
                    trendMetric === 'size' ? formatFileSize(v) : v.toString()
                  }
                />
                <ChartTooltip
                  cursor={false}
                  content={
                    <ChartTooltipContent
                      hideLabel
                      formatter={(value) => {
                        const label =
                          trendMetric === 'count'
                            ? t('stats.chartLabelCount')
                            : t('stats.chartLabelSize');
                        const formattedValue =
                          trendMetric === 'size'
                            ? formatFileSize(Number(value))
                            : `${value} ${t('stats.countUnit')}`;
                        return (
                          <>
                            <div className='flex items-center gap-1.5'>
                              <span
                                className='size-2 rounded-full animate-pulse'
                                style={{
                                  backgroundColor:
                                    trendMetric === 'count'
                                      ? 'var(--color-count)'
                                      : 'var(--color-size)',
                                }}
                              />
                              <span className='text-muted-foreground'>
                                {label}
                              </span>
                            </div>
                            <span className='text-foreground font-mono font-medium tabular-nums ml-auto'>
                              {formattedValue}
                            </span>
                          </>
                        );
                      }}
                    />
                  }
                />
                <Area
                  type='monotone'
                  dataKey='value'
                  stroke={
                    trendMetric === 'count'
                      ? 'var(--color-count)'
                      : 'var(--color-size)'
                  }
                  strokeWidth={2.5}
                  fillOpacity={1}
                  fill={
                    trendMetric === 'count'
                      ? 'url(#colorCount)'
                      : 'url(#colorSize)'
                  }
                />
              </AreaChart>
            </ChartContainer>
          </div>
        </CardContent>
      </Card>

      {/* 两个饼图分布 */}
      <div className='grid gap-6 md:grid-cols-2'>
        {/* 饼图 1: 格式数量分布 */}
        <Card className='bg-card/20 border-border/40 flex flex-col'>
          <CardHeader>
            <CardTitle className='text-sm font-bold'>
              {t('stats.countDistTitle')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('stats.countDistDesc')}
            </CardDescription>
          </CardHeader>
          <CardContent className='flex-1 flex flex-col justify-between gap-4'>
            {categoryCountData.length === 0 ? (
              <div className='h-[240px] flex items-center justify-center text-xs text-muted-foreground'>
                {t('stats.noCategoryData')}
              </div>
            ) : (
              <div className='h-[240px] w-full'>
                <ChartContainer
                  config={statsChartConfig}
                  className='w-full h-full'
                >
                  <PieChart>
                    <Pie
                      data={categoryCountData}
                      cx='50%'
                      cy='50%'
                      innerRadius={60}
                      outerRadius={80}
                      paddingAngle={3}
                      dataKey='value'
                    >
                      {categoryCountData.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.fill} />
                      ))}
                    </Pie>
                    <ChartTooltip
                      cursor={false}
                      content={
                        <ChartTooltipContent
                          hideLabel
                          formatter={(value, name) => {
                            const configObj =
                              statsChartConfig[
                                name as keyof typeof statsChartConfig
                              ];
                            const label = configObj?.label || name;
                            const color =
                              configObj?.color || 'hsl(var(--muted))';
                            return (
                              <>
                                <div className='flex items-center gap-1.5'>
                                  <span
                                    className='size-2 rounded-full'
                                    style={{ backgroundColor: color }}
                                  />
                                  <span className='text-muted-foreground'>
                                    {label}
                                  </span>
                                </div>
                                <span className='text-foreground font-mono font-medium tabular-nums ml-auto'>
                                  {value} {t('stats.countUnit')}
                                </span>
                              </>
                            );
                          }}
                        />
                      }
                    />
                    <ChartLegend
                      content={
                        <ChartLegendContent
                          nameKey='name'
                          className='flex-wrap justify-center gap-x-4 gap-y-1 text-[11px] pt-4'
                        />
                      }
                    />
                  </PieChart>
                </ChartContainer>
              </div>
            )}
            <div className='bg-sky-500/5 border border-sky-500/10 rounded-lg p-3 flex items-start gap-2.5 text-xs text-sky-600 dark:text-sky-400'>
              <Info className='size-4 shrink-0 mt-0.5' />
              <p className='leading-normal'>
                {t('stats.maxCountHint', {
                  name: maxStats.maxCountName,
                  count: maxStats.maxCount,
                })}
              </p>
            </div>
          </CardContent>
        </Card>

        {/* 饼图 2: 格式容量分布 */}
        <Card className='bg-card/20 border-border/40 flex flex-col'>
          <CardHeader>
            <CardTitle className='text-sm font-bold'>
              {t('stats.sizeDistTitle')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('stats.sizeDistDesc')}
            </CardDescription>
          </CardHeader>
          <CardContent className='flex-1 flex flex-col justify-between gap-4'>
            {categorySizeData.length === 0 ? (
              <div className='h-[240px] flex items-center justify-center text-xs text-muted-foreground'>
                {t('stats.noSizeData')}
              </div>
            ) : (
              <div className='h-[240px] w-full'>
                <ChartContainer
                  config={statsChartConfig}
                  className='w-full h-full'
                >
                  <PieChart>
                    <Pie
                      data={categorySizeData}
                      cx='50%'
                      cy='50%'
                      innerRadius={60}
                      outerRadius={80}
                      paddingAngle={3}
                      dataKey='value'
                    >
                      {categorySizeData.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.fill} />
                      ))}
                    </Pie>
                    <ChartTooltip
                      cursor={false}
                      content={
                        <ChartTooltipContent
                          hideLabel
                          formatter={(value, name) => {
                            const configObj =
                              statsChartConfig[
                                name as keyof typeof statsChartConfig
                              ];
                            const label = configObj?.label || name;
                            const color =
                              configObj?.color || 'hsl(var(--muted))';
                            return (
                              <>
                                <div className='flex items-center gap-1.5'>
                                  <span
                                    className='size-2 rounded-full'
                                    style={{ backgroundColor: color }}
                                  />
                                  <span className='text-muted-foreground'>
                                    {label}
                                  </span>
                                </div>
                                <span className='text-foreground font-mono font-medium tabular-nums ml-auto'>
                                  {formatFileSize(Number(value))}
                                </span>
                              </>
                            );
                          }}
                        />
                      }
                    />
                    <ChartLegend
                      content={
                        <ChartLegendContent
                          nameKey='name'
                          className='flex-wrap justify-center gap-x-4 gap-y-1 text-[11px] pt-4'
                        />
                      }
                    />
                  </PieChart>
                </ChartContainer>
              </div>
            )}
            <div className='bg-emerald-500/5 border border-emerald-500/10 rounded-lg p-3 flex items-start gap-2.5 text-xs text-emerald-600 dark:text-emerald-400'>
              <Info className='size-4 shrink-0 mt-0.5' />
              <p className='leading-normal'>
                {t('stats.maxSizeHint', {
                  name: maxStats.maxSizeName,
                  size: formatFileSize(maxStats.maxSize),
                })}
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
