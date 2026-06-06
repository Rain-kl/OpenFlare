import {Suspense} from 'react';

import {LoadingState} from '@/components/feedback/loading-state';
import {OAuthCallback} from '@/features/auth/components/oauth-callback';

export default function OAuthSourceCallbackPage() {
  return (
    <Suspense fallback={<LoadingState />}>
      <OAuthCallback />
    </Suspense>
  );
}
