'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Globe, Pencil, Trash2 } from 'lucide-react';
import { toast } from 'sonner';

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  ProxyRouteService,
  TlsCertificateService,
  ZoneService,
  zoneQueryKey,
} from '@/lib/services/openflare';
import { useTranslations } from 'next-intl';

import { getErrorMessage } from '../components/website-utils';
import { ZoneCertificatesPanel } from './components/zone-certificates';
import { ZoneDomainsTable } from './components/zone-domains-table';
import { ZoneEditorDialog } from './components/zone-editor-dialog';
import { ZoneOverviewPanel } from './components/zone-overview';

const zoneTabs = ['overview', 'domains', 'certificates', 'settings'] as const;
export type ZonePageTab = (typeof zoneTabs)[number];

function getZonePageTab(value: string | null | undefined): ZonePageTab {
  return zoneTabs.includes(value as ZonePageTab)
    ? (value as ZonePageTab)
    : 'overview';
}

function getZoneIdFromPathname(pathname: string | null): number {
  const match = pathname?.match(/^\/websites\/([^/]+)$/);
  return Number(match?.[1]);
}

export function ZonePageClient() {
  const t = useTranslations('websites');
  const tc = useTranslations('common');
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);

  const router = useRouter();
  const pathname = usePathname();
  const zoneId = useMemo(() => getZoneIdFromPathname(pathname), [pathname]);
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const activeTab = useMemo(
    () => getZonePageTab(searchParams.get('tab')),
    [searchParams],
  );
  const [editZone, setEditZone] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const setActiveTab = useCallback(
    (tab: string) => {
      const next = getZonePageTab(tab);
      const params = new URLSearchParams(searchParams.toString());
      if (next === 'overview') {
        params.delete('tab');
      } else {
        params.set('tab', next);
      }
      const query = params.toString();
      router.replace(query ? `${pathname}?${query}` : pathname, {
        scroll: false,
      });
    },
    [pathname, router, searchParams],
  );

  const overviewQuery = useQuery({
    queryKey: [...zoneQueryKey, zoneId],
    queryFn: () => ZoneService.getOverview(zoneId),
    enabled: Number.isInteger(zoneId) && zoneId > 0,
  });

  const certificatesQuery = useQuery({
    queryKey: ['openflare', 'tls-certificates'],
    queryFn: () => TlsCertificateService.list(),
    enabled: overviewQuery.isSuccess,
  });

  const routesQuery = useQuery({
    queryKey: ['openflare', 'proxy-routes'],
    queryFn: () => ProxyRouteService.list(),
    enabled: overviewQuery.isSuccess,
  });

  const deleteZone = useMutation({
    mutationFn: () => ZoneService.deleteById(zoneId),
    onSuccess: async () => {
      toast.success(t('zoneDeleted'));
      await queryClient.invalidateQueries({ queryKey: zoneQueryKey });
      window.location.assign('/websites');
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  if (!mounted) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder icon={Globe} description={t('loadingDetail')} />
      </div>
    );
  }

  if (!Number.isInteger(zoneId) || zoneId <= 0) {
    return (
      <div className='py-6 px-1'>
        <EmptyStateWithBorder icon={Globe} description={t('invalidId')} />
      </div>
    );
  }

  if (overviewQuery.isLoading) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder icon={Globe} description={t('loadingDetail')} />
      </div>
    );
  }

  if (overviewQuery.isError || !overviewQuery.data) {
    return (
      <div className='space-y-4 py-6 px-1'>
        <Button
          variant='ghost'
          size='sm'
          className='h-8 gap-1.5 px-0 text-xs'
          onClick={() => router.back()}
        >
          <ArrowLeft className='size-3.5' />
          {t('back')}
        </Button>
        <EmptyStateWithBorder icon={Globe} description={t('notFound')} />
      </div>
    );
  }

  const overview = overviewQuery.data;
  const certificates = certificatesQuery.data ?? [];
  const routes = routesQuery.data ?? [];
  const boundCertCount = new Set(
    overview.domains
      .map((domain) => domain.cert_id)
      .filter((id): id is number => id != null),
  ).size;

  return (
    <div className='space-y-6 py-6 px-1'>
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

        <div className='flex items-center gap-2'>
          <Globe className='size-5 text-primary' />
          <div>
            <h1 className='text-2xl font-semibold tracking-tight'>
              {overview.zone.domain}
            </h1>
            <p className='text-sm text-muted-foreground'>
              {t('detailSubtitle')}
            </p>
          </div>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className='w-full'>
        <TabsList variant='line' className='mb-6 inline-flex w-fit gap-8'>
          <TabsTrigger
            value='overview'
            className='px-0 pb-2 text-xs font-semibold'
          >
            {t('tabOverview')}
          </TabsTrigger>
          <TabsTrigger
            value='domains'
            className='px-0 pb-2 text-xs font-semibold'
          >
            {t('tabDomains', { count: overview.domains.length })}
          </TabsTrigger>
          <TabsTrigger
            value='certificates'
            className='px-0 pb-2 text-xs font-semibold'
          >
            {t('tabCertificates', { count: boundCertCount })}
          </TabsTrigger>
          <TabsTrigger
            value='settings'
            className='px-0 pb-2 text-xs font-semibold'
          >
            {tc('settings')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value='overview'>
          <ZoneOverviewPanel overview={overview} zoneId={zoneId} />
        </TabsContent>
        <TabsContent value='domains'>
          <ZoneDomainsTable
            zoneId={zoneId}
            zoneRoot={overview.zone.domain}
            domains={overview.domains}
            certificates={certificates}
            routes={routes}
            routesLoading={routesQuery.isLoading}
            onChanged={() => overviewQuery.refetch()}
          />
        </TabsContent>
        <TabsContent value='certificates'>
          <ZoneCertificatesPanel
            domains={overview.domains}
            certificates={certificates}
          />
        </TabsContent>
        <TabsContent value='settings'>
          <div className='space-y-4'>
            <div className='rounded-lg border p-4'>
              <p className='text-sm font-semibold'>{t('basicInfo')}</p>
              <p className='mt-1 text-sm text-muted-foreground'>
                {t('basicInfoDesc')}
              </p>
              <div className='mt-3 grid gap-2 text-sm sm:grid-cols-2'>
                <div>
                  <p className='text-xs text-muted-foreground'>
                    {t('currentRoot')}
                  </p>
                  <p className='mt-1 font-mono text-[13px] font-medium'>
                    {overview.zone.domain}
                  </p>
                </div>
                <div>
                  <p className='text-xs text-muted-foreground'>
                    {t('domainCount')}
                  </p>
                  <p className='mt-1 font-mono text-[13px] font-medium'>
                    {overview.domains.length}
                  </p>
                </div>
              </div>
              <Button
                variant='outline'
                size='sm'
                className='mt-4 h-7 text-xs'
                onClick={() => setEditZone(true)}
              >
                <Pencil className='mr-1 size-3.5' />
                {t('editRoot')}
              </Button>
            </div>

            <div className='rounded-lg border border-destructive/30 p-4'>
              <p className='text-sm font-semibold text-destructive'>
                {t('dangerZone')}
              </p>
              <p className='mt-1 text-sm text-muted-foreground'>
                {t('dangerZoneDesc')}
              </p>
              <Button
                variant='destructive'
                size='sm'
                className='mt-4 h-7 text-xs'
                onClick={() => setConfirmDelete(true)}
              >
                <Trash2 className='mr-1 size-3.5' />
                {t('deleteZone')}
              </Button>
            </div>
          </div>
        </TabsContent>
      </Tabs>

      <ZoneEditorDialog
        open={editZone}
        onOpenChange={setEditZone}
        zone={overview.zone}
      />

      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteZoneTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteZoneDesc', { domain: overview.zone.domain })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteZone.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={deleteZone.isPending}
              onClick={(event) => {
                event.preventDefault();
                deleteZone.mutate();
              }}
            >
              {deleteZone.isPending ? t('deleting') : t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
