import {Suspense} from 'react';

import {Skeleton} from '@/components/ui/skeleton';

import {NodesPageClient} from './page-client';

function NodesPageFallback() {
  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex items-center justify-between gap-3">
        <Skeleton className="h-8 w-40" />
        <div className="flex gap-2">
          <Skeleton className="h-7 w-20" />
          <Skeleton className="h-7 w-24" />
        </div>
      </div>
      <Skeleton className="h-96 w-full" />
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