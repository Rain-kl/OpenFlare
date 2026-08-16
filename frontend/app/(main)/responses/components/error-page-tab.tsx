'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Expand, Loader2, Pencil, Save } from 'lucide-react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { TagsInput } from '@/components/ui/tags-input';
import { previewOriginErrorPageHTML } from '@/lib/openflare/default-origin-error-page-html';
import {
  validateStatusCodeTagMessage,
  validateStatusCodeTags,
} from '@/lib/openflare/status-code-tags';
import { OptionService } from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import {
  defaultErrorPageFields,
  invalidateResponseQueries,
  KEY_ENABLED,
  KEY_GET_ONLY,
  KEY_STATUS_CODES,
  mapOptionsToErrorFields,
  type ErrorPageFields,
} from './shared';

export function ErrorPageTab({
  optionMap,
}: {
  optionMap: Record<string, string>;
}) {
  const t = useTranslations('responses');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

  const [fields, setFields] = useState<ErrorPageFields>(defaultErrorPageFields);
  const [tagError, setTagError] = useState<string | null>(null);

  useEffect(() => {
    setFields(mapOptionsToErrorFields(optionMap));
    setTagError(null);
  }, [optionMap]);

  const previewSrcDoc = useMemo(
    () => previewOriginErrorPageHTML(fields.html),
    [fields.html],
  );

  /** 仅保存策略项 */
  const savePolicyMutation = useMutation({
    mutationFn: async () => {
      validateStatusCodeTags(fields.statusCodes);
      await OptionService.updateBatch([
        { key: KEY_ENABLED, value: String(fields.enabled) },
        { key: KEY_GET_ONLY, value: String(fields.getOnly) },
        {
          key: KEY_STATUS_CODES,
          value: JSON.stringify(fields.statusCodes),
        },
      ]);
    },
    onSuccess: async () => {
      toast.success(t('policySaved'));
      await invalidateResponseQueries(queryClient);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('saveFailed'));
    },
  });

  const handleValidateTag = (tag: string) => {
    const message = validateStatusCodeTagMessage(tag);
    if (message) {
      setTagError(message);
      toast.error(message);
      return message;
    }
    setTagError(null);
    return null;
  };

  return (
    <div className='space-y-6'>
      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-start justify-between gap-4 space-y-0'>
          <div className='space-y-1.5'>
            <CardTitle className='text-base'>{t('triggerPolicy')}</CardTitle>
            <CardDescription>{t('triggerPolicyDesc')}</CardDescription>
          </div>
          <Button
            size='sm'
            className='shrink-0'
            disabled={savePolicyMutation.isPending}
            onClick={() => savePolicyMutation.mutate()}
          >
            {savePolicyMutation.isPending ? (
              <Loader2 className='size-3.5 animate-spin' />
            ) : (
              <Save className='size-3.5' />
            )}
            {tc('save')}
          </Button>
        </CardHeader>
        <CardContent className='space-y-0 divide-y'>
          <div className='flex items-start justify-between gap-6 pb-5'>
            <div className='space-y-1'>
              <Label className='text-sm font-medium'>
                {t('enableErrorPage')}
              </Label>
              <p className='text-sm text-muted-foreground'>
                {t('enableErrorPageDesc')}
              </p>
            </div>
            <Switch
              checked={fields.enabled}
              onCheckedChange={(enabled) =>
                setFields((prev) => ({ ...prev, enabled }))
              }
              aria-label={t('enableErrorPage')}
              className='mt-0.5 shrink-0'
            />
          </div>

          <div className='flex items-start justify-between gap-6 py-5'>
            <div className='space-y-1'>
              <Label className='text-sm font-medium'>{t('getOnly')}</Label>
              <p className='text-sm text-muted-foreground'>
                {t('getOnlyDesc')}
              </p>
            </div>
            <Switch
              checked={fields.getOnly}
              disabled={!fields.enabled}
              onCheckedChange={(getOnly) =>
                setFields((prev) => ({ ...prev, getOnly }))
              }
              aria-label={t('getOnly')}
              className='mt-0.5 shrink-0'
            />
          </div>

          <div className='flex flex-col gap-3 pt-5'>
            <div className='space-y-1'>
              <Label
                htmlFor='origin-error-status-codes'
                className='text-sm font-medium'
              >
                {t('statusCodes')}
              </Label>
              <p className='text-sm text-muted-foreground'>
                {t('statusCodesDesc')}
              </p>
            </div>
            <TagsInput
              id='origin-error-status-codes'
              value={fields.statusCodes}
              onChange={(statusCodes) => {
                setFields((prev) => ({ ...prev, statusCodes }));
                setTagError(null);
              }}
              validateTag={handleValidateTag}
              placeholder={t('statusCodesPlaceholder')}
              aria-invalid={!!tagError}
              disabled={!fields.enabled}
            />
            {tagError ? (
              <p className='text-xs text-destructive'>{tagError}</p>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none overflow-hidden'>
        <CardHeader className='flex flex-row items-start justify-between gap-3 space-y-0'>
          <div className='space-y-1.5'>
            <CardTitle className='text-base'>{t('pagePreview')}</CardTitle>
          </div>
          <div className='flex shrink-0 flex-wrap gap-2'>
            <Button variant='outline' size='sm' asChild>
              <Link href='/responses/error-page/preview'>
                <Expand className='size-3.5' />
                {t('preview')}
              </Link>
            </Button>
            <Button size='sm' asChild>
              <Link href='/responses/error-page/edit'>
                <Pencil className='size-3.5' />
                {t('edit')}
              </Link>
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className='overflow-hidden rounded-md border bg-muted/30'>
            <iframe
              title={t('errorPagePreview')}
              sandbox=''
              srcDoc={previewSrcDoc}
              className='h-[32rem] w-full bg-background'
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
