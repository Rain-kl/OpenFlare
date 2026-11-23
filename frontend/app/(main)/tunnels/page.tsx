import { Suspense } from 'react';

import { Skeleton } from '@/components/ui/skeleton';

import { TunnelsPageClient } from './page-client';

function TunnelsPageFallback() {
  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex items-center justify-between gap-3'>
        <Skeleton className='h-8 w-44' />
        <div className='flex gap-2'>
          <Skeleton className='h-7 w-24' />
          <Skeleton className='h-7 w-28' />
        </div>
      </div>
      <Skeleton className='h-96 w-full' />
    </div>
  );
}

export default function TunnelsPage() {
  return (
    <Suspense fallback={<TunnelsPageFallback />}>
      <TunnelsPageClient />
    </Suspense>
  );
}
