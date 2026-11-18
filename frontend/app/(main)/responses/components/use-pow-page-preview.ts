'use client';

import { useLocale, useTranslations } from 'next-intl';

import { previewPoWPage } from '@/lib/openflare/pow-page';
import type { PoWPageConfig } from '@/lib/services/openflare';

export function usePoWPagePreview() {
  const t = useTranslations('pow');
  const locale = useLocale();

  return (config: PoWPageConfig) =>
    previewPoWPage(config, locale, {
      title: t('previewTitle'),
      status: t('previewStatus'),
      protected: t('previewProtected'),
      why: t('previewWhy'),
      why_description: t('previewWhyDescription'),
      noscript: t('previewNoscript'),
      success: t('previewSuccess'),
    });
}
