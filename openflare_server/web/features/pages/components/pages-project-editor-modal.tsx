'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';

import { AppModal } from '@/components/ui/app-modal';
import {
  createPagesProject,
  updatePagesProject,
} from '@/features/pages/api/pages';
import type { PagesProject } from '@/features/pages/types';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
import { projectQueryKey, projectsQueryKey } from '../utils';
import { pagesProjectSchema, type PagesProjectFormValues } from '../schemas';

interface PagesProjectEditorModalProps {
  isOpen: boolean;
  onClose: () => void;
  project?: PagesProject | null;
  onSaved?: (project: PagesProject, mode: 'create' | 'update') => void;
}

function toFormValues(project?: PagesProject | null): PagesProjectFormValues {
  if (!project) {
    return {
      name: '',
      slug: '',
      description: '',
      spa_fallback_enabled: false,
      spa_fallback_path: '/index.html',
      api_proxy_enabled: false,
      api_proxy_path: '',
      api_proxy_pass: '',
      api_proxy_rewrite: '',
    };
  }
  return {
    name: project.name,
    slug: project.slug,
    description: project.description || '',
    spa_fallback_enabled: project.spa_fallback_enabled,
    spa_fallback_path: project.spa_fallback_path,
    api_proxy_enabled: project.api_proxy_enabled || false,
    api_proxy_path: project.api_proxy_path || '',
    api_proxy_pass: project.api_proxy_pass || '',
    api_proxy_rewrite: project.api_proxy_rewrite || '',
  };
}

export function PagesProjectEditorModal({
  isOpen,
  onClose,
  project,
  onSaved,
}: PagesProjectEditorModalProps) {
  const queryClient = useQueryClient();

  const form = useForm<PagesProjectFormValues>({
    resolver: zodResolver(pagesProjectSchema),
    defaultValues: toFormValues(project),
  });

  useEffect(() => {
    if (isOpen) {
      form.reset(toFormValues(project));
    }
  }, [form, project, isOpen]);

  const mutation = useMutation({
    mutationFn: (values: PagesProjectFormValues) => {
      const payload = {
        name: values.name.trim(),
        slug: values.slug?.trim() || '',
        description: values.description?.trim() || '',
        enabled: project ? project.enabled : true,
        spa_fallback_enabled: values.spa_fallback_enabled,
        spa_fallback_path: values.spa_fallback_enabled
          ? values.spa_fallback_path.trim()
          : project
            ? project.spa_fallback_path
            : '/index.html',
        api_proxy_enabled: values.api_proxy_enabled,
        api_proxy_path: values.api_proxy_enabled
          ? values.api_proxy_path.trim()
          : '',
        api_proxy_pass: values.api_proxy_enabled
          ? values.api_proxy_pass.trim()
          : '',
        api_proxy_rewrite: values.api_proxy_enabled
          ? values.api_proxy_rewrite.trim()
          : '',
      };
      return project
        ? updatePagesProject(project.id, payload)
        : createPagesProject(payload);
    },
    onSuccess: (savedProject) => {
      onClose();
      if (project) {
        queryClient.invalidateQueries({
          queryKey: projectQueryKey(project.id),
        });
      }
      queryClient.invalidateQueries({ queryKey: projectsQueryKey });
      onSaved?.(savedProject, project ? 'update' : 'create');
    },
  });

  const handleSubmit = form.handleSubmit((values) => {
    mutation.mutate(values);
  });

  const spaFallbackEnabled = form.watch('spa_fallback_enabled');
  const apiProxyEnabled = form.watch('api_proxy_enabled');

  return (
    <AppModal
      isOpen={isOpen}
      onClose={onClose}
      title={project ? '编辑 Pages 项目' : '新建 Pages 项目'}
      description={
        project
          ? '修改静态站点项目的基础配置与高级反代规则。'
          : '配置静态站点项目的基础信息与高级反代规则。创建后再上传已构建的 zip 部署包。'
      }
      footer={
        <div className="flex flex-wrap justify-end gap-3">
          <SecondaryButton type="button" onClick={onClose}>
            取消
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            form="pages-project-editor-form"
            disabled={mutation.isPending || !form.formState.isValid}
          >
            {mutation.isPending
              ? '保存中...'
              : project
                ? '保存修改'
                : '创建项目'}
          </PrimaryButton>
        </div>
      }
    >
      <form
        id="pages-project-editor-form"
        className="grid gap-4 md:grid-cols-2"
        onSubmit={handleSubmit}
      >
        <ResourceField
          label="项目名称"
          error={form.formState.errors.name?.message}
        >
          <ResourceInput
            placeholder="Marketing Site"
            {...form.register('name')}
            required
          />
        </ResourceField>
        <ResourceField
          label="项目标识"
          hint="留空时会按名称自动生成。"
          error={form.formState.errors.slug?.message}
        >
          <ResourceInput
            placeholder="marketing-site"
            {...form.register('slug')}
          />
        </ResourceField>
        <ResourceField
          label="描述"
          className="md:col-span-2"
          error={form.formState.errors.description?.message}
        >
          <ResourceInput
            placeholder="这个项目托管的静态站点用途"
            {...form.register('description')}
          />
        </ResourceField>
        <ToggleField
          label="启用 SPA fallback"
          description="开启后未命中的路径会回退到指定文件，适合 React/Vue history 路由。"
          checked={spaFallbackEnabled}
          onChange={(checked) =>
            form.setValue('spa_fallback_enabled', checked, {
              shouldValidate: true,
            })
          }
        />
        <ResourceField
          label="SPA 回退路径"
          hint="以 / 开头，例如 /index.html 或 /app.html。关闭 fallback 时不会生效。"
          error={form.formState.errors.spa_fallback_path?.message}
        >
          <ResourceInput
            placeholder="/index.html"
            disabled={!spaFallbackEnabled}
            {...form.register('spa_fallback_path')}
          />
        </ResourceField>
        <ToggleField
          label="启用 API 反向代理"
          description="允许为该静态站点配置反代后端（例如反代指定 API 路径至您的后端服务）。"
          checked={apiProxyEnabled}
          onChange={(checked) =>
            form.setValue('api_proxy_enabled', checked, {
              shouldValidate: true,
            })
          }
        />
        {apiProxyEnabled && (
          <>
            <ResourceField
              label="反代匹配路径"
              hint="以 / 开头，例如 /api 或 /api/v1。匹配该前缀的请求将被转发。"
              error={form.formState.errors.api_proxy_path?.message}
            >
              <ResourceInput
                placeholder="/api"
                {...form.register('api_proxy_path')}
                required
              />
            </ResourceField>
            <ResourceField
              label="后端服务地址"
              hint="包含协议和主机的完整 URL，例如 http://127.0.0.1:8080。"
              error={form.formState.errors.api_proxy_pass?.message}
            >
              <ResourceInput
                placeholder="http://127.0.0.1:8080"
                {...form.register('api_proxy_pass')}
                required
              />
            </ResourceField>
            <ResourceField
              label="路径重写目标"
              hint="可选。如果配置为 /，请求 /api/users 将被重写转发至后端 /users 路径。"
              error={form.formState.errors.api_proxy_rewrite?.message}
            >
              <ResourceInput
                placeholder="/"
                {...form.register('api_proxy_rewrite')}
              />
            </ResourceField>
          </>
        )}
        {mutation.isError ? (
          <p className="text-sm text-[var(--status-danger-foreground)] md:col-span-2">
            {mutation.error instanceof Error
              ? mutation.error.message
              : '操作失败，请稍后重试。'}
          </p>
        ) : null}
      </form>
    </AppModal>
  );
}
