'use client';

import {useSearchParams} from 'next/navigation';

import {ProxyRouteConfigPage} from '@/features/proxy-routes/components/proxy-route-config-page';

export default function ProxyRouteDetailRoute() {
  const searchParams = useSearchParams();

  return (
    <ProxyRouteConfigPage
      routeId={searchParams.get('id') ?? ''}
      initialSection={searchParams.get('section') ?? 'domains'}
    />
  );
}
