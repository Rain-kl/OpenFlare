'use client';

import {useSearchParams} from 'next/navigation';

import {PagesProjectDetailPage} from '@/features/pages/components/pages-page';

export default function PagesProjectDetailRoute() {
  const searchParams = useSearchParams();

  return <PagesProjectDetailPage projectId={searchParams.get('id') ?? ''} />;
}
