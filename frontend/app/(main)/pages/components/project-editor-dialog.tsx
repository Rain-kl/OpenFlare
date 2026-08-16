'use client';

import { useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
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
  const t = useTranslations('pages');
  const tCommon = useTranslations('common');
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
      toast.success(project ? t('updated') : t('created'));
      await queryClient.invalidateQueries({ queryKey: projectsQueryKey });
      if (project) {
        await queryClient.invalidateQueries({
          queryKey: projectQueryKey(project.id),
        });
      }
      onOpenChange(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('saveFailed'));
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-2xl max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>
            {project ? t('editTitle') : t('createTitle')}
          </DialogTitle>
          <DialogDescription>{t('editorDesc')}</DialogDescription>
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
            {tCommon('cancel')}
          </Button>
          <Button
            type='submit'
            form='pages-project-form'
            disabled={mutation.isPending}
          >
            {mutation.isPending ? (
              <>
                <Loader2 className='size-4 animate-spin mr-1' />
                {t('saving')}
              </>
            ) : project ? (
              t('saveChanges')
            ) : (
              t('createProject')
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
