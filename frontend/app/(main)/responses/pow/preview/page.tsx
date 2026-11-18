'use client';

import { useTranslations } from 'next-intl';

import { POW_PAGE_KEY, readPoWPage } from '@/lib/openflare/pow-page';

import { ResponsePageFullPreview } from '../../components/response-page-full-preview';
import { ResponsePageOption } from '../../components/response-page-option';
import { usePoWPagePreview } from '../../components/use-pow-page-preview';

export default function PoWPagePreviewPage() {
  const t = useTranslations('pow');
  const preview = usePoWPagePreview();

  return (
    <ResponsePageOption optionKey={POW_PAGE_KEY}>
      {(value) => (
        <ResponsePageFullPreview
          title={t('pagePreview')}
          backPath='/responses?tab=pow'
          backLabel={t('backToPage')}
          editPath='/responses/pow/edit'
          html={preview(readPoWPage(value))}
          sandbox='allow-scripts'
        />
      )}
    </ResponsePageOption>
  );
}
