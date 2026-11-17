'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useLocale, useTranslations } from 'next-intl';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2, Save } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { HtmlEditorWorkspace } from '@/components/common/html-editor-workspace';
import {
  POW_HTML_MAX_BYTES,
  POW_PAGE_KEY,
  powTemplates,
  previewPoWPage,
  readPoWPage,
} from '@/lib/openflare/pow-page';
import { OptionService, type PoWPageConfig } from '@/lib/services/openflare';
import { invalidateResponseQueries } from './shared';

export function PoWPageTab({
  optionMap,
}: {
  optionMap: Record<string, string>;
}) {
  const t = useTranslations('pow');
  const locale = useLocale();
  const queryClient = useQueryClient();
  const [config, setConfig] = useState<PoWPageConfig>(() =>
    readPoWPage(optionMap[POW_PAGE_KEY]),
  );
  useEffect(() => setConfig(readPoWPage(optionMap[POW_PAGE_KEY])), [optionMap]);
  const save = useMutation({
    mutationFn: async () => {
      if (
        new TextEncoder().encode(config.html ?? '').length > POW_HTML_MAX_BYTES
      )
        throw new Error(t('htmlTooLarge'));
      await OptionService.updateBatch([
        { key: POW_PAGE_KEY, value: JSON.stringify(config) },
      ]);
    },
    onSuccess: async () => {
      toast.success(t('pageSaved'));
      await invalidateResponseQueries(queryClient);
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : t('saveFailed')),
  });
  const preview = (html: string) =>
    previewPoWPage({ ...config, html }, locale, {
      title: t('previewTitle'),
      status: t('previewStatus'),
      protected: t('previewProtected'),
      why: t('previewWhy'),
      why_description: t('previewWhyDescription'),
      noscript: t('previewNoscript'),
      success: t('previewSuccess'),
    });
  return (
    <div className='flex flex-col gap-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('pageSettings')}</CardTitle>
          <CardDescription>{t('pageHelp')}</CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel>{t('pageStyle')}</FieldLabel>
              <ToggleGroup
                type='single'
                variant='outline'
                value={config.template}
                onValueChange={(template: PoWPageConfig['template']) => {
                  if (template) setConfig((prev) => ({ ...prev, template }));
                }}
              >
                <ToggleGroupItem value='shield'>{t('shield')}</ToggleGroupItem>
                <ToggleGroupItem value='classic'>
                  {t('classic')}
                </ToggleGroupItem>
              </ToggleGroup>
              <FieldDescription>{t('templateHelp')}</FieldDescription>
            </Field>
            <Field>
              <FieldLabel>{t('widgetStyle')}</FieldLabel>
              <ToggleGroup
                type='single'
                variant='outline'
                value={config.widget}
                onValueChange={(widget: PoWPageConfig['widget']) => {
                  if (widget) setConfig((prev) => ({ ...prev, widget }));
                }}
              >
                <ToggleGroupItem value='progress'>
                  {t('progress')}
                </ToggleGroupItem>
                <ToggleGroupItem value='turnstile'>
                  {t('turnstile')}
                </ToggleGroupItem>
              </ToggleGroup>
            </Field>
            <div className='flex flex-wrap items-center gap-2'>
              <Button disabled={save.isPending} onClick={() => save.mutate()}>
                {save.isPending ? (
                  <Loader2 className='animate-spin' data-icon='inline-start' />
                ) : (
                  <Save data-icon='inline-start' />
                )}
                {t('save')}
              </Button>
              <Button
                variant='outline'
                onClick={() =>
                  setConfig((prev) => ({
                    ...prev,
                    html: powTemplates[prev.template],
                  }))
                }
              >
                {t('loadTemplate')}
              </Button>
              <Button
                variant='outline'
                onClick={() => setConfig((prev) => ({ ...prev, html: '' }))}
              >
                {t('useBuiltin')}
              </Button>
              <Button variant='link' asChild>
                <Link href='/proxy-routes'>{t('policySettings')}</Link>
              </Button>
            </div>
          </FieldGroup>
        </CardContent>
      </Card>
      <p className='text-sm text-muted-foreground'>
        {t('customHelp', { widget: '{{widget}}', host: '{{host}}' })}
      </p>
      <HtmlEditorWorkspace
        value={config.html ?? ''}
        onChange={(html) => setConfig((prev) => ({ ...prev, html }))}
        preview={preview}
        maxBytes={POW_HTML_MAX_BYTES}
        showPreviewLink={false}
        previewTitle={t('pagePreview')}
        previewSandbox='allow-scripts'
        footerHint={t('previewHelp')}
      />
    </div>
  );
}
