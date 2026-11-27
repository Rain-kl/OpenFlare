'use client';

import { Settings } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useRouter } from 'next/navigation';
import { useEffect } from 'react';

import { LoadingStateWithBorder } from '@/components/layout/loading';

export default function CloudflareSettingsRedirectPage() {
  const router = useRouter();
  const t = useTranslations('cloudflare.settings');

  useEffect(() => {
    router.replace('/dns/settings');
  }, [router]);

  return (
    <div className='w-full py-6 px-1'>
      <LoadingStateWithBorder icon={Settings} description={t('redirecting')} />
    </div>
  );
}
