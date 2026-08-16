'use client';

import Link from 'next/link';
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { MapPin, Plus, RefreshCw, Trash2 } from 'lucide-react';
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
import { type OriginItem, OriginService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { useTranslations } from 'next-intl';

import { OriginEditorDialog } from './components/origin-editor-dialog';

const originsQueryKey = ['openflare', 'origins'] as const;

export default function OriginsPage() {
  const t = useTranslations('origins');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingOrigin, setEditingOrigin] = useState<OriginItem | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<OriginItem | null>(null);

  const originsQuery = useQuery({
    queryKey: originsQueryKey,
    queryFn: () => OriginService.list(),
  });

  const origins = useMemo(() => originsQuery.data ?? [], [originsQuery.data]);

  const deleteMutation = useMutation({
    mutationFn: (id: number) => OriginService.deleteById(id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      setDeleteTarget(null);
      await queryClient.invalidateQueries({ queryKey: originsQueryKey });
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('deleteFailed'));
    },
  });

  return (
    <div className='py-6 px-1 space-y-6'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex items-center gap-2'>
          <MapPin className='size-5 text-primary' />
          <div>
            <h1 className='text-2xl font-semibold tracking-tight'>
              {t('title')}
            </h1>
            <p className='text-sm text-muted-foreground'>{t('subtitle')}</p>
          </div>
        </div>
        <div className='flex gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void originsQuery.refetch()}
            disabled={originsQuery.isFetching}
          >
            <RefreshCw
              className={`size-3.5 mr-1 ${originsQuery.isFetching ? 'animate-spin' : ''}`}
            />
            {t('refresh')}
          </Button>
          <Button
            size='sm'
            onClick={() => {
              setEditingOrigin(null);
              setEditorOpen(true);
            }}
          >
            <Plus className='size-3.5 mr-1' />
            {t('create')}
          </Button>
        </div>
      </div>

      {originsQuery.isError ? (
        <ErrorInline
          message={
            originsQuery.error instanceof Error
              ? originsQuery.error.message
              : t('loadFailed')
          }
          onRetry={() => void originsQuery.refetch()}
        />
      ) : null}

      <div className='border border-dashed rounded-lg overflow-hidden bg-background'>
        {originsQuery.isLoading ? (
          <LoadingStateWithBorder />
        ) : origins.length === 0 ? (
          <EmptyStateWithBorder
            title={t('emptyTitle')}
            description={t('emptyDesc')}
          />
        ) : (
          <div className='grid gap-0 md:grid-cols-2'>
            {origins.map((origin) => (
              <article
                key={origin.id}
                className='border-b border-dashed p-4 md:[&:nth-child(odd)]:border-r'
              >
                <div className='flex flex-col gap-4'>
                  <div className='space-y-2'>
                    <div className='flex flex-wrap items-center gap-2'>
                      <h2 className='text-base font-semibold'>{origin.name}</h2>
                      <Badge
                        variant='outline'
                        className={`text-[10px] ${
                          origin.route_count > 0
                            ? 'text-emerald-600 border-emerald-500/20'
                            : 'text-amber-600 border-amber-500/20'
                        }`}
                      >
                        {t('routeCount', { count: origin.route_count })}
                      </Badge>
                    </div>
                    <p className='text-sm'>{origin.address}</p>
                    <p className='text-sm text-muted-foreground'>
                      {origin.remark || t('noRemark')}
                    </p>
                    <p className='text-xs text-muted-foreground'>
                      {t('lastUpdated', {
                        date: formatDateTime(origin.updated_at),
                      })}
                    </p>
                  </div>
                  <div className='flex flex-wrap gap-2'>
                    <Button variant='outline' size='sm' asChild>
                      <Link href={`/origins/detail?id=${origin.id}`}>
                        {t('detail')}
                      </Link>
                    </Button>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() => {
                        setEditingOrigin(origin);
                        setEditorOpen(true);
                      }}
                    >
                      {t('edit')}
                    </Button>
                    <Button
                      variant='destructive'
                      size='sm'
                      onClick={() => setDeleteTarget(origin)}
                    >
                      <Trash2 className='size-3.5 mr-1' />
                      {tc('delete')}
                    </Button>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </div>

      <OriginEditorDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        origin={editingOrigin}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              onClick={() => {
                if (deleteTarget) deleteMutation.mutate(deleteTarget.id);
              }}
            >
              {t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
