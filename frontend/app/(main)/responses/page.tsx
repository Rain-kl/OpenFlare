'use client';

import { Suspense, useMemo } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { MessageSquareText } from 'lucide-react';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { useAuth } from '@/components/providers/auth-provider';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { OptionService } from '@/lib/services/openflare';

import { ErrorPageTab } from './components/error-page-tab';
import { OfflinePageTab } from './components/offline-page-tab';
import { useTranslations } from 'next-intl';

import { OPTIONS_QUERY_KEY, optionsToMap } from './components/shared';

type ResponseTab = 'error' | 'offline';

function resolveTab(tabParam: string | null): ResponseTab {
  if (tabParam === 'offline') {
    return 'offline';
  }
  return 'error';
}

function ResponsesPageContent() {
  const t = useTranslations('responses');
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();

  const activeTab = resolveTab(searchParams.get('tab'));

  const handleTabChange = (value: string) => {
    const nextTab = resolveTab(value);
    const params = new URLSearchParams(searchParams.toString());
    if (nextTab === 'error') {
      params.delete('tab');
    } else {
      params.set('tab', nextTab);
    }
    const query = params.toString();
    router.replace(query ? `/responses?${query}` : '/responses', {
      scroll: false,
    });
  };

  const optionsQuery = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  const optionMap = useMemo(
    () => optionsToMap(optionsQuery.data ?? []),
    [optionsQuery.data],
  );

  if (authLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder
          icon={MessageSquareText}
          description={t('loadingPermission')}
        />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='w-full py-6 px-1'>
        <EmptyStateWithBorder
          icon={MessageSquareText}
          title={t('forbidden')}
          description={t('forbiddenSettings')}
        />
      </div>
    );
  }

  if (optionsQuery.isLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder
          icon={MessageSquareText}
          description={t('loadingConfig')}
        />
      </div>
    );
  }

  if (optionsQuery.isError) {
    return (
      <div className='w-full py-6 px-1'>
        <ErrorInline
          message={
            optionsQuery.error instanceof Error
              ? optionsQuery.error.message
              : t('loadFailed')
          }
          onRetry={() => void optionsQuery.refetch()}
        />
      </div>
    );
  }

  if (!optionsQuery.data) return null;

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex items-center gap-2'>
        <MessageSquareText className='size-5 text-primary' />
        <div>
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
          <p className='text-sm text-muted-foreground'>{t('subtitle')}</p>
        </div>
      </div>

      <Tabs
        value={activeTab}
        onValueChange={handleTabChange}
        className='w-full'
      >
        <TabsList variant='line' className='mb-6 inline-flex w-fit gap-8'>
          <TabsTrigger
            value='error'
            className='px-0 pb-2 text-xs font-semibold'
          >
            {t('errorPage')}
          </TabsTrigger>
          <TabsTrigger
            value='offline'
            className='px-0 pb-2 text-xs font-semibold'
          >
            {t('offlinePage')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value='error' className='focus-visible:outline-none'>
          <ErrorPageTab optionMap={optionMap} />
        </TabsContent>

        <TabsContent value='offline' className='focus-visible:outline-none'>
          <OfflinePageTab optionMap={optionMap} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default function ResponsesPage() {
  const t = useTranslations('responses');
  return (
    <Suspense
      fallback={
        <div className='w-full py-6 px-1'>
          <LoadingStateWithBorder
            icon={MessageSquareText}
            description={t('loadingPage')}
          />
        </div>
      }
    >
      <ResponsesPageContent />
    </Suspense>
  );
}
