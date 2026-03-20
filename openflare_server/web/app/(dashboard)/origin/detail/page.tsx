'use client';

import { useSearchParams } from 'next/navigation';

import { OriginDetailPage } from '@/features/origins/components/origin-detail-page';

export default function OriginDetailRoute() {
  const searchParams = useSearchParams();

  return <OriginDetailPage originId={searchParams.get('id') ?? ''} />;
}
