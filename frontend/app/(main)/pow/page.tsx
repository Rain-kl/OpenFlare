'use client';

import Link from 'next/link';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ShieldCheck } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useAuth } from '@/components/providers/auth-provider';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import {
  defaultPoWConfig,
  PoWService,
  ProxyRouteService,
} from '@/lib/services/openflare';
import { PolicyEditor } from './components/policy-editor';

export default function PoWPage() {
  const t = useTranslations('pow');
  const { user, loading } = useAuth();
  const [selectedID, setSelectedID] = useState('');
  const query = useQuery({
    queryKey: ['openflare', 'pow'],
    queryFn: async () => {
      const [sites, policies] = await Promise.all([
        ProxyRouteService.list(),
        PoWService.list(),
      ]);
      return { sites, policies };
    },
    enabled: !!user?.is_admin,
  });
  if (loading || query.isLoading)
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder icon={ShieldCheck} description={t('loading')} />
      </div>
    );
  if (!user?.is_admin)
    return (
      <div className='w-full py-6 px-1'>
        <EmptyStateWithBorder
          icon={ShieldCheck}
          title={t('forbidden')}
          description={t('forbiddenHelp')}
        />
      </div>
    );
  if (query.isError)
    return (
      <div className='w-full py-6 px-1'>
        <ErrorInline
          message={t('loadFailed')}
          onRetry={() => void query.refetch()}
        />
      </div>
    );
  const sites = query.data?.sites ?? [];
  const routeID = Number(selectedID || sites[0]?.id || 0);
  const storedPolicy = query.data?.policies.find(
    (item) => item.route_id === routeID,
  );
  const policy = storedPolicy ?? {
    route_id: routeID,
    enabled: false,
    config: defaultPoWConfig,
  };
  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <ShieldCheck className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <Button variant='outline' asChild>
          <Link href='/responses?tab=pow'>{t('pageSettings')}</Link>
        </Button>
      </div>
      <p className='text-sm text-muted-foreground'>{t('description')}</p>
      {sites.length ? (
        <>
          <Field>
            <FieldLabel htmlFor='pow-site'>{t('site')}</FieldLabel>
            <Select value={String(routeID)} onValueChange={setSelectedID}>
              <SelectTrigger id='pow-site' className='w-full md:w-96'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {sites.map((site) => (
                    <SelectItem key={site.id} value={String(site.id)}>
                      {site.site_name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <PolicyEditor
            key={routeID}
            policy={policy}
            configured={!!storedPolicy}
          />
        </>
      ) : (
        <EmptyStateWithBorder
          icon={ShieldCheck}
          title={t('noSites')}
          description={t('noSitesHelp')}
        />
      )}
    </div>
  );
}
