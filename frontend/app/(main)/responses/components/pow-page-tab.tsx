'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { Loader2, Save } from 'lucide-react';
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
import { POW_PAGE_KEY, readPoWPage } from '@/lib/openflare/pow-page';
import type { PoWPageConfig } from '@/lib/services/openflare';

import { ResponsePagePreviewCard } from './response-page-preview-card';
import { usePoWPagePreview } from './use-pow-page-preview';
import { useSaveResponsePageOption } from './use-save-response-page-option';

export function PoWPageTab({
  optionMap,
}: {
  optionMap: Record<string, string>;
}) {
  const t = useTranslations('pow');
  const preview = usePoWPagePreview();
  const saved = readPoWPage(optionMap[POW_PAGE_KEY]);
  const [draft, setDraft] = useState<
    Partial<Pick<PoWPageConfig, 'template' | 'widget'>>
  >({});
  const config = { ...saved, ...draft };
  const save = useSaveResponsePageOption(POW_PAGE_KEY, t('pageSaved'));
  const hasChanges =
    config.template !== saved.template || config.widget !== saved.widget;

  return (
    <div className='space-y-6'>
      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-start justify-between gap-4 space-y-0'>
          <div className='space-y-1.5'>
            <CardTitle className='text-base'>{t('pageSettings')}</CardTitle>
            <CardDescription>{t('pageHelp')}</CardDescription>
          </div>
          <Button
            size='sm'
            className='shrink-0'
            disabled={save.isPending || !hasChanges}
            onClick={() =>
              save.mutate(
                (latest) =>
                  JSON.stringify({ ...readPoWPage(latest), ...draft }),
                { onSuccess: () => setDraft({}) },
              )
            }
          >
            {save.isPending ? (
              <Loader2 className='size-3.5 animate-spin' />
            ) : (
              <Save className='size-3.5' />
            )}
            {t('save')}
          </Button>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel>{t('pageStyle')}</FieldLabel>
              <ToggleGroup
                type='single'
                variant='outline'
                disabled={save.isPending}
                value={config.template}
                onValueChange={(template: PoWPageConfig['template']) => {
                  if (template) setDraft((prev) => ({ ...prev, template }));
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
                disabled={save.isPending}
                value={config.widget}
                onValueChange={(widget: PoWPageConfig['widget']) => {
                  if (widget) setDraft((prev) => ({ ...prev, widget }));
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
          </FieldGroup>
        </CardContent>
      </Card>
      <ResponsePagePreviewCard
        title={t('pagePreview')}
        editLabel={t('editPage')}
        previewPath='/responses/pow/preview'
        editPath='/responses/pow/edit'
        html={preview(config)}
        sandbox='allow-scripts'
      />
    </div>
  );
}
