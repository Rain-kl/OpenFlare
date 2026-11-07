'use client';

import { useTranslations } from 'next-intl';
import { LegalPageLayout } from '@/components/common/docs/legal-page-layout';
import { createHowToUseSections } from '@/components/common/docs/how-to-use';
import { DOCS_LAST_UPDATED } from '@/components/common/docs/api';

export default function HowToUsePage() {
  const rt = useTranslations('runtime');

  return (
    <LegalPageLayout
      title={rt('platform_user_guide_d0ead991')}
      lastUpdated={DOCS_LAST_UPDATED}
      sections={createHowToUseSections(rt)}
      description={
        <p className='text-muted-foreground text-sm leading-relaxed'>
          {rt('welcome_to_the_openflare_management_platform_this_e8ecbe4d')}
        </p>
      }
    />
  );
}
