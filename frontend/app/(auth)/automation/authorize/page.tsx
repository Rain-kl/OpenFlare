import { Suspense } from 'react';
import type { Metadata } from 'next';
import { AutomationConsentPage } from '@/components/auth/automation-consent-page';

export const metadata: Metadata = { referrer: 'same-origin' };

export default function Page() {
  return (
    <Suspense>
      <AutomationConsentPage />
    </Suspense>
  );
}
