import { Suspense } from 'react';

import { Skeleton } from '@/components/ui/skeleton';

import { TunnelDetailPageClient } from './page-client';

function TunnelDetailFallback() {
  return (
    <div className='py-6 px-1 space-y-6'>
      <Skeleton className='h-8 w-48' />
      <Skeleton className='h-36 w-full' />
      <Skeleton className='h-64 w-full' />
    </div>
  );
}

export default function TunnelDetailPage() {
  return (
    <Suspense fallback={<TunnelDetailFallback />}>
      <TunnelDetailPageClient />
    </Suspense>
  );
}
