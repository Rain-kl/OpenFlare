import { Suspense } from 'react';

import { Skeleton } from '@/components/ui/skeleton';

import { NodesPageClient } from './page-client';

function NodesPageFallback() {
  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex items-center justify-between gap-3'>
        <Skeleton className='h-8 w-40' />
        <div className='flex gap-2'>
          <Skeleton className='size-8' />
          <Skeleton className='size-8' />
        </div>
      </div>
      <Skeleton className='h-96 w-full' />
    </div>
  );
}

export default function NodesPage() {
  return (
    <Suspense fallback={<NodesPageFallback />}>
      <NodesPageClient />
    </Suspense>
  );
}
