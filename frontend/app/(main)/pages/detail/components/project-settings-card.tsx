'use client';

import { useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { type PagesProject, PagesService } from '@/lib/services/openflare';

import {
  projectQueryKey,
  projectsQueryKey,
  sourceQueryKey,
} from '../../components/pages-utils';
import {
  buildProjectPayload,
  ProjectFormFields,
  toFormValues,
  usePagesProjectForm,
} from '../../components/project-form';

interface ProjectSettingsCardProps {
  project: PagesProject;
}

export function ProjectSettingsCard({ project }: ProjectSettingsCardProps) {
  const queryClient = useQueryClient();
  const form = usePagesProjectForm(project);

  useEffect(() => {
    form.reset(toFormValues(project));
  }, [form, project]);

  const mutation = useMutation({
    mutationFn: async (values: Parameters<typeof buildProjectPayload>[0]) => {
      const payload = buildProjectPayload(values, project);
      return PagesService.updateProject(project.id, payload);
    },
    onSuccess: async () => {
      toast.success('项目已更新');
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
        queryClient.invalidateQueries({
          queryKey: projectQueryKey(project.id),
        }),
        queryClient.invalidateQueries({
          queryKey: sourceQueryKey(project.id),
        }),
      ]);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='flex flex-row items-center justify-between gap-4'>
        <div>
          <CardTitle className='text-base'>编辑 Pages 项目</CardTitle>
          <CardDescription>
            配置静态站点托管参数，保存后会同步到项目详情与代理引用。
          </CardDescription>
        </div>
        <div className='flex shrink-0 flex-wrap gap-2'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={!form.formState.isDirty || mutation.isPending}
            onClick={() => form.reset(toFormValues(project))}
          >
            重置
          </Button>
          <Button
            type='submit'
            size='sm'
            form='pages-project-settings-form'
            disabled={!form.formState.isDirty || mutation.isPending}
          >
            {mutation.isPending ? (
              <>
                <Loader2 className='mr-1 size-4 animate-spin' />
                保存中...
              </>
            ) : (
              '保存修改'
            )}
          </Button>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        <form
          id='pages-project-settings-form'
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <ProjectFormFields form={form} idPrefix='settings' />
        </form>
      </CardContent>
    </Card>
  );
}
