'use client';

import Link from 'next/link';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'next/navigation';
import { ArrowLeft, MapPin, Trash2 } from 'lucide-react';
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
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { OriginService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { useTranslations } from 'next-intl';

import { OriginEditorDialog } from '../components/origin-editor-dialog';

export function OriginDetailPageClient() {
  const t = useTranslations('origins');
  const tc = useTranslations('common');
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const originId = searchParams.get('id')?.trim() ?? '';
  const parsedId = Number(originId);
  const enabled = originId !== '' && Number.isFinite(parsedId);

  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const originQuery = useQuery({
    queryKey: ['openflare', 'origins', originId],
    queryFn: () => OriginService.getById(parsedId),
    enabled,
  });

  const deleteMutation = useMutation({
    mutationFn: () => OriginService.deleteById(parsedId),
    onSuccess: async () => {
      toast.success(t('deleted'));
      await queryClient.invalidateQueries({
        queryKey: ['openflare', 'origins'],
      });
      window.location.href = '/origins';
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('deleteFailed'));
    },
  });

  const origin = originQuery.data;

  if (!enabled) {
    return (
      <div className='py-6 px-1'>
        <EmptyStateWithBorder description={t('missingId')} />
      </div>
    );
  }

  if (originQuery.isLoading) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder
          icon={MapPin}
          description={t('loadingDetail')}
        />
      </div>
    );
  }

  if (originQuery.isError) {
    return (
      <div className='py-6 px-1'>
        <ErrorInline
          message={
            originQuery.error instanceof Error
              ? originQuery.error.message
              : t('loadFailed')
          }
          onRetry={() => void originQuery.refetch()}
        />
      </div>
    );
  }

  if (!origin) {
    return (
      <div className='py-6 px-1 space-y-4'>
        <Button variant='ghost' size='sm' asChild>
          <Link href='/origins'>
            <ArrowLeft className='size-4 mr-1' />
            {t('backToList')}
          </Link>
        </Button>
        <EmptyStateWithBorder description={t('notFound')} />
      </div>
    );
  }

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between'>
        <div className='space-y-2'>
          <Button variant='ghost' size='sm' className='h-8 px-2 -ml-2' asChild>
            <Link href='/origins'>
              <ArrowLeft className='size-4 mr-1' />
              {t('backToList')}
            </Link>
          </Button>
          <div className='flex items-center gap-2'>
            <MapPin className='size-5 text-primary' />
            <h1 className='text-2xl font-semibold tracking-tight'>
              {origin.name}
            </h1>
          </div>
          <p className='text-sm text-muted-foreground'>{origin.address}</p>
        </div>
        <div className='flex gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setEditorOpen(true)}
          >
            {t('editOrigin')}
          </Button>
          <Button
            variant='destructive'
            size='sm'
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className='size-3.5 mr-1' />
            {t('deleteOrigin')}
          </Button>
        </div>
      </div>

      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <div className='rounded-lg border border-dashed px-4 py-3'>
          <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
            {t('boundRoutes')}
          </p>
          <Badge variant='outline' className='mt-2 text-[10px]'>
            {t('routeCount', { count: origin.route_count })}
          </Badge>
        </div>
        <div className='rounded-lg border border-dashed px-4 py-3'>
          <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
            {t('createdAt')}
          </p>
          <p className='mt-2 text-sm'>{formatDateTime(origin.created_at)}</p>
        </div>
        <div className='rounded-lg border border-dashed px-4 py-3'>
          <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
            {t('updatedAt')}
          </p>
          <p className='mt-2 text-sm'>{formatDateTime(origin.updated_at)}</p>
        </div>
        <div className='rounded-lg border border-dashed px-4 py-3 sm:col-span-2 xl:col-span-1'>
          <p className='text-[10px] uppercase tracking-wider text-muted-foreground'>
            {t('remark')}
          </p>
          <p className='mt-2 text-sm text-muted-foreground'>
            {origin.remark || t('noRemark')}
          </p>
        </div>
      </div>

      <div className='border border-dashed rounded-lg overflow-hidden bg-background'>
        <div className='px-4 py-3 border-b border-dashed'>
          <h2 className='text-sm font-semibold'>{t('relatedRoutes')}</h2>
          <p className='text-xs text-muted-foreground mt-1'>
            {t('relatedRoutesDesc')}
          </p>
        </div>
        {origin.routes.length === 0 ? (
          <EmptyStateWithBorder
            title={t('noRelatedRoutes')}
            description={t('noRelatedRoutesDesc')}
          />
        ) : (
          <Table>
            <TableHeader className='bg-muted/40'>
              <TableRow className='border-dashed hover:bg-transparent'>
                <TableHead className='text-xs font-semibold'>
                  {t('domain')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('address')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('status')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('updatedAt')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {origin.routes.map((route) => (
                <TableRow key={route.id} className='border-dashed'>
                  <TableCell className='text-xs font-medium'>
                    {route.domain}
                  </TableCell>
                  <TableCell className='text-xs text-muted-foreground'>
                    {route.origin_url}
                  </TableCell>
                  <TableCell>
                    <Badge variant='outline' className='text-[10px]'>
                      {route.enabled ? t('enabled') : t('disabled')}
                    </Badge>
                  </TableCell>
                  <TableCell className='text-xs text-muted-foreground'>
                    {formatDateTime(route.updated_at)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <OriginEditorDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        origin={origin}
        onSaved={() => void originQuery.refetch()}
      />

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: origin.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              onClick={() => deleteMutation.mutate()}
            >
              {t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
