'use client';

import { useRouter } from 'next/navigation';
import { useEffect } from 'react';

import { LoadingState } from '@/components/feedback/loading-state';

interface LegacyRouteRedirectProps {
  href: string;
}

export function LegacyRouteRedirect({ href }: LegacyRouteRedirectProps) {
  const router = useRouter();

  useEffect(() => {
    router.replace(href);
  }, [href, router]);

  return (
    <main className='flex min-h-screen items-center justify-center px-4 py-8'>
      <LoadingState className='w-full max-w-md' />
    </main>
  );
}
