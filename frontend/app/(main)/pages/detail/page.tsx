'use client';

import Link from 'next/link';
import { Suspense } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useRouter, useSearchParams } from 'next/navigation';
import { ArrowLeft, FileText } from 'lucide-react';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { PagesService } from '@/lib/services/openflare';

import { projectQueryKey } from '../components/pages-utils';
import { DeploymentsTab } from './tabs/deployments-tab';
import { SettingsTab } from './tabs/settings-tab';

type PagesDetailTab = 'deployments' | 'settings';

function resolveTab(value: string | null): PagesDetailTab {
  if (value === 'settings') return 'settings';
  return 'deployments';
}

function PagesDetailPageFallback() {
  return (
    <div className='flex w-full flex-col gap-6 px-1 py-6'>
      <Skeleton className='h-8 w-32' />
      <Skeleton className='h-12 w-full max-w-xl' />
      <Skeleton className='h-10 w-64' />
      <Skeleton className='h-64 w-full' />
    </div>
  );
}

function PagesDetailRoute() {
  const searchParams = useSearchParams();
  const router = useRouter();

  const rawProjectId = searchParams.get('id')?.trim() ?? '';
  const projectId = Number(rawProjectId);
  const validProjectId =
    rawProjectId !== '' && Number.isInteger(projectId) && projectId > 0;
  const tab = resolveTab(searchParams.get('tab'));

  const projectQuery = useQuery({
    queryKey: projectQueryKey(projectId),
    queryFn: () => PagesService.getProject(projectId),
    enabled: validProjectId,
  });

  const handleTabChange = (value: string) => {
    const next = resolveTab(value);
    const params = new URLSearchParams();
    params.set('id', String(projectId));
    if (next === 'settings') {
      params.set('tab', 'settings');
    }
    router.replace(`/pages/detail?${params.toString()}`);
  };

  if (!validProjectId) {
    return (
      <div className='w-full px-1 py-6'>
        <EmptyStateWithBorder description='缺少有效的 Pages 项目 ID。' />
      </div>
    );
  }

  if (projectQuery.isLoading) {
    return (
      <div className='w-full px-1 py-6'>
        <LoadingStateWithBorder icon={FileText} description='加载项目详情...' />
      </div>
    );
  }

  if (projectQuery.isError) {
    return (
      <div className='w-full px-1 py-6'>
        <div className='rounded-lg border p-4'>
          <ErrorInline
            message={
              projectQuery.error instanceof Error
                ? projectQuery.error.message
                : '项目详情加载失败'
            }
            onRetry={() => void projectQuery.refetch()}
          />
        </div>
      </div>
    );
  }

  const project = projectQuery.data;
  if (!project) {
    return (
      <div className='flex w-full flex-col gap-4 px-1 py-6'>
        <Button variant='ghost' size='sm' asChild>
          <Link href='/pages'>
            <ArrowLeft data-icon='inline-start' />
            返回列表
          </Link>
        </Button>
        <EmptyStateWithBorder description='Pages 项目不存在或已被删除。' />
      </div>
    );
  }

  return (
    <div className='flex w-full flex-col gap-6 px-1 py-6'>
      <div className='flex flex-col gap-4'>
        <Button variant='ghost' size='sm' className='self-start' asChild>
          <Link href='/pages'>
            <ArrowLeft data-icon='inline-start' />
            返回列表
          </Link>
        </Button>
        <div className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between'>
          <div className='flex flex-col gap-2'>
            <div className='flex items-center gap-2'>
              <FileText className='size-5 text-primary' />
              <h1 className='text-2xl font-semibold tracking-tight'>
                {project.name}
              </h1>
              <Badge variant={project.enabled ? 'secondary' : 'outline'}>
                {project.enabled ? '已启用' : '已停用'}
              </Badge>
            </div>
            <p className='text-sm text-muted-foreground'>
              {project.slug} · {project.deployment_count} 个部署
            </p>
          </div>
        </div>
      </div>

      <Tabs
        value={tab}
        onValueChange={handleTabChange}
        className='w-full'
      >
        <TabsList variant='line' className='mb-6 inline-flex w-fit gap-8'>
          <TabsTrigger
            value='deployments'
            className='px-0 pb-2 text-xs font-semibold'
          >
            部署
          </TabsTrigger>
          <TabsTrigger
            value='settings'
            className='px-0 pb-2 text-xs font-semibold'
          >
            设置
          </TabsTrigger>
        </TabsList>

        <TabsContent
          value='deployments'
          className='focus-visible:outline-none'
        >
          <DeploymentsTab project={project} />
        </TabsContent>

        <TabsContent value='settings' className='focus-visible:outline-none'>
          <SettingsTab project={project} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default function PagesDetailPage() {
  return (
    <Suspense fallback={<PagesDetailPageFallback />}>
      <PagesDetailRoute />
    </Suspense>
  );
}
