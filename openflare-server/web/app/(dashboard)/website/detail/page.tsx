'use client';

import {useSearchParams} from 'next/navigation';

import {WebsiteDetailPage} from '@/features/websites/components/website-detail-page';

export default function WebsiteDetailRoute() {
  const searchParams = useSearchParams();

  return <WebsiteDetailPage websiteId={searchParams.get('id') ?? ''} />;
}
