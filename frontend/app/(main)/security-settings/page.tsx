'use client';

import { SlidersHorizontal } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { useAuth } from '@/components/providers/auth-provider';

import { RateLimitConfigCard } from './components/rate-limit-config-card';

export default function SecuritySettingsPage() {
  const t = useTranslations('securitySettings');
  const { user, loading: authLoading } = useAuth();

  if (authLoading) {
    return (
      <div className='w-full px-1 py-6'>
        <LoadingStateWithBorder
          icon={SlidersHorizontal}
          description={t('loadingAuth')}
        />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='w-full px-1 py-6'>
        <EmptyStateWithBorder
          icon={SlidersHorizontal}
          title={t('forbiddenTitle')}
          description={t('forbiddenDesc')}
        />
      </div>
    );
  }

  return (
    <div className='flex w-full flex-col gap-6 px-1 py-6'>
      <div className='flex flex-col gap-1.5'>
        <div className='flex items-center gap-2'>
          <SlidersHorizontal className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <p className='text-sm text-muted-foreground'>{t('subtitle')}</p>
      </div>

      <RateLimitConfigCard />
    </div>
  );
}
