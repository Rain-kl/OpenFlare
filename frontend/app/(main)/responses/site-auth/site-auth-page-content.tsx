'use client';

import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { useAuth } from '@/components/providers/auth-provider';
import { OptionService } from '@/lib/services/openflare';

import { OPTIONS_QUERY_KEY } from '../components/shared';
import { SITE_AUTH_HTML_KEY } from '../components/site-auth-page-html';

export function SiteAuthPageContent({
  children,
}: {
  children: (html: string) => ReactNode;
}) {
  const t = useTranslations('responses');
  const { user, loading: authLoading } = useAuth();
  const options = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  if (options.data && user?.is_admin && !authLoading) {
    const html = options.data.find(({ key }) => key === SITE_AUTH_HTML_KEY);
    return children(html?.value ?? '');
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
