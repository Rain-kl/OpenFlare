'use client';

import Link from 'next/link';
import { Suspense, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRouter, useSearchParams } from 'next/navigation';
import { ArrowLeft, FileText, Pencil, Trash2, Upload } from 'lucide-react';
import { toast } from 'sonner';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
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
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Spinner } from '@/components/ui/spinner';
import { PagesService } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import {
  DeploymentUploadDialog,
  pagesEntryPath,
} from '../components/deployment-upload-dialog';
import { ProjectEditorDialog } from '../components/project-editor-dialog';
import {
  projectQueryKey,
  projectsQueryKey,
  sourceQueryKey,
} from '../components/pages-utils';
import { DeploymentHistory } from './components/deployment-history';
import { PagesSourceCard } from './components/pages-source-card';

function PagesDetailPageFallback() {
  return (
    <div className='flex w-full flex-col gap-6 px-1 py-6'>
      <Skeleton className='h-8 w-32' />
      <Skeleton className='h-12 w-full max-w-xl' />
      <div className='grid gap-4 lg:grid-cols-2'>
        <Skeleton className='h-40 w-full' />
        <Skeleton className='h-40 w-full' />
      </div>
      <Skeleton className='h-64 w-full' />
    </div>
  );
}

function PagesDetailRoute() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [deleteProjectOpen, setDeleteProjectOpen] = useState(false);

  const rawProjectId = searchParams.get('id')?.trim() ?? '';
  const projectId = Number(rawProjectId);
  const validProjectId =
    rawProjectId !== '' && Number.isInteger(projectId) && projectId > 0;

  const projectQuery = useQuery({
    queryKey: projectQueryKey(projectId),
    queryFn: () => PagesService.getProject(projectId),
    enabled: validProjectId,
  });

  const deleteProjectMutation = useMutation({
    mutationFn: () => PagesService.deleteProject(projectId),
    onSuccess: async () => {
      toast.success('项目已删除');
      await queryClient.invalidateQueries({ queryKey: projectsQueryKey });
      router.push('/pages');
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '删除失败');
    },
  });

  if (!validProjectId) {
    return (
      <div className='w-full px-1 py-6'>
        <EmptyStateWithBorder description='缺少有效的 Pages 项目 ID。' />
      </div>
    );
  }

  if (projectQuery.isLoading) {
    return (
      <div className='w-full px-1 py-6'>
        <LoadingStateWithBorder icon={FileText} description='加载项目详情...' />
      </div>
    );
  }

  if (projectQuery.isError) {
    return (
      <div className='w-full px-1 py-6'>
        <div className='rounded-lg border p-4'>
          <ErrorInline
            message={
              projectQuery.error instanceof Error
                ? projectQuery.error.message
                : '项目详情加载失败'
            }
            onRetry={() => void projectQuery.refetch()}
          />
        </div>
      </div>
    );
  }

  const project = projectQuery.data;
  if (!project) {
    return (
      <div className='flex w-full flex-col gap-4 px-1 py-6'>
        <Button variant='ghost' size='sm' asChild>
          <Link href='/pages'>
            <ArrowLeft data-icon='inline-start' />
            返回列表
          </Link>
        </Button>
        <EmptyStateWithBorder description='Pages 项目不存在或已被删除。' />
      </div>
    );
  }

  const activeDeployment = project.active_deployment;
  const entryPath = pagesEntryPath(project.root_dir ?? '', project.entry_file);

  return (
    <div className='flex w-full flex-col gap-6 px-1 py-6'>
      <div className='flex flex-col gap-4'>
        <Button variant='ghost' size='sm' className='self-start' asChild>
          <Link href='/pages'>
            <ArrowLeft data-icon='inline-start' />
            返回列表
          </Link>
        </Button>
        <div className='flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between'>
          <div className='flex flex-col gap-2'>
            <div className='flex items-center gap-2'>
              <FileText className='size-5 text-primary' />
              <h1 className='text-2xl font-semibold tracking-tight'>
                {project.name}
              </h1>
            </div>
            <p className='text-sm text-muted-foreground'>
              {project.slug} · {project.deployment_count} 个部署
            </p>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => setEditorOpen(true)}
            >
              <Pencil data-icon='inline-start' />
              编辑项目
            </Button>
            <Button type='button' size='sm' onClick={() => setUploadOpen(true)}>
              <Upload data-icon='inline-start' />
              上传部署包
            </Button>
            <Button
              type='button'
              variant='destructive'
              size='sm'
              onClick={() => setDeleteProjectOpen(true)}
            >
              <Trash2 data-icon='inline-start' />
              删除项目
            </Button>
          </div>
        </div>
      </div>

      <div className='grid gap-4 lg:grid-cols-2'>
        <Card>
          <CardHeader>
            <CardTitle>当前生产部署</CardTitle>
            <CardDescription>
              Agent 当前应拉取并提供服务的不可变部署。
            </CardDescription>
            <CardAction>
              <Badge variant={activeDeployment ? 'default' : 'outline'}>
                {activeDeployment ? '生产中' : '未发布'}
              </Badge>
            </CardAction>
          </CardHeader>
          <CardContent className='flex flex-col gap-2'>
            {activeDeployment ? (
              <>
                <p className='text-lg font-semibold'>
                  部署 #{activeDeployment.deployment_number}
                </p>
                <p className='font-mono text-xs text-muted-foreground'>
                  {activeDeployment.checksum.slice(0, 20)}
                </p>
                <p className='text-xs text-muted-foreground'>
                  激活于{' '}
                  {activeDeployment.activated_at
                    ? formatDateTime(activeDeployment.activated_at)
                    : '未知时间'}
                </p>
              </>
            ) : (
              <p className='text-sm text-muted-foreground'>
                尚无生产部署。上传或同步来源后，从部署历史激活一个版本。
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>站点入口</CardTitle>
            <CardDescription>
              解包校验、发布快照与 Agent 切换共同使用此路径。
            </CardDescription>
            <CardAction>
              <Badge variant={project.enabled ? 'secondary' : 'outline'}>
                {project.enabled ? '项目已启用' : '项目已停用'}
              </Badge>
            </CardAction>
          </CardHeader>
          <CardContent className='flex flex-col gap-2'>
            <code className='rounded-md border bg-muted/20 px-3 py-2 text-sm'>
              {entryPath}
            </code>
            <p className='text-xs text-muted-foreground'>
              项目更新于 {formatDateTime(project.updated_at)}
            </p>
          </CardContent>
        </Card>
      </div>

      <PagesSourceCard projectId={projectId} />
      <DeploymentHistory
        projectId={projectId}
        activeDeploymentId={project.active_deployment_id}
      />

      <ProjectEditorDialog
        open={editorOpen}
        onOpenChange={(nextOpen) => {
          setEditorOpen(nextOpen);
          if (!nextOpen) {
            void queryClient.invalidateQueries({
              queryKey: sourceQueryKey(projectId),
            });
          }
        }}
        project={project}
      />
      <DeploymentUploadDialog
        open={uploadOpen}
        onOpenChange={setUploadOpen}
        projectId={projectId}
        rootDir={project.root_dir ?? ''}
        entryFile={project.entry_file}
      />

      <AlertDialog open={deleteProjectOpen} onOpenChange={setDeleteProjectOpen}>
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
    </div>
  );
}

export default function PagesDetailPage() {
  return (
    <Suspense fallback={<PagesDetailPageFallback />}>
      <PagesDetailRoute />
    </Suspense>
  );
}
