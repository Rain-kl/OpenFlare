/*
Copyright 2026 Arctel.net

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  BarChart3,
  Globe,
  RefreshCw,
  TrendingUp,
  Users,
  XCircle,
} from 'lucide-react';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts';

import services from '@/lib/services';
import { useTranslations } from 'next-intl';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { EmptyStateWithBorder } from '@/components/layout/empty';
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
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart';
import { Spinner } from '@/components/ui/spinner';

interface TrendData {
  date: string;
  count: number;
}

interface BrowserData {
  browser: string;
  count: number;
}

interface TopUserData {
  user_id: string;
  username: string;
  nickname: string;
  count: number;
}

function useAnalyticsChartConfig() {
  const t = useTranslations('admin.logs.analytics');
  return {
    count: {
      label: t('requestVolume'),
      color: 'hsl(var(--primary))',
    },
  } satisfies ChartConfig;
}

// Format YYYY-MM-DD to MM/DD
function formatDateLabel(dateStr: string) {
  if (!dateStr || dateStr.length < 10) return dateStr;
  const parts = dateStr.split('-');
  if (parts.length === 3) {
    return `${parts[1]}/${parts[2]}`;
  }
  return dateStr;
}

export function AccessAnalytics() {
  const t = useTranslations('admin.logs.analytics');
  const chartConfig = useAnalyticsChartConfig();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [clickhouseDisabled, setClickhouseDisabled] = useState(false);

  const [trend, setTrend] = useState<TrendData[]>([]);
  const [browsers, setBrowsers] = useState<BrowserData[]>([]);
  const [topUsers, setTopUsers] = useState<TopUserData[]>([]);

  const fetchAnalytics = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await services.adminLog.getLogsAnalytics();

      // Formats the date labels for the X-axis representation
      const formattedTrend = (data.trend || []).map((item) => ({
        ...item,
        formattedDate: formatDateLabel(item.date),
      }));

      setTrend(formattedTrend);
      setBrowsers(data.browsers || []);
      setTopUsers(data.top_users || []);
      setClickhouseDisabled(false);
    } catch (err) {
      const errorInstance =
        err instanceof Error ? err : new Error(t('fetchFailed'));
      const errMsg = errorInstance.message || '';
      if (errMsg.includes('ClickHouse') || errMsg.includes('未启用')) {
        setClickhouseDisabled(true);
      } else {
        setError(errorInstance);
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchAnalytics();
  }, [fetchAnalytics]);

  const totalBrowserRequests = browsers.reduce(
    (sum, item) => sum + item.count,
    0,
  );
  const totalTrendRequests = trend.reduce((sum, item) => sum + item.count, 0);

  if (clickhouseDisabled) {
    return (
      <div className='flex flex-col items-center justify-center p-8 border border-dashed rounded-lg bg-card text-center my-6 min-h-[300px]'>
        <XCircle className='size-10 text-muted-foreground mb-3' />
        <h3 className='text-base font-semibold'>{t('clickhouseDisabled')}</h3>
        <p className='text-sm text-muted-foreground mt-1 max-w-[400px] mb-4'>
          {t('clickhouseDisabledDesc')}
        </p>
      </div>
    );
  }

  if (error) {
    return <ErrorInline error={error} onRetry={fetchAnalytics} />;
  }

  if (loading) {
    return (
      <LoadingStateWithBorder
        icon={BarChart3}
        description={t('loadingAnalytics')}
      />
    );
  }

  return (
    <div className='space-y-6'>
      {/* Overview Cards */}
      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
        <Card className='bg-card/25 border-border/40'>
          <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
            <CardTitle className='text-sm font-semibold tracking-tight'>
              {t('totalRequests7d')}
            </CardTitle>
            <TrendingUp className='size-4 text-muted-foreground' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold font-mono'>
              {totalTrendRequests.toLocaleString()}
            </div>
            <p className='text-[10px] text-muted-foreground mt-1'>
              {t('totalRequests7dDesc')}
            </p>
          </CardContent>
        </Card>
        <Card className='bg-card/25 border-border/40'>
          <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
            <CardTitle className='text-sm font-semibold tracking-tight'>
              {t('activeBrowserTypes')}
            </CardTitle>
            <Globe className='size-4 text-muted-foreground' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold font-mono'>
              {browsers.length}
            </div>
            <p className='text-[10px] text-muted-foreground mt-1'>
              {t('activeBrowserTypesDesc')}
            </p>
          </CardContent>
        </Card>
        <Card className='bg-card/25 border-border/40 sm:col-span-2 lg:col-span-1'>
          <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
            <CardTitle className='text-sm font-semibold tracking-tight'>
              {t('activeUsers')}
            </CardTitle>
            <Users className='size-4 text-muted-foreground' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold font-mono'>
              {topUsers.length}
            </div>
            <p className='text-[10px] text-muted-foreground mt-1'>
              {t('activeUsersDesc')}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Access Trend Chart */}
      <Card className='bg-card/20 border-border/40'>
        <CardHeader className='flex flex-row items-center justify-between'>
          <div>
            <CardTitle className='text-base font-bold'>
              {t('weeklyAccessTrend')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('weeklyAccessTrendDesc')}
            </CardDescription>
          </div>
          <Button
            variant='ghost'
            size='icon'
            className='size-8'
            onClick={fetchAnalytics}
            disabled={loading}
          >
            {loading ? (
              <Spinner className='size-4' />
            ) : (
              <RefreshCw className='size-4 text-muted-foreground' />
            )}
          </Button>
        </CardHeader>
        <CardContent className='pl-2 pr-4 pt-2'>
          {trend.length === 0 ? (
            <EmptyStateWithBorder
              icon={TrendingUp}
              description={t('noTrendData')}
            />
          ) : (
            <div className='h-[280px] w-full'>
              <ChartContainer
                config={chartConfig}
                className='w-full h-full aspect-auto'
              >
                <AreaChart
                  data={trend}
                  margin={{ top: 10, right: 10, left: -20, bottom: 0 }}
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
                  </defs>
                  <CartesianGrid strokeDasharray='3 3' vertical={false} />
                  <XAxis
                    dataKey='formattedDate'
                    tickLine={false}
                    axisLine={false}
                    dy={10}
                    className='font-mono'
                  />
                  <YAxis
                    tickLine={false}
                    axisLine={false}
                    dx={-10}
                    className='font-mono'
                  />
                  <ChartTooltip content={<ChartTooltipContent />} />
                  <Area
                    type='monotone'
                    dataKey='count'
                    stroke='var(--color-count)'
                    strokeWidth={2}
                    fillOpacity={1}
                    fill='url(#colorCount)'
                    name={t('requestCount')}
                  />
                </AreaChart>
              </ChartContainer>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Two Columns for Ranking Statistics */}
      <div className='grid gap-6 md:grid-cols-2'>
        {/* Browser Rankings */}
        <Card className='bg-card/20 border-border/40 flex flex-col h-full'>
          <CardHeader>
            <CardTitle className='text-base font-bold flex items-center gap-1.5'>
              <Globe className='size-4.5 text-muted-foreground' />
              {t('browserRanking')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('browserRankingDesc')}
            </CardDescription>
          </CardHeader>
          <CardContent className='flex-1'>
            {browsers.length === 0 ? (
              <EmptyStateWithBorder
                icon={Globe}
                description={t('noBrowserData')}
              />
            ) : (
              <div className='space-y-4'>
                {browsers.map((item, index) => {
                  const percent =
                    totalBrowserRequests > 0
                      ? (item.count / totalBrowserRequests) * 100
                      : 0;
                  return (
                    <div key={item.browser} className='space-y-1.5'>
                      <div className='flex items-center justify-between text-xs font-medium'>
                        <span className='flex items-center gap-2'>
                          <Badge
                            variant='outline'
                            className='px-1.5 py-0 h-5 font-mono text-[10px] select-none'
                          >
                            {index + 1}
                          </Badge>
                          <span className='font-semibold'>{item.browser}</span>
                        </span>
                        <span className='text-muted-foreground font-mono text-xs'>
                          {t('countTimes', {
                            count: item.count.toLocaleString(),
                            percent: percent.toFixed(1),
                          })}
                        </span>
                      </div>
                      <div className='h-2 w-full rounded-full bg-muted overflow-hidden'>
                        <div
                          className='h-full bg-primary rounded-full transition-all duration-500'
                          style={{ width: `${percent}%` }}
                        />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Top Users */}
        <Card className='bg-card/20 border-border/40 flex flex-col h-full'>
          <CardHeader>
            <CardTitle className='text-base font-bold flex items-center gap-1.5'>
              <Users className='size-4.5 text-muted-foreground' />
              {t('topActiveUsers')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('topActiveUsersDesc')}
            </CardDescription>
          </CardHeader>
          <CardContent className='flex-1'>
            {topUsers.length === 0 ? (
              <EmptyStateWithBorder
                icon={Users}
                description={t('noActiveUsersData')}
              />
            ) : (
              <div className='divide-y divide-border/40'>
                {topUsers.map((user) => (
                  <div
                    key={user.user_id}
                    className='flex items-center justify-between py-2.5 first:pt-0 last:pb-0'
                  >
                    <div className='flex items-center gap-3'>
                      <div className='flex size-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary font-bold text-xs select-none'>
                        {(user.username || 'U').slice(0, 1).toUpperCase()}
                      </div>
                      <div className='truncate max-w-[180px] sm:max-w-xs'>
                        <div className='text-xs font-bold leading-tight truncate'>
                          {user.username || t('unknown')}
                        </div>
                        <div className='text-[10px] text-muted-foreground leading-normal truncate'>
                          {user.nickname
                            ? `(${user.nickname})`
                            : t('noNickname')}{' '}
                          |
                          ID: {user.user_id}
                        </div>
                      </div>
                    </div>
                    <div className='text-right shrink-0'>
                      <div className='text-xs font-bold font-mono'>
                        {user.count.toLocaleString()}
                      </div>
                      <div className='text-[9px] text-muted-foreground uppercase font-bold tracking-wider'>
                        {t('requests')}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
