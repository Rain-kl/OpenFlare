import { Suspense } from 'react';

import { LoadingState } from '@/components/feedback/loading-state';
import { PasswordResetRequestForm } from '@/features/auth/components/password-reset-request-form';

export default function ResetPasswordPage() {
  return (
    <Suspense fallback={<LoadingState />}>
      <PasswordResetRequestForm />
    </Suspense>
  );
}
