'use client';

import {useQuery} from '@tanstack/react-query';
import {useState} from 'react';

import {AppCard} from '@/components/ui/app-card';
import {getPagesProjects} from '@/features/pages/api/pages';
import {PrimaryButton,} from '@/features/shared/components/resource-primitives';
import {PagesProjectEditorModal} from './pages-project-editor-modal';
import {PagesProjectListItem} from './pages-project-list-item';
import {projectsQueryKey} from '../utils';

export { PagesProjectDetailPage } from './pages-detail-page';

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
            创建 Pages 项目，上传已构建 of zip 静态资源包，然后在规则中选择
            Pages 项目作为上游。发布后 Agent
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
          />
        ) : (
          (projectsQuery.data ?? []).map((project) => (
            <PagesProjectListItem key={project.id} project={project} />
          ))
        )}
      </div>

      <PagesProjectEditorModal
        isOpen={isCreateModalOpen}
        onClose={() => setCreateModalOpen(false)}
      />
    </div>
  );
}
