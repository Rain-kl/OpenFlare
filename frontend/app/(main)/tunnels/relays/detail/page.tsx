import { Suspense } from 'react';

import { Skeleton } from '@/components/ui/skeleton';

import { RelayDetailPageClient } from './page-client';

function RelayDetailFallback() {
  return (
    <div className='py-6 px-1 space-y-6'>
      <Skeleton className='h-8 w-48' />
      <Skeleton className='h-48 w-full' />
    </div>
  );
}

export default function RelayDetailPage() {
  return (
    <Suspense fallback={<RelayDetailFallback />}>
      <RelayDetailPageClient />
    </Suspense>
  );
}
