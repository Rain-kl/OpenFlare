'use client';

import { useTranslations } from 'next-intl';
import { LegalPageLayout } from '@/components/common/docs/legal-page-layout';
import {
  TERMS_LAST_UPDATED,
  createTermsSections,
} from '@/components/common/docs/terms';

export default function TermsOfServicePage() {
  const rt = useTranslations('runtime');

  return (
    <LegalPageLayout
      title={rt('terms_of_service_4e80a140')}
      lastUpdated={TERMS_LAST_UPDATED}
      sections={createTermsSections(rt)}
      description={
        <span>
          {rt('please_read_the_following_terms_carefully_to_c46a6a61')}
        </span>
      }
    />
  );
}
