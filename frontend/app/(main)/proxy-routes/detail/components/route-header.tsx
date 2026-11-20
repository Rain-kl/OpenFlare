'use client';

import { ArrowLeft, Route } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useRouter } from 'next/navigation';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { ProxyRouteItem } from '@/lib/services/openflare';

interface RouteHeaderProps {
  route: ProxyRouteItem;
}

export function RouteHeader({ route }: RouteHeaderProps) {
  const t = useTranslations('proxyRoutes');
  const router = useRouter();

  return (
    <div className='space-y-4'>
      <Button
        variant='ghost'
        size='sm'
        className='h-8 gap-1.5 px-0 text-xs'
        onClick={() => router.back()}
      >
        <ArrowLeft className='size-3.5' />
        {t('back')}
      </Button>
      <div className='flex flex-wrap items-center gap-2'>
        <Route className='size-5 text-primary' />
        <h1 className='text-2xl font-semibold tracking-tight'>
          {route.site_name}
        </h1>
        <Badge variant={route.enabled ? 'default' : 'secondary'}>
          {route.enabled ? t('enabled') : t('disabled')}
        </Badge>
      </div>
    </div>
  );
}
