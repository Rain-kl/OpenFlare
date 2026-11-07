'use client';

import { useTranslations } from 'next-intl';
import type { LucideIcon } from 'lucide-react';

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

interface PlaceholderPageProps {
  title: string;
  icon: LucideIcon;
}

export function PlaceholderPage({ title, icon: Icon }: PlaceholderPageProps) {
  const rt = useTranslations('runtime');

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex items-center gap-2'>
        <Icon className='size-5 text-primary' />
        <h1 className='text-2xl font-semibold tracking-tight'>{title}</h1>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className='text-base'>
            {rt('under_development_78c33fbf')}
          </CardTitle>
          <CardDescription>
            {rt('this_feature_is_being_migrated_and_developed_61ebaf78')}
          </CardDescription>
        </CardHeader>
      </Card>
    </div>
  );
}
