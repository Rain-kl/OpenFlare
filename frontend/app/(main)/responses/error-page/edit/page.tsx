'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  FileCode2,
  Loader2,
  RotateCcw,
  Save,
  Sparkles,
} from 'lucide-react';
import { toast } from 'sonner';

import { useAuth } from '@/components/providers/auth-provider';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { HtmlEditorWorkspace } from '@/components/common/html-editor-workspace';
import {
  ORIGIN_ERROR_PAGE_HTML_MAX_BYTES,
  previewOriginErrorPageHTML,
} from '@/lib/openflare/default-origin-error-page-html';
import {
  DEFAULT_ORIGIN_ERROR_PAGE_TEMPLATE_ID,
  getOriginErrorPageTemplate,
  ORIGIN_ERROR_PAGE_TEMPLATES,
} from '@/lib/openflare/origin-error-page-templates';
import { OptionService } from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import {
  invalidateResponseQueries,
  KEY_HTML,
  mapOptionsToErrorFields,
  OPTIONS_QUERY_KEY,
  optionsToMap,
} from '../../components/shared';

export default function ErrorPageEditPage() {
  const t = useTranslations('responses');
  const tc = useTranslations('common');
  const { user, loading: authLoading } = useAuth();
  const queryClient = useQueryClient();
  const [html, setHtml] = useState('');
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [templateId, setTemplateId] = useState(
    DEFAULT_ORIGIN_ERROR_PAGE_TEMPLATE_ID,
  );

  const optionsQuery = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  useEffect(() => {
    if (!optionsQuery.data) return;
    setHtml(mapOptionsToErrorFields(optionsToMap(optionsQuery.data)).html);
  }, [optionsQuery.data]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      const bytes = new TextEncoder().encode(html).length;
      if (bytes > ORIGIN_ERROR_PAGE_HTML_MAX_BYTES) {
        throw new Error(t('htmlTooLarge'));
      }
      await OptionService.updateBatch([{ key: KEY_HTML, value: html }]);
    },
    onSuccess: async () => {
      toast.success(t('errorHtmlSaved'));
      await invalidateResponseQueries(queryClient);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('saveFailed'));
    },
  });

  const loadSelectedTemplate = () => {
    const tmpl = getOriginErrorPageTemplate(templateId);
    if (!tmpl) {
      toast.error(t('templateNotFound'));
      return;
    }
    setHtml(tmpl.html);
    toast.success(t('templateLoaded', { name: tmpl.name }));
  };

  const restoreDefault = () => {
    setHtml('');
    setRestoreOpen(false);
    toast.success(t('errorRestored'));
  };

  if (authLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder
          icon={FileCode2}
          description={t('loadingPermission')}
        />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='w-full py-6 px-1'>
        <EmptyStateWithBorder
          icon={FileCode2}
          title={t('forbidden')}
          description={t('forbiddenEditError')}
        />
      </div>
    );
  }

  if (optionsQuery.isLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder
          icon={FileCode2}
          description={t('loadingErrorConfig')}
        />
      </div>
    );
  }

  if (optionsQuery.isError) {
    return (
      <div className='w-full py-6 px-1'>
        <ErrorInline
          message={
            optionsQuery.error instanceof Error
              ? optionsQuery.error.message
              : t('loadFailed')
          }
          onRetry={() => void optionsQuery.refetch()}
        />
      </div>
    );
  }

  return (
    <div className='flex flex-col gap-4 py-6 px-1'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex items-center gap-3'>
          <Button variant='outline' size='icon' className='h-8 w-8' asChild>
            <Link href='/responses?tab=error' aria-label={t('backToError')}>
              <ArrowLeft className='size-4' />
            </Link>
          </Button>
          <div className='flex items-center gap-2'>
            <FileCode2 className='size-5 text-primary' />
            <div>
              <h1 className='text-2xl font-semibold tracking-tight'>
                {t('editErrorHtml')}
              </h1>
            </div>
          </div>
        </div>
      </div>

      <HtmlEditorWorkspace
        value={html}
        onChange={setHtml}
        preview={previewOriginErrorPageHTML}
        toolbarRight={
          <>
            <div className='flex items-center gap-1.5'>
              <span className='text-[11px] text-muted-foreground hidden sm:inline'>
                {t('builtinTemplates')}
              </span>
              <Select value={templateId} onValueChange={setTemplateId}>
                <SelectTrigger className='h-7 w-[160px] text-[11px] bg-background'>
                  <SelectValue placeholder={t('selectTemplate')} />
                </SelectTrigger>
                <SelectContent>
                  {ORIGIN_ERROR_PAGE_TEMPLATES.map((tmpl) => (
                    <SelectItem
                      key={tmpl.id}
                      value={tmpl.id}
                      className='text-xs'
                    >
                      {tmpl.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                className='h-7 px-2 text-[11px]'
                onClick={loadSelectedTemplate}
              >
                <Sparkles className='size-3.5' />
                {t('loadTemplate')}
              </Button>
            </div>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='h-7 px-2 text-[11px]'
              onClick={() => setRestoreOpen(true)}
            >
              <RotateCcw className='size-3.5' />
              {t('restoreDefault')}
            </Button>
            <Button
              size='sm'
              className='h-7 px-3 text-[11px]'
              disabled={saveMutation.isPending}
              onClick={() => saveMutation.mutate()}
            >
              {saveMutation.isPending ? (
                <Loader2 className='size-3 animate-spin' />
              ) : (
                <Save className='size-3' />
              )}
              {tc('save')}
            </Button>
          </>
        }
      />

      <AlertDialog open={restoreOpen} onOpenChange={setRestoreOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('restoreTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('restoreErrorDesc')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={restoreDefault}>
              {t('confirmRestore')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
