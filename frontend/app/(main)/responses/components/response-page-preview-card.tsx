'use client';

import Link from 'next/link';
import { Expand, Pencil } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

export function ResponsePagePreviewCard({
  title,
  editLabel,
  previewPath,
  editPath,
  html,
  sandbox = '',
}: {
  title: string;
  editLabel: string;
  previewPath: string;
  editPath: string;
  html: string;
  sandbox?: string;
}) {
  const t = useTranslations('responses');
  return (
    <Card className='overflow-hidden border-dashed shadow-none'>
      <CardHeader className='flex flex-row items-start justify-between gap-3 space-y-0'>
        <CardTitle className='text-base'>{t('pagePreview')}</CardTitle>
        <div className='flex shrink-0 flex-wrap gap-2'>
          <Button variant='outline' size='sm' asChild>
            <Link href={previewPath}>
              <Expand className='size-3.5' />
              {t('preview')}
            </Link>
          </Button>
          <Button size='sm' asChild>
            <Link href={editPath}>
              <Pencil className='size-3.5' />
              {t('edit')}
            </Link>
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className='relative overflow-hidden rounded-md border bg-muted/30'>
          <iframe
            title={title}
            sandbox={sandbox}
            srcDoc={html}
            tabIndex={-1}
            className='block h-[32rem] w-full bg-background'
          />
          <Link
            href={editPath}
            aria-label={editLabel}
            className='absolute inset-0 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring'
          />
        </div>
      </CardContent>
    </Card>
  );
}
