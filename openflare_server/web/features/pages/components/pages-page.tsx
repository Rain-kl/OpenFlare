'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useState, type FormEvent } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { AppModal } from '@/components/ui/app-modal';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  activatePagesDeployment,
  createPagesProject,
  deletePagesDeployment,
  deletePagesProject,
  getPagesProject,
  getPagesDeployments,
  getPagesProjects,
  updatePagesProject,
  uploadPagesDeployment,
} from '@/features/pages/api/pages';
import type { PagesProject } from '@/features/pages/types';
import {
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';

const projectsQueryKey = ['pages-projects'];

function projectQueryKey(projectId: string | number) {
  return ['pages-project', String(projectId)];
}

function deploymentsQueryKey(projectId: string | number) {
  return ['pages-deployments', Number(projectId)];
}

function formatBytes(value: number) {
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KiB`;
  }
  return `${(value / 1024 / 1024).toFixed(1)} MiB`;
}

function formatDate(value?: string | null) {
  if (!value) {
    return '未激活';
  }
  return new Date(value).toLocaleString();
}

export function PagesPage() {
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);

  const projectsQuery = useQuery({
    queryKey: projectsQueryKey,
    queryFn: getPagesProjects,
  });

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <p className="text-sm font-medium text-[var(--foreground-secondary)]">
            OpenFlare Pages
          </p>
          <h1 className="text-2xl font-semibold text-[var(--foreground-primary)]">
            边缘静态站点托管
          </h1>
          <p className="max-w-3xl text-sm leading-6 text-[var(--foreground-secondary)]">
            创建 Pages 项目，上传已构建的 zip 静态资源包，然后在规则中选择 Pages
            项目作为上游。发布后 Agent
            会拉取部署包并在边缘节点本地服务静态文件。
          </p>
        </div>
        <PrimaryButton
          type="button"
          className="w-full lg:w-auto"
          onClick={() => setCreateModalOpen(true)}
        >
          新建 Pages 项目
        </PrimaryButton>
      </div>

      <div className="space-y-4">
        {projectsQuery.isLoading ? (
          <AppCard>正在加载 Pages 项目...</AppCard>
        ) : projectsQuery.error ? (
          <AppCard>
            <p className="text-sm text-[var(--status-danger-foreground)]">
              {projectsQuery.error.message}
            </p>
          </AppCard>
        ) : (projectsQuery.data ?? []).length === 0 ? (
          <AppCard
            title="还没有 Pages 项目"
            description="先创建一个项目，再上传静态资源包。"
            action={
              <SecondaryButton
                type="button"
                onClick={() => setCreateModalOpen(true)}
              >
                新建 Pages 项目
              </SecondaryButton>
            }
          >
          </AppCard>
        ) : (
          (projectsQuery.data ?? []).map((project) => (
            <PagesProjectListItem key={project.id} project={project} />
          ))
        )}
      </div>

      <PagesProjectCreateModal
        isOpen={isCreateModalOpen}
        onClose={() => setCreateModalOpen(false)}
      />
    </div>
  );
}

export function PagesProjectDetailPage({ projectId }: { projectId: string }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [entryFile, setEntryFile] = useState('index.html');
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [isEditModalOpen, setEditModalOpen] = useState(false);

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

  const uploadMutation = useMutation({
    mutationFn: () => {
      if (!file) {
        throw new Error('请选择 zip 文件');
      }
      setUploadProgress(0);
      return uploadPagesDeployment(parsedProjectId, file, entryFile, (percent) => {
        setUploadProgress(percent);
      });
    },
    onSuccess: () => {
      setFile(null);
      setUploadProgress(null);
      queryClient.invalidateQueries({
        queryKey: deploymentsQueryKey(parsedProjectId),
      });
      queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) });
      queryClient.invalidateQueries({ queryKey: projectsQueryKey });
    },
    onError: () => {
      setUploadProgress(null);
    },
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


      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)]">
        <AppCard
          title="上传部署包"
          description="上传已构建的 zip 静态资源包，默认入口 index.html。"
        >
          <div className="space-y-4">
            <ResourceField
              label="部署包"
              hint="仅支持 zip，Server 会校验文件数量、体积、路径逃逸和入口文件。"
            >
              <ResourceInput
                type="file"
                accept=".zip,application/zip"
                onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              />
            </ResourceField>
            <ResourceField label="入口文件">
              <ResourceInput
                value={entryFile}
                onChange={(event) => setEntryFile(event.target.value)}
              />
            </ResourceField>
            {uploadProgress !== null && (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs font-medium text-[var(--foreground-secondary)]">
                  <span>上传进度</span>
                  <span>{uploadProgress}%</span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--surface-muted)]">
                  <div
                    className="h-full rounded-full bg-[var(--brand-primary)] transition-all duration-300 ease-out"
                    style={{ width: `${uploadProgress}%` }}
                  />
                </div>
              </div>
            )}
            <PrimaryButton
              type="button"
              disabled={!file || uploadMutation.isPending}
              onClick={() => uploadMutation.mutate()}
            >
              {uploadMutation.isPending
                ? `上传中 (${uploadProgress ?? 0}%)...`
                : '上传部署'}
            </PrimaryButton>
            {uploadMutation.error ? (
              <p className="text-sm text-[var(--status-danger-foreground)]">
                {uploadMutation.error.message}
              </p>
            ) : null}
          </div>
        </AppCard>

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
            />
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
                      {deployment.checksum.slice(0, 16)} ·{' '}
                      {deployment.file_count} files ·{' '}
                      {formatBytes(deployment.total_size)}
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
      </div>

      <PagesProjectEditModal
        isOpen={isEditModalOpen}
        onClose={() => setEditModalOpen(false)}
        project={project}
      />
    </div>
  );
}

function PagesProjectEditModal({
  isOpen,
  onClose,
  project,
}: {
  isOpen: boolean;
  onClose: () => void;
  project: PagesProject;
}) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(project.name);
  const [slug, setSlug] = useState(project.slug);
  const [description, setDescription] = useState(project.description || '');
  const [spaFallbackEnabled, setSpaFallbackEnabled] = useState(project.spa_fallback_enabled);
  const [spaFallbackPath, setSpaFallbackPath] = useState(project.spa_fallback_path);

  useEffect(() => {
    setName(project.name);
    setSlug(project.slug);
    setDescription(project.description || '');
    setSpaFallbackEnabled(project.spa_fallback_enabled);
    setSpaFallbackPath(project.spa_fallback_path);
  }, [project]);

  const updateMutation = useMutation({
    mutationFn: () =>
      updatePagesProject(project.id, {
        name,
        slug,
        description,
        enabled: project.enabled,
        spa_fallback_enabled: spaFallbackEnabled,
        spa_fallback_path: spaFallbackPath,
      }),
    onSuccess: () => {
      onClose();
      queryClient.invalidateQueries({ queryKey: projectQueryKey(project.id) });
      queryClient.invalidateQueries({ queryKey: projectsQueryKey });
    },
  });

  function handleUpdate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    updateMutation.mutate();
  }

  return (
    <AppModal
      isOpen={isOpen}
      onClose={onClose}
      title="编辑 Pages 项目"
      description="修改静态站点项目的基础配置。"
      footer={
        <div className="flex flex-wrap justify-end gap-3">
          <SecondaryButton type="button" onClick={onClose}>
            取消
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            form="pages-project-edit-form"
            disabled={updateMutation.isPending || name.trim() === ''}
          >
            {updateMutation.isPending ? '保存中...' : '保存修改'}
          </PrimaryButton>
        </div>
      }
    >
      <form
        id="pages-project-edit-form"
        className="grid gap-4 md:grid-cols-2"
        onSubmit={handleUpdate}
      >
        <ResourceField label="项目名称">
          <ResourceInput
            value={name}
            placeholder="Marketing Site"
            onChange={(event) => setName(event.target.value)}
            required
          />
        </ResourceField>
        <ResourceField label="项目标识" hint="留空时会按名称自动生成。">
          <ResourceInput
            value={slug}
            placeholder="marketing-site"
            onChange={(event) => setSlug(event.target.value)}
          />
        </ResourceField>
        <ResourceField label="描述" className="md:col-span-2">
          <ResourceInput
            value={description}
            placeholder="这个项目托管的静态站点用途"
            onChange={(event) => setDescription(event.target.value)}
          />
        </ResourceField>
        <ToggleField
          label="启用 SPA fallback"
          description="开启后未命中的路径会回退到指定文件，适合 React/Vue history 路由。"
          checked={spaFallbackEnabled}
          onChange={setSpaFallbackEnabled}
        />
        <ResourceField
          label="SPA 回退路径"
          hint="以 / 开头，例如 /index.html 或 /app.html。关闭 fallback 时不会生效。"
        >
          <ResourceInput
            value={spaFallbackPath}
            placeholder="/index.html"
            disabled={!spaFallbackEnabled}
            onChange={(event) => setSpaFallbackPath(event.target.value)}
          />
        </ResourceField>
        {updateMutation.error ? (
          <p className="text-sm text-[var(--status-danger-foreground)] md:col-span-2">
            {updateMutation.error.message}
          </p>
        ) : null}
      </form>
    </AppModal>
  );
}

function PagesProjectCreateModal({
  isOpen,
  onClose,
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [spaFallbackEnabled, setSpaFallbackEnabled] = useState(false);
  const [spaFallbackPath, setSpaFallbackPath] = useState('/index.html');

  const resetForm = () => {
    setName('');
    setSlug('');
    setDescription('');
    setSpaFallbackEnabled(false);
    setSpaFallbackPath('/index.html');
  };

  const closeModal = () => {
    resetForm();
    onClose();
  };

  const createMutation = useMutation({
    mutationFn: () =>
      createPagesProject({
        name,
        slug,
        description,
        enabled: true,
        spa_fallback_enabled: spaFallbackEnabled,
        spa_fallback_path: spaFallbackPath,
      }),
    onSuccess: () => {
      resetForm();
      onClose();
      queryClient.invalidateQueries({ queryKey: projectsQueryKey });
    },
  });

  function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    createMutation.mutate();
  }

  return (
    <AppModal
      isOpen={isOpen}
      onClose={closeModal}
      title="新建 Pages 项目"
      description="配置静态站点项目的基础信息。创建后再上传已构建的 zip 部署包。"
      footer={
        <div className="flex flex-wrap justify-end gap-3">
          <SecondaryButton type="button" onClick={closeModal}>
            取消
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            form="pages-project-create-form"
            disabled={createMutation.isPending || name.trim() === ''}
          >
            {createMutation.isPending ? '创建中...' : '创建项目'}
          </PrimaryButton>
        </div>
      }
    >
      <form
        id="pages-project-create-form"
        className="grid gap-4 md:grid-cols-2"
        onSubmit={handleCreate}
      >
        <ResourceField label="项目名称">
          <ResourceInput
            value={name}
            placeholder="Marketing Site"
            onChange={(event) => setName(event.target.value)}
            required
          />
        </ResourceField>
        <ResourceField label="项目标识" hint="留空时会按名称自动生成。">
          <ResourceInput
            value={slug}
            placeholder="marketing-site"
            onChange={(event) => setSlug(event.target.value)}
          />
        </ResourceField>
        <ResourceField label="描述" className="md:col-span-2">
          <ResourceInput
            value={description}
            placeholder="这个项目托管的静态站点用途"
            onChange={(event) => setDescription(event.target.value)}
          />
        </ResourceField>
        <ToggleField
          label="启用 SPA fallback"
          description="开启后未命中的路径会回退到指定文件，适合 React/Vue history 路由。"
          checked={spaFallbackEnabled}
          onChange={setSpaFallbackEnabled}
        />
        <ResourceField
          label="SPA 回退路径"
          hint="以 / 开头，例如 /index.html 或 /app.html。关闭 fallback 时不会生效。"
        >
          <ResourceInput
            value={spaFallbackPath}
            placeholder="/index.html"
            disabled={!spaFallbackEnabled}
            onChange={(event) => setSpaFallbackPath(event.target.value)}
          />
        </ResourceField>
        {createMutation.error ? (
          <p className="text-sm text-[var(--status-danger-foreground)] md:col-span-2">
            {createMutation.error.message}
          </p>
        ) : null}
      </form>
    </AppModal>
  );
}

function PagesProjectListItem({ project }: { project: PagesProject }) {
  return (
    <Link
      href={`/pages/detail?id=${project.id}`}
      className="group block rounded-[28px] border border-[var(--border-default)] bg-[var(--surface-panel)] p-5 shadow-[var(--shadow-card)] transition hover:-translate-y-0.5 hover:border-[var(--border-strong)] hover:shadow-[var(--shadow-soft)]"
    >
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-lg font-semibold text-[var(--foreground-primary)]">
              {project.name}
            </h2>
            <StatusBadge
              label={project.enabled ? '已启用' : '已停用'}
              variant={project.enabled ? 'success' : 'warning'}
            />
            <StatusBadge
              label={project.spa_fallback_enabled ? 'SPA fallback' : '严格 404'}
              variant={project.spa_fallback_enabled ? 'info' : 'warning'}
            />
          </div>
          <p className="text-sm text-[var(--foreground-secondary)]">
            {project.slug}
          </p>
          {project.description ? (
            <p className="line-clamp-2 text-sm leading-6 text-[var(--foreground-secondary)]">
              {project.description}
            </p>
          ) : null}
          {project.spa_fallback_enabled ? (
            <p className="text-xs text-[var(--foreground-secondary)]">
              回退路径：{project.spa_fallback_path || '/index.html'}
            </p>
          ) : null}
        </div>

        <div className="grid shrink-0 grid-cols-2 gap-3 text-sm md:min-w-80">
          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] px-4 py-3">
            <p className="text-xs text-[var(--foreground-secondary)]">部署数</p>
            <p className="mt-1 font-semibold text-[var(--foreground-primary)]">
              {project.deployment_count}
            </p>
          </div>
          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] px-4 py-3">
            <p className="text-xs text-[var(--foreground-secondary)]">
              当前激活
            </p>
            <p className="mt-1 font-semibold text-[var(--foreground-primary)]">
              {project.active_deployment
                ? `#${project.active_deployment.deployment_number}`
                : '暂无'}
            </p>
          </div>
        </div>
      </div>
      <div className="mt-4 flex items-center justify-between border-t border-[var(--border-default)] pt-4">
        <p className="text-xs text-[var(--foreground-secondary)]">
          激活时间：{formatDate(project.active_deployment?.activated_at)}
        </p>
        <span className="text-sm font-medium text-[var(--brand-primary)] transition group-hover:translate-x-1">
          查看详情 →
        </span>
      </div>
    </Link>
  );
}

