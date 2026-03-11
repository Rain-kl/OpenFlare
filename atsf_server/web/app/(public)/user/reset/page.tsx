import { Suspense } from 'react';

import { LoadingState } from '@/components/feedback/loading-state';
import { PasswordResetConfirmForm } from '@/features/auth/components/password-reset-confirm-form';

export default function LegacyResetPasswordPage() {
  return (
    <Suspense fallback={<LoadingState />}>
      <PasswordResetConfirmForm />
    </Suspense>
  );
}
