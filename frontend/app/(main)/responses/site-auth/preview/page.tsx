'use client';

import Link from 'next/link';
import { ArrowLeft, Pencil } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';

import { previewSiteAuthHTML } from '../../components/site-auth-page-html';
import { SiteAuthPageContent } from '../site-auth-page-content';

export default function SiteAuthPagePreviewPage() {
  const t = useTranslations('responses');

  return (
    <SiteAuthPageContent>
      {(html) => (
        <div className='fixed inset-0 z-50 flex flex-col bg-background'>
          <div className='flex shrink-0 items-center justify-between gap-3 border-b px-4 py-2'>
            <div className='flex min-w-0 items-center gap-2'>
              <Button variant='outline' size='icon' className='size-8' asChild>
                <Link
                  href='/responses?tab=site-auth'
                  aria-label={t('siteAuthBack')}
                >
                  <ArrowLeft className='size-4' />
                </Link>
              </Button>
              <h1 className='truncate text-sm font-semibold'>
                {t('siteAuthPreview')}
              </h1>
            </div>
            <Button variant='outline' size='sm' asChild>
              <Link href='/responses/site-auth/edit'>
                <Pencil className='size-3.5' />
                {t('edit')}
              </Link>
            </Button>
          </div>
          <iframe
            title={t('siteAuthPreview')}
            sandbox=''
            srcDoc={previewSiteAuthHTML(html)}
            className='min-h-0 w-full flex-1 border-0 bg-background'
          />
        </div>
      )}
    </SiteAuthPageContent>
  );
}
