'use client';

import { useTranslations } from 'next-intl';
import { LegalPageLayout } from '@/components/common/docs/legal-page-layout';
import { createPrivacySections } from '@/components/common/docs/privacy';
import { TERMS_LAST_UPDATED } from '@/components/common/docs/terms';

export default function PrivacyPolicyPage() {
  const rt = useTranslations('runtime');

  return (
    <LegalPageLayout
      title={rt('privacy_policy_b6da1ca6')}
      lastUpdated={TERMS_LAST_UPDATED}
      sections={createPrivacySections(rt)}
      description={
        <span>
          {rt('we_value_your_privacy_this_policy_explains_2c9e3c0b')}
          <br className='hidden md:block' />
          {rt('and_the_rights_you_have_over_it_4560abf2')}
        </span>
      }
    />
  );
}
