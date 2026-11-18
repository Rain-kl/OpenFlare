'use client';

import Link from 'next/link';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

import { ResponsePagePreviewCard } from './response-page-preview-card';
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

      <ResponsePagePreviewCard
        title={t('siteAuthPreview')}
        editLabel={t('siteAuthEdit')}
        previewPath='/responses/site-auth/preview'
        editPath='/responses/site-auth/edit'
        html={previewSiteAuthHTML(html)}
      />
    </div>
  );
}
