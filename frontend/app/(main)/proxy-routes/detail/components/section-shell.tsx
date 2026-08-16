'use client';

import type { ReactNode } from 'react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

interface SectionShellProps {
  title: string;
  description: string;
  titleExtra?: ReactNode;
  formId: string;
  saving?: boolean;
  children: ReactNode;
}

export function SectionShell({
  title,
  description,
  titleExtra,
  formId,
  saving = false,
  children,
}: SectionShellProps) {
  const t = useTranslations('proxyRoutes');
  const tc = useTranslations('common');
  return (
    <Card>
      <CardHeader className='flex flex-row items-start justify-between gap-4 space-y-0'>
        <div className='space-y-1'>
          <div className='flex items-center gap-1.5'>
            <CardTitle className='text-sm font-semibold'>{title}</CardTitle>
            {titleExtra}
          </div>
          <CardDescription>{description}</CardDescription>
        </div>
        <Button
          type='submit'
          form={formId}
          size='sm'
          className='h-8 shrink-0 text-xs'
          disabled={saving}
        >
          {saving ? t('saving') : tc('save')}
        </Button>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}
