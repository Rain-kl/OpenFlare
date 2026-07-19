'use client';

import { useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { type PagesProject, PagesService } from '@/lib/services/openflare';

import { projectQueryKey, projectsQueryKey } from './pages-utils';
import {
  buildProjectPayload,
  ProjectFormFields,
  toFormValues,
  usePagesProjectForm,
} from './project-form';

interface ProjectEditorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  project?: PagesProject | null;
}

export function ProjectEditorDialog({
  open,
  onOpenChange,
  project,
}: ProjectEditorDialogProps) {
  const queryClient = useQueryClient();
  const form = usePagesProjectForm(project);

  useEffect(() => {
    if (open) form.reset(toFormValues(project));
  }, [form, project, open]);

  const mutation = useMutation({
    mutationFn: async (values: Parameters<typeof buildProjectPayload>[0]) => {
      const payload = buildProjectPayload(values, project);
      return project
        ? PagesService.updateProject(project.id, payload)
        : PagesService.createProject(payload);
    },
    onSuccess: async () => {
      toast.success(project ? '项目已更新' : '项目已创建');
      await queryClient.invalidateQueries({ queryKey: projectsQueryKey });
      if (project) {
        await queryClient.invalidateQueries({
          queryKey: projectQueryKey(project.id),
        });
      }
      onOpenChange(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-2xl max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>
            {project ? '编辑 Pages 项目' : '新建 Pages 项目'}
          </DialogTitle>
          <DialogDescription>
            配置静态站点托管参数，上传部署包后在代理规则中选择 Pages 上游。
          </DialogDescription>
        </DialogHeader>

        <form
          id='pages-project-form'
          className='space-y-4'
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          <ProjectFormFields form={form} idPrefix='dialog' />
        </form>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            type='submit'
            form='pages-project-form'
            disabled={mutation.isPending}
          >
            {mutation.isPending ? (
              <>
                <Loader2 className='size-4 animate-spin mr-1' />
                保存中...
              </>
            ) : project ? (
              '保存修改'
            ) : (
              '创建项目'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
