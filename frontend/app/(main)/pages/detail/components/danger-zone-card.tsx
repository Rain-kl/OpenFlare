'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
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
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Spinner } from '@/components/ui/spinner';
import { type PagesProject, PagesService } from '@/lib/services/openflare';

import { projectsQueryKey } from '../../components/pages-utils';

interface DangerZoneCardProps {
  project: PagesProject;
}

export function DangerZoneCard({ project }: DangerZoneCardProps) {
  const t = useTranslations('pages');
  const tCommon = useTranslations('common');
  const router = useRouter();
  const queryClient = useQueryClient();
  const [deleteOpen, setDeleteOpen] = useState(false);

  const deleteProjectMutation = useMutation({
    mutationFn: () => PagesService.deleteProject(project.id),
    onSuccess: async () => {
      toast.success(t('deleted'));
      await queryClient.invalidateQueries({ queryKey: projectsQueryKey });
      router.push('/pages');
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('deleteFailed'));
    },
  });

  return (
    <>
      <Card className='border-dashed border-destructive/30 bg-destructive/5 shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base text-destructive'>
            {t('danger.title')}
          </CardTitle>
        </CardHeader>
        <CardContent className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='space-y-1'>
            <p className='text-sm font-medium'>{t('danger.deleteProject')}</p>
            <p className='text-xs text-muted-foreground'>
              {t('danger.deleteHint', {
                name: project.name,
                slug: project.slug,
              })}
            </p>
          </div>
          <Button
            type='button'
            size='sm'
            variant='destructive'
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 data-icon='inline-start' />
            {t('danger.deleteProject')}
          </Button>
        </CardContent>
      </Card>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('danger.confirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('danger.confirmDesc', { name: project.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteProjectMutation.isPending}>
              {tCommon('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleteProjectMutation.isPending}
              onClick={(event) => {
                event.preventDefault();
                deleteProjectMutation.mutate();
              }}
            >
              {deleteProjectMutation.isPending ? (
                <Spinner data-icon='inline-start' />
              ) : null}
              {t('danger.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
