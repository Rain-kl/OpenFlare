'use client';

import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {useState} from 'react';

import {EmptyState} from '@/components/feedback/empty-state';
import {ErrorState} from '@/components/feedback/error-state';
import {LoadingState} from '@/components/feedback/loading-state';
import {PageHeader} from '@/components/layout/page-header';
import {AppCard} from '@/components/ui/app-card';
import {
    activatePagesDeployment,
    deletePagesDeployment,
    deletePagesProject,
    getPagesDeployments,
    getPagesProject,
} from '@/features/pages/api/pages';
import {DangerButton, PrimaryButton, SecondaryButton,} from '@/features/shared/components/resource-primitives';
import {PagesProjectEditorModal} from './pages-project-editor-modal';
import {PagesDeploymentUploadModal} from './pages-deployment-upload-modal';
import {formatBytes} from '@/lib/utils/metrics';
import {deploymentsQueryKey, projectQueryKey, projectsQueryKey,} from '../utils';

export function PagesProjectDetailPage({ projectId }: { projectId: string }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isUploadModalOpen, setUploadModalOpen] = useState(false);

  const parsedProjectId = Number(projectId);
  const projectQuery = useQuery({
    queryKey: projectQueryKey(projectId),
    queryFn: () => getPagesProject(parsedProjectId),
    enabled: projectId !== '' && Number.isFinite(parsedProjectId),
  });
  const deploymentsQuery = useQuery({
    queryKey: deploymentsQueryKey(parsedProjectId),
    queryFn: () => getPagesDeployments(parsedProjectId),
    enabled: projectId !== '' && Number.isFinite(parsedProjectId),
  });

  const activateMutation = useMutation({
    mutationFn: (deploymentId: number) =>
      activatePagesDeployment(parsedProjectId, deploymentId),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: deploymentsQueryKey(parsedProjectId),
      });
      queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) });
      queryClient.invalidateQueries({ queryKey: projectsQueryKey });
    },
  });

  const deleteDeploymentMutation = useMutation({
    mutationFn: (deploymentId: number) =>
      deletePagesDeployment(parsedProjectId, deploymentId),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: deploymentsQueryKey(parsedProjectId),
      });
      queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) });
      queryClient.invalidateQueries({ queryKey: projectsQueryKey });
    },
  });

  const deleteProjectMutation = useMutation({
    mutationFn: () => deletePagesProject(parsedProjectId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: projectsQueryKey });
      router.push('/pages');
    },
  });

  if (projectId === '' || !Number.isFinite(parsedProjectId)) {
    return (
      <EmptyState
        title="Pages 项目不存在"
        description="缺少有效的 Pages 项目 ID，请从项目列表重新进入。"
      />
    );
  }

  if (projectQuery.isLoading) {
    return <LoadingState />;
  }

  if (projectQuery.isError) {
    return (
      <ErrorState
        title="Pages 项目加载失败"
        description={projectQuery.error.message}
      />
    );
  }

  const project = projectQuery.data;
  if (!project) {
    return (
      <EmptyState
        title="Pages 项目不存在"
        description="该项目可能已被删除，或当前 ID 无法匹配到项目记录。"
      />
    );
  }

  const handleDeleteProject = () => {
    if (!window.confirm(`确认删除 Pages 项目 ${project.name} 吗？`)) {
      return;
    }
    deleteProjectMutation.mutate();
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title={project.name}
        description={`${project.slug} · Pages 静态站点项目详情`}
        action={
          <>
            <Link
              href="/pages"
              className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
            >
              返回列表
            </Link>
            <SecondaryButton
              type="button"
              onClick={() => setEditModalOpen(true)}
            >
              编辑项目
            </SecondaryButton>
            <PrimaryButton
              type="button"
              onClick={() => setUploadModalOpen(true)}
            >
              上传部署包
            </PrimaryButton>
            <DangerButton
              type="button"
              disabled={deleteProjectMutation.isPending}
              onClick={handleDeleteProject}
            >
              删除项目
            </DangerButton>
          </>
        }
      />

      <AppCard
        title="部署历史"
        description="部署不可变；激活后发布配置，Agent 才会拉取并切换静态资源。"
      >
        {deploymentsQuery.isLoading ? (
          <p className="text-sm text-[var(--foreground-secondary)]">
            加载中...
          </p>
        ) : deploymentsQuery.isError ? (
          <p className="text-sm text-[var(--status-danger-foreground)]">
            {deploymentsQuery.error.message}
          </p>
        ) : (deploymentsQuery.data ?? []).length === 0 ? (
          <EmptyState
            title="暂无部署"
            description="上传 zip 部署包后，可以在这里激活某个部署版本。"
          >
          </EmptyState>
        ) : (
          <div className="overflow-hidden rounded-2xl border border-[var(--border-default)]">
            {(deploymentsQuery.data ?? []).map((deployment) => (
              <div
                key={deployment.id}
                className="flex flex-col gap-3 border-b border-[var(--border-default)] p-4 last:border-b-0 md:flex-row md:items-center md:justify-between"
              >
                <div>
                  <p className="text-sm font-medium text-[var(--foreground-primary)]">
                    #{deployment.deployment_number}{' '}
                    {deployment.status === 'active' ? '· 已激活' : ''}
                  </p>
                  <p className="mt-1 text-xs text-[var(--foreground-secondary)]">
                    {deployment.checksum.slice(0, 16)} · {deployment.file_count}{' '}
                    files · {formatBytes(deployment.total_size)}
                  </p>
                </div>
                <div className="flex gap-2">
                  <SecondaryButton
                    type="button"
                    disabled={
                      deployment.status === 'active' ||
                      activateMutation.isPending
                    }
                    onClick={() => {
                      if (
                        window.confirm(
                          `确认激活部署 #${deployment.deployment_number} 吗？`,
                        )
                      ) {
                        activateMutation.mutate(deployment.id);
                      }
                    }}
                  >
                    激活
                  </SecondaryButton>
                  <DangerButton
                    type="button"
                    disabled={
                      deployment.status === 'active' ||
                      deleteDeploymentMutation.isPending
                    }
                    onClick={() => {
                      if (
                        window.confirm(
                          `确认删除部署 #${deployment.deployment_number} 吗？`,
                        )
                      ) {
                        deleteDeploymentMutation.mutate(deployment.id);
                      }
                    }}
                  >
                    删除
                  </DangerButton>
                </div>
              </div>
            ))}
          </div>
        )}
      </AppCard>

      <PagesProjectEditorModal
        isOpen={isEditModalOpen}
        onClose={() => setEditModalOpen(false)}
        project={project}
      />

      <PagesDeploymentUploadModal
        isOpen={isUploadModalOpen}
        onClose={() => setUploadModalOpen(false)}
        projectId={parsedProjectId}
      />
    </div>
  );
}
