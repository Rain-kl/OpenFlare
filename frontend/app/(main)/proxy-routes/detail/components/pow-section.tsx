'use client';

import { useQuery } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';

import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  defaultPoWConfig,
  PoWService,
  type ProxyRouteItem,
} from '@/lib/services/openflare';
import { PolicyEditor } from './pow/policy-editor';

export function PoWSection({ route }: { route: ProxyRouteItem }) {
  const t = useTranslations('pow');
  const query = useQuery({
    queryKey: ['openflare', 'pow', route.id],
    queryFn: async () => {
      const policies = await PoWService.list();
      return policies.find((policy) => policy.route_id === route.id) ?? null;
    },
  });

  if (query.isLoading)
    return <LoadingStateWithBorder description={t('loading')} />;
  if (query.isError)
    return (
      <ErrorInline
        message={
          query.error instanceof Error ? query.error.message : t('loadFailed')
        }
        onRetry={() => void query.refetch()}
      />
    );

  return (
    <PolicyEditor
      key={route.id}
      policy={
        query.data ?? {
          route_id: route.id,
          enabled: false,
          config: defaultPoWConfig,
        }
      }
      configured={!!query.data}
    />
  );
}
