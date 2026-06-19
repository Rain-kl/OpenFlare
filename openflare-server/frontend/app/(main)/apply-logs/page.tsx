import {Suspense} from 'react';

import {Skeleton} from '@/components/ui/skeleton';

import {ApplyLogsPageClient} from './page-client';

function ApplyLogsPageFallback() {
  return (
    <div className="py-6 px-1 space-y-6">
      <Skeleton className="h-8 w-40" />
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
      <Skeleton className="h-96 w-full" />
    </div>
  );
}

export default function ApplyLogsPage() {
  return (
    <Suspense fallback={<ApplyLogsPageFallback />}>
      <ApplyLogsPageClient />
    </Suspense>
  );
}