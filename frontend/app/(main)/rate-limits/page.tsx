'use client';

import { Suspense, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Gauge } from 'lucide-react';

import { useAuth } from '@/components/providers/auth-provider';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { AccessLogService } from '@/lib/services/openflare';

import type { RateLimitRangeHours } from '../access-logs/components/access-log-utils';
import { AnalysisTab } from './components/analysis-tab';
import { ConfigTab } from './components/config-tab';

type RateLimitTab = 'analysis' | 'config';

function resolveTab(value: string | null): RateLimitTab {
  if (value === 'config') return 'config';
  return 'analysis';
}

function RateLimitsPageContent() {
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();
  const tab = resolveTab(searchParams.get('tab'));
  const [hours, setHours] = useState<RateLimitRangeHours>(24);
  const [hosts, setHosts] = useState<string[]>([]);

  const overviewQuery = useQuery({
    queryKey: ['openflare', 'rate-limits', 'overview', hours, hosts, 3],
    queryFn: () =>
      AccessLogService.getOverview({
        hours,
        hosts: hosts.length > 0 ? hosts : undefined,
        bucket_minutes: 3,
      }),
    enabled: !!user?.is_admin && tab === 'analysis',
  });

  const handleTabChange = (value: string) => {
    const next = resolveTab(value);
    router.replace(
      next === 'analysis' ? '/rate-limits' : '/rate-limits?tab=config',
    );
  };

  if (authLoading) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder icon={Gauge} description='加载权限信息...' />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='py-6 px-1'>
        <EmptyStateWithBorder
          icon={Gauge}
          title='权限不足'
          description='只有管理员可以访问限流设置。'
        />
      </div>
    );
  }

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex items-center gap-2'>
        <Gauge className='size-5 text-primary' />
        <div>
          <h1 className='text-2xl font-semibold tracking-tight'>限流</h1>
          <p className='text-sm text-muted-foreground'>
            查看边缘请求压力，并配置站点默认并发与带宽限流。
          </p>
        </div>
      </div>

      <Tabs value={tab} onValueChange={handleTabChange}>
        <TabsList className='grid w-full max-w-md grid-cols-2'>
          <TabsTrigger value='analysis'>分析</TabsTrigger>
          <TabsTrigger value='config'>配置</TabsTrigger>
        </TabsList>

        <TabsContent value='analysis' className='mt-4'>
          <AnalysisTab
            data={overviewQuery.data}
            loading={overviewQuery.isLoading}
            error={
              overviewQuery.error instanceof Error ? overviewQuery.error : null
            }
            hours={hours}
            hosts={hosts}
            onHoursChange={setHours}
            onHostsChange={setHosts}
            onRetry={() => void overviewQuery.refetch()}
          />
        </TabsContent>

        <TabsContent value='config' className='mt-4'>
          <ConfigTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default function RateLimitsPage() {
  return (
    <Suspense
      fallback={
        <div className='py-6 px-1'>
          <LoadingStateWithBorder icon={Gauge} description='加载限流页面...' />
        </div>
      }
    >
      <RateLimitsPageContent />
    </Suspense>
  );
}
