'use client';

import {useSearchParams} from 'next/navigation';

import {NodeDetailPage} from '@/features/nodes/components/node-detail-page';

export default function NodeDetailRoute() {
  const searchParams = useSearchParams();

  return <NodeDetailPage nodeId={searchParams.get('id') ?? ''} />;
}
