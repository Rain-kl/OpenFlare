'use client';

import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { useAuth } from '@/components/providers/auth-provider';
import { OptionService } from '@/lib/services/openflare';

import { OPTIONS_QUERY_KEY } from './shared';

export function ResponsePageOption({
  optionKey,
  children,
}: {
  optionKey: string;
  children: (value: string) => ReactNode;
}) {
  const t = useTranslations('responses');
  const { user, loading: authLoading } = useAuth();
  const options = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  if (options.data && user?.is_admin && !authLoading) {
    const option = options.data.find(({ key }) => key === optionKey);
    return children(option?.value ?? '');
  }

  return (
    <div className='w-full py-6 px-1'>
      {authLoading ? (
        <LoadingStateWithBorder description={t('loadingPermission')} />
      ) : !user?.is_admin ? (
        <EmptyStateWithBorder
          title={t('forbidden')}
          description={t('forbiddenSettings')}
        />
      ) : options.isError ? (
        <ErrorInline
          message={options.error.message}
          onRetry={() => void options.refetch()}
        />
      ) : (
        <LoadingStateWithBorder description={t('loadingConfig')} />
      )}
    </div>
  );
}
