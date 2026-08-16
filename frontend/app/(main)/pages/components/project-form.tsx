'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslations } from 'next-intl';
import { useMemo } from 'react';
import { useForm, type UseFormReturn } from 'react-hook-form';
import { z } from 'zod';

import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { type PagesProject } from '@/lib/services/openflare';

function createPagesProjectSchema(t: (key: string) => string) {
  return z
    .object({
      name: z.string().trim().min(1, t('form.nameRequired')).max(255),
      slug: z.string().trim().max(255).optional().or(z.literal('')),
      spa_fallback_enabled: z.boolean(),
      spa_fallback_path: z.string().trim(),
      api_proxy_enabled: z.boolean(),
      api_proxy_path: z.string().trim(),
      api_proxy_pass: z.string().trim(),
      api_proxy_rewrite: z.string().trim(),
      root_dir: z.string().trim().max(512).optional().or(z.literal('')),
      entry_file: z.string().trim().min(1, t('form.entryRequired')).max(512),
    })
    .superRefine((data, ctx) => {
      if (
        data.spa_fallback_enabled &&
        !data.spa_fallback_path.startsWith('/')
      ) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['spa_fallback_path'],
          message: t('form.fallbackPathPrefix'),
        });
      }
      if (data.api_proxy_enabled) {
        if (!data.api_proxy_path.startsWith('/')) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['api_proxy_path'],
            message: t('form.matchPathPrefix'),
          });
        }
        if (!/^https?:\/\//i.test(data.api_proxy_pass)) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['api_proxy_pass'],
            message: t('form.backendPrefix'),
          });
        }
      }
    });
}

export type PagesProjectFormValues = z.infer<
  ReturnType<typeof createPagesProjectSchema>
>;

export function toFormValues(
  project?: PagesProject | null,
): PagesProjectFormValues {
  if (!project) {
    return {
      name: '',
      slug: '',
      spa_fallback_enabled: false,
      spa_fallback_path: '/index.html',
      api_proxy_enabled: false,
      api_proxy_path: '',
      api_proxy_pass: '',
      api_proxy_rewrite: '',
      root_dir: '',
      entry_file: 'index.html',
    };
  }
  return {
    name: project.name,
    slug: project.slug,
    spa_fallback_enabled: project.spa_fallback_enabled,
    spa_fallback_path: project.spa_fallback_path,
    api_proxy_enabled: project.api_proxy_enabled || false,
    api_proxy_path: project.api_proxy_path || '',
    api_proxy_pass: project.api_proxy_pass || '',
    api_proxy_rewrite: project.api_proxy_rewrite || '',
    root_dir: project.root_dir || '',
    entry_file: project.entry_file || 'index.html',
  };
}

export function buildProjectPayload(
  values: PagesProjectFormValues,
  project?: PagesProject | null,
) {
  return {
    name: values.name.trim(),
    slug: values.slug?.trim() || '',
    description: project?.description || '',
    enabled: project ? project.enabled : true,
    spa_fallback_enabled: values.spa_fallback_enabled,
    spa_fallback_path: values.spa_fallback_enabled
      ? values.spa_fallback_path.trim()
      : project?.spa_fallback_path || '/index.html',
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
    root_dir: values.root_dir?.trim() || '',
    entry_file: values.entry_file.trim(),
  };
}

export function usePagesProjectForm(project?: PagesProject | null) {
  const t = useTranslations('pages');
  const schema = useMemo(() => createPagesProjectSchema((key) => t(key)), [t]);
  return useForm<PagesProjectFormValues>({
    resolver: zodResolver(schema),
    defaultValues: toFormValues(project),
  });
}

interface ProjectFormFieldsProps {
  form: UseFormReturn<PagesProjectFormValues>;
  idPrefix?: string;
}

export function ProjectFormFields({
  form,
  idPrefix = '',
}: ProjectFormFieldsProps) {
  const t = useTranslations('pages');
  const spaEnabled = form.watch('spa_fallback_enabled');
  const apiEnabled = form.watch('api_proxy_enabled');
  const fieldId = (name: string) => (idPrefix ? `${idPrefix}-${name}` : name);

  return (
    <div className='space-y-4'>
      <div className='grid gap-4 md:grid-cols-2'>
        <div className='space-y-1.5'>
          <Label htmlFor={fieldId('name')}>{t('form.name')}</Label>
          <Input id={fieldId('name')} {...form.register('name')} />
          {form.formState.errors.name ? (
            <p className='text-xs text-destructive'>
              {form.formState.errors.name.message}
            </p>
          ) : null}
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor={fieldId('slug')}>{t('form.slug')}</Label>
          <Input
            id={fieldId('slug')}
            placeholder={t('form.slugPlaceholder')}
            {...form.register('slug')}
          />
        </div>
      </div>

      <div className='grid gap-4 md:grid-cols-2'>
        <div className='space-y-1.5'>
          <Label htmlFor={fieldId('entry_file')}>{t('form.entryFile')}</Label>
          <Input id={fieldId('entry_file')} {...form.register('entry_file')} />
        </div>
        <div className='space-y-1.5'>
          <Label htmlFor={fieldId('root_dir')}>{t('form.rootDir')}</Label>
          <Input
            id={fieldId('root_dir')}
            placeholder={t('form.optional')}
            {...form.register('root_dir')}
          />
        </div>
      </div>

      <div className='flex items-center justify-between rounded-lg border border-dashed px-4 py-3'>
        <div>
          <p className='text-sm font-medium'>{t('form.spaFallback')}</p>
          <p className='text-xs text-muted-foreground'>
            {t('form.spaFallbackDesc')}
          </p>
        </div>
        <Switch
          checked={spaEnabled}
          onCheckedChange={(checked) =>
            form.setValue('spa_fallback_enabled', checked, {
              shouldDirty: true,
            })
          }
        />
      </div>
      {spaEnabled ? (
        <div className='space-y-1.5'>
          <Label htmlFor={fieldId('spa_fallback_path')}>
            {t('form.fallbackPath')}
          </Label>
          <Input
            id={fieldId('spa_fallback_path')}
            {...form.register('spa_fallback_path')}
          />
        </div>
      ) : null}

      <div className='flex items-center justify-between rounded-lg border border-dashed px-4 py-3'>
        <div>
          <p className='text-sm font-medium'>{t('form.apiProxy')}</p>
          <p className='text-xs text-muted-foreground'>
            {t('form.apiProxyDesc')}
          </p>
        </div>
        <Switch
          checked={apiEnabled}
          onCheckedChange={(checked) =>
            form.setValue('api_proxy_enabled', checked, { shouldDirty: true })
          }
        />
      </div>
      {apiEnabled ? (
        <div className='grid gap-4 md:grid-cols-2'>
          <div className='space-y-1.5'>
            <Label htmlFor={fieldId('api_proxy_path')}>
              {t('form.matchPath')}
            </Label>
            <Input
              id={fieldId('api_proxy_path')}
              {...form.register('api_proxy_path')}
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor={fieldId('api_proxy_pass')}>
              {t('form.backend')}
            </Label>
            <Input
              id={fieldId('api_proxy_pass')}
              {...form.register('api_proxy_pass')}
            />
          </div>
          <div className='space-y-1.5 md:col-span-2'>
            <Label htmlFor={fieldId('api_proxy_rewrite')}>
              {t('form.rewrite')}
            </Label>
            <Input
              id={fieldId('api_proxy_rewrite')}
              {...form.register('api_proxy_rewrite')}
            />
          </div>
        </div>
      ) : null}
    </div>
  );
}
