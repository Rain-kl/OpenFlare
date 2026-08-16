'use client';

import { useEffect } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
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
  const t = useTranslations('pages');
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
      toast.success(t('updated'));
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
      toast.error(error instanceof Error ? error.message : t('saveFailed'));
    },
  });

  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='flex flex-row items-center justify-between gap-4'>
        <div>
          <CardTitle className='text-base'>{t('editTitle')}</CardTitle>
          <CardDescription>{t('settingsDesc')}</CardDescription>
        </div>
        <div className='flex shrink-0 flex-wrap gap-2'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={!form.formState.isDirty || mutation.isPending}
            onClick={() => form.reset(toFormValues(project))}
          >
            {t('reset')}
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
                {t('saving')}
              </>
            ) : (
              t('saveChanges')
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
