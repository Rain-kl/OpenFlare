import { Cloud } from 'lucide-react';
import { getTranslations } from 'next-intl/server';
import { Suspense } from 'react';

import { LoadingStateWithBorder } from '@/components/layout/loading';

import { CloudflareGroupDetailPageClient } from './page-client';

/** Placeholder for static export; real ids resolve client-side via useParams. */
export async function generateStaticParams() {
  return [{ id: '1' }];
}

export default async function CloudflareGroupDetailPage() {
  const t = await getTranslations('cloudflare');
  return (
    <Suspense
      fallback={
        <div className='w-full py-6 px-1'>
          <LoadingStateWithBorder
            icon={Cloud}
            description={t('loadingDetail')}
          />
        </div>
      }
    >
      <CloudflareGroupDetailPageClient />
    </Suspense>
  );
}
