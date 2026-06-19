import {Suspense} from 'react';

import {Skeleton} from '@/components/ui/skeleton';

import {ProxyRouteDetailPageClient} from './page-client';

function DetailPageFallback() {
  return (
    <div className="py-6 px-1 space-y-6">
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-10 w-full max-w-xl" />
      <Skeleton className="h-64 w-full" />
    </div>
  );
}

export default function ProxyRouteDetailPage() {
  return (
    <Suspense fallback={<DetailPageFallback />}>
      <ProxyRouteDetailPageClient />
    </Suspense>
  );
}
