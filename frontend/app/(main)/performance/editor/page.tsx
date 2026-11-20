import { Suspense } from 'react';
import { PolicyEditorPageClient, PolicyEditorSkeleton } from './page-client';

export default function PerformancePolicyEditorPage() {
  return (
    <Suspense fallback={<PolicyEditorSkeleton />}>
      <PolicyEditorPageClient />
    </Suspense>
  );
}
