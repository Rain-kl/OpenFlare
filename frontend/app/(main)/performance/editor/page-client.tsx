'use client';

import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Gauge } from 'lucide-react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { PerformanceService } from '@/lib/services/openflare';
import {
  PolicyEditor,
  performancePoliciesQueryKey,
} from '../components/policy-editor';

export function PolicyEditorSkeleton() {
  return (
    <div className='flex w-full flex-col gap-6 px-1 py-6'>
      <div className='flex flex-col gap-4'>
        <Skeleton className='h-8 w-32' />
        <div className='flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between'>
          <div className='flex flex-col gap-2'>
            <Skeleton className='h-8 w-48' />
            <Skeleton className='h-4 w-80 max-w-full' />
          </div>
          <div className='flex gap-2'>
            <Skeleton className='h-9 w-20' />
            <Skeleton className='h-9 w-24' />
          </div>
        </div>
      </div>
      <Skeleton className='h-32 w-full' />
      <Skeleton className='h-9 w-96 max-w-full' />
      <Skeleton className='h-80 w-full' />
    </div>
  );
}

function BackLink() {
  const t = useTranslations('performancePolicies');
  return (
    <Button variant='ghost' size='sm' className='-ml-2 w-fit text-xs' asChild>
      <Link href='/performance'>
        <ArrowLeft data-icon='inline-start' />
        {t('back')}
      </Link>
    </Button>
  );
}

export function PolicyEditorPageClient() {
  const t = useTranslations('performancePolicies');
  const searchParams = useSearchParams();
  const rawId = searchParams.get('id')?.trim() ?? '';
  const hasId = rawId !== '';
  const parsedId = Number(rawId);
  const validId = !hasId || (Number.isSafeInteger(parsedId) && parsedId > 0);

  const query = useQuery({
    queryKey: performancePoliciesQueryKey,
    queryFn: () => PerformanceService.list(),
    enabled: validId,
  });

  if (!validId) {
    return (
      <div className='flex w-full flex-col gap-6 px-1 py-6'>
        <BackLink />
        <EmptyStateWithBorder icon={Gauge} description={t('invalidId')} />
      </div>
    );
  }

  if (query.isPending) {
    return <PolicyEditorSkeleton />;
  }

  if (query.isError) {
    return (
      <div className='flex w-full flex-col gap-6 px-1 py-6'>
        <BackLink />
        <div className='rounded-lg border border-dashed p-6'>
          <ErrorInline
            message={
              query.error instanceof Error
                ? query.error.message
                : t('loadFailed')
            }
            onRetry={() => void query.refetch()}
            className='justify-center'
          />
        </div>
      </div>
    );
  }

  const policy = hasId
    ? query.data.policies.find((item) => item.id === parsedId)
    : undefined;

  if (hasId && !policy) {
    return (
      <div className='flex w-full flex-col gap-6 px-1 py-6'>
        <BackLink />
        <EmptyStateWithBorder icon={Gauge} description={t('notFound')} />
      </div>
    );
  }

  return (
    <PolicyEditor
      key={policy ? policy.id : 'new'}
      policy={policy}
      parameters={query.data.parameters}
    />
  );
}
