'use client';

import Link from 'next/link';
import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Pencil, X } from 'lucide-react';

import { useAuth } from '@/components/providers/auth-provider';
import { Button } from '@/components/ui/button';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { OptionService } from '@/lib/services/openflare';

import { effectiveOfflinePageHTML } from '@/lib/openflare/offline-page-templates';
import { useTranslations } from 'next-intl';

import {
  mapOptionsToOfflineFields,
  OPTIONS_QUERY_KEY,
  optionsToMap,
} from '../../components/shared';

export default function OfflinePagePreviewPage() {
  const t = useTranslations('responses');
  const { user, loading: authLoading } = useAuth();

  const optionsQuery = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  const html = useMemo(() => {
    if (!optionsQuery.data) return '';
    const rawHtml = mapOptionsToOfflineFields(
      optionsToMap(optionsQuery.data),
    ).html;
    return effectiveOfflinePageHTML(rawHtml);
  }, [optionsQuery.data]);

  if (authLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder description={t('loadingPermission')} />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='w-full py-6 px-1'>
        <EmptyStateWithBorder
          title={t('forbidden')}
          description={t('forbiddenPreviewOffline')}
        />
      </div>
    );
  }

  if (optionsQuery.isLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder description={t('loadingOfflineConfig')} />
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

  return (
    <div className='fixed inset-0 z-50 flex flex-col bg-background'>
      <div className='flex items-center justify-between gap-3 border-b bg-background/95 px-4 py-2 backdrop-blur shrink-0'>
        <div className='flex items-center gap-2 min-w-0'>
          <Button variant='outline' size='icon' className='h-8 w-8' asChild>
            <Link href='/responses?tab=offline' aria-label={t('backToOffline')}>
              <ArrowLeft className='size-4' />
            </Link>
          </Button>
          <div className='min-w-0'>
            <p className='text-sm font-semibold truncate'>
              {t('offlinePreview')}
            </p>
            <p className='text-[11px] text-muted-foreground font-mono truncate'>
              {t('fullscreen')}
            </p>
          </div>
        </div>
        <div className='flex items-center gap-2 shrink-0'>
          <Button variant='outline' size='sm' asChild>
            <Link href='/responses/offline/edit'>
              <Pencil className='size-3.5' />
              {t('edit')}
            </Link>
          </Button>
          <Button variant='ghost' size='icon' className='h-8 w-8' asChild>
            <Link href='/responses?tab=offline' aria-label={t('closePreview')}>
              <X className='size-4' />
            </Link>
          </Button>
        </div>
      </div>
      <iframe
        title={t('offlinePreview')}
        sandbox=''
        srcDoc={html}
        className='flex-1 w-full border-0 bg-background min-h-0'
      />
    </div>
  );
}
