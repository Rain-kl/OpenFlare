'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Plus, RefreshCw, Route, Trash2 } from 'lucide-react';
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
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { ProxyRouteItem } from '@/lib/services/openflare';
import { ProxyRouteService } from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import {
  getRouteDomainNames,
  getRouteDomainsLabel,
  getRoutePrimaryDomain,
  getUpstreamSummary,
} from './components/helpers';
import { ProxyRouteCreateSheet } from './components/proxy-route-create-sheet';

export function ProxyRoutesPageClient() {
  const t = useTranslations('proxyRoutes');
  const tc = useTranslations('common');
  const router = useRouter();
  const [routes, setRoutes] = useState<ProxyRouteItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [keyword, setKeyword] = useState('');
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ProxyRouteItem | null>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchRoutes = useCallback(async () => {
    setLoading(true);
    try {
      const data = await ProxyRouteService.list();
      setRoutes(data);
    } catch (error) {
      toast.error(t('loadListFailed'), {
        description: error instanceof Error ? error.message : t('unknownError'),
      });
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void fetchRoutes();
  }, [fetchRoutes]);

  const filteredRoutes = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    if (!normalizedKeyword) {
      return routes;
    }

    return routes.filter((route) => {
      const haystack = [
        route.site_name,
        getRoutePrimaryDomain(route),
        ...getRouteDomainNames(route),
        route.origin_url,
      ]
        .join(' ')
        .toLowerCase();

      return haystack.includes(normalizedKeyword);
    });
  }, [keyword, routes]);

  const handleDelete = async () => {
    if (!deleteTarget) {
      return;
    }

    setDeleting(true);
    try {
      await ProxyRouteService.deleteById(deleteTarget.id);
      toast.success(t('siteDeleted'));
      setDeleteTarget(null);
      await fetchRoutes();
    } catch (error) {
      toast.error(t('deleteFailed'), {
        description: error instanceof Error ? error.message : t('unknownError'),
      });
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex items-center gap-2'>
          <Route className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button
            size='sm'
            variant='secondary'
            className='h-8 gap-1.5 text-xs'
            onClick={() => void fetchRoutes()}
            disabled={loading}
          >
            <RefreshCw className={`size-3 ${loading ? 'animate-spin' : ''}`} />
            {t('refresh')}
          </Button>
          <Button
            size='sm'
            className='h-8 gap-1.5 text-xs'
            onClick={() => setIsCreateOpen(true)}
          >
            <Plus className='size-3.5' />
            {t('createRule')}
          </Button>
        </div>
      </div>

      <Card className='border-border/40 shadow-sm'>
        <CardHeader className='pb-3'>
          <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <CardTitle className='text-sm font-semibold'>
              {t('listTitle')}
            </CardTitle>
            <Input
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              placeholder={t('searchPlaceholder')}
              className='h-8 max-w-sm text-xs'
            />
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className='space-y-2'>
              {Array.from({ length: 5 }).map((_, index) => (
                <Skeleton key={index} className='h-10 w-full' />
              ))}
            </div>
          ) : filteredRoutes.length === 0 ? (
            <div className='flex flex-col items-center justify-center gap-2 py-12 text-center'>
              <p className='text-sm font-medium'>
                {routes.length === 0 ? t('emptyTitle') : t('noMatches')}
              </p>
              <p className='text-xs text-muted-foreground'>
                {routes.length === 0 ? t('emptyDesc') : t('noMatchesDesc')}
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('siteName')}</TableHead>
                  <TableHead>{t('domain')}</TableHead>
                  <TableHead>{t('status')}</TableHead>
                  <TableHead>{t('upstream')}</TableHead>
                  <TableHead className='text-right'>{t('actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredRoutes.map((route) => (
                  <TableRow key={route.id}>
                    <TableCell className='font-medium'>
                      {route.site_name}
                    </TableCell>
                    <TableCell
                      className='max-w-[220px] truncate'
                      title={getRouteDomainsLabel(route, t)}
                    >
                      {getRoutePrimaryDomain(route) ||
                        getRouteDomainsLabel(route, t)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={route.enabled ? 'default' : 'secondary'}>
                        {route.enabled ? t('enabled') : t('disabled')}
                      </Badge>
                    </TableCell>
                    <TableCell
                      className='max-w-[280px] truncate'
                      title={getUpstreamSummary(route, t)}
                    >
                      {getUpstreamSummary(route, t)}
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex justify-end gap-2'>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs'
                          asChild
                        >
                          <Link href={`/proxy-routes/detail?id=${route.id}`}>
                            {t('configure')}
                          </Link>
                        </Button>
                        <Button
                          variant='ghost'
                          size='sm'
                          className='h-7 text-xs text-destructive hover:text-destructive'
                          onClick={() => setDeleteTarget(route)}
                        >
                          <Trash2 className='size-3.5' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <ProxyRouteCreateSheet
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
        onCreated={(route) => {
          toast.success(t('siteCreated'));
          void fetchRoutes();
          router.push(`/proxy-routes/detail?id=${route.id}&section=domains`);
        }}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteSiteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget
                ? t('deleteSiteDesc', { name: deleteTarget.site_name })
                : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {tc('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={deleting}
              onClick={(event) => {
                event.preventDefault();
                void handleDelete();
              }}
            >
              {deleting ? t('deleting') : tc('delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
