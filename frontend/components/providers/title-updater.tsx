'use client';

import { useQuery } from '@tanstack/react-query';
import { useEffect } from 'react';
import { ConfigService } from '@/lib/services/config';
import { usePathname } from 'next/navigation';
import { useTranslations } from 'next-intl';

export function SiteTitleUpdater() {
  const t = useTranslations('providers.titleUpdater');
  const pathname = usePathname();
  const publicConfigQuery = useQuery({
    queryKey: ['public-config'],
    queryFn: () => ConfigService.getPublicConfig(),
  });

  useEffect(() => {
    const siteName = publicConfigQuery.data?.site_name || 'OpenFlare';

    // Determine the page suffix based on path
    let suffix = '';
    if (pathname === '/login') {
      suffix = ` - ${t('login')}`;
    } else if (pathname === '/register') {
      suffix = ` - ${t('register')}`;
    } else if (pathname.startsWith('/admin')) {
      suffix = ` - ${t('admin')}`;
    } else if (pathname === '/') {
      suffix = ` - ${t('dashboard')}`;
    }

    document.title = `${siteName}${suffix}`;
  }, [publicConfigQuery.data?.site_name, pathname, t]);

  return null;
}
