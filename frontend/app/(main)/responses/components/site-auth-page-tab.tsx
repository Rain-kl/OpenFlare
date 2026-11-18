'use client';

import Link from 'next/link';
import { Expand, Pencil } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

import { SITE_AUTH_HTML_KEY, previewSiteAuthHTML } from './site-auth-page-html';

export function SiteAuthPageTab({
  optionMap,
}: {
  optionMap: Record<string, string>;
}) {
  const t = useTranslations('responses');
  const html = optionMap[SITE_AUTH_HTML_KEY] ?? '';

  return (
    <div className='space-y-6'>
      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-start justify-between gap-4 space-y-0'>
          <div className='space-y-1.5'>
            <CardTitle className='text-base'>{t('siteAuthSettings')}</CardTitle>
            <CardDescription>{t('siteAuthPageDescription')}</CardDescription>
          </div>
          <Button variant='outline' size='sm' className='shrink-0' asChild>
            <Link href='/proxy-routes'>{t('siteAuthConfigureSites')}</Link>
          </Button>
        </CardHeader>
        <CardContent>
          <p className='text-sm font-medium'>{t('siteAuthCurrentTemplate')}</p>
          <p className='text-sm text-muted-foreground'>
            {html.trim() ? t('siteAuthCustom') : t('siteAuthDefault')}
          </p>
        </CardContent>
      </Card>

      <Card className='overflow-hidden border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-start justify-between gap-3 space-y-0'>
          <CardTitle className='text-base'>{t('pagePreview')}</CardTitle>
          <div className='flex shrink-0 flex-wrap gap-2'>
            <Button variant='outline' size='sm' asChild>
              <Link href='/responses/site-auth/preview'>
                <Expand className='size-3.5' />
                {t('preview')}
              </Link>
            </Button>
            <Button size='sm' asChild>
              <Link href='/responses/site-auth/edit'>
                <Pencil className='size-3.5' />
                {t('edit')}
              </Link>
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className='relative overflow-hidden rounded-md border bg-muted/30'>
            <iframe
              title={t('siteAuthPreview')}
              sandbox=''
              srcDoc={previewSiteAuthHTML(html)}
              tabIndex={-1}
              className='block h-[32rem] w-full bg-background'
            />
            <Link
              href='/responses/site-auth/edit'
              aria-label={t('siteAuthEdit')}
              className='absolute inset-0 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring'
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
