'use client';

import { useTranslations } from 'next-intl';
import { LegalPageLayout } from '@/components/common/docs/legal-page-layout';
import {
  createApiSections,
  DOCS_LAST_UPDATED,
} from '@/components/common/docs/api';

export default function ApiDocPage() {
  const rt = useTranslations('runtime');

  return (
    <LegalPageLayout
      title={rt('api_documentation_cb3d2229')}
      lastUpdated={DOCS_LAST_UPDATED}
      sections={createApiSections(rt)}
      description={
        <p className='text-muted-foreground text-sm leading-relaxed'>
          {rt('openflare_provides_a_simple_powerful_api_for_ba400468')}
        </p>
      }
    />
  );
}
