'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { Trash2 } from 'lucide-react';
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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Spinner } from '@/components/ui/spinner';
import { type PagesProject, PagesService } from '@/lib/services/openflare';

import { projectsQueryKey } from '../../components/pages-utils';

interface DangerZoneCardProps {
  project: PagesProject;
}

export function DangerZoneCard({ project }: DangerZoneCardProps) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [deleteOpen, setDeleteOpen] = useState(false);

  const deleteProjectMutation = useMutation({
    mutationFn: () => PagesService.deleteProject(project.id),
    onSuccess: async () => {
      toast.success('项目已删除');
      await queryClient.invalidateQueries({ queryKey: projectsQueryKey });
      router.push('/pages');
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '删除失败');
    },
  });

  return (
    <>
      <Card className='border-dashed border-destructive/30 bg-destructive/5 shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base text-destructive'>危险区域</CardTitle>
        </CardHeader>
        <CardContent className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='space-y-1'>
            <p className='text-sm font-medium'>删除项目</p>
            <p className='text-xs text-muted-foreground'>
              将永久删除 {project.name}（{project.slug}）及其全部部署。
            </p>
          </div>
          <Button
            type='button'
            size='sm'
            variant='destructive'
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 data-icon='inline-start' />
            删除项目
          </Button>
        </CardContent>
      </Card>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除 Pages 项目</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除项目 {project.name} 吗？此操作不可恢复。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteProjectMutation.isPending}>
              取消
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
              确认删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
