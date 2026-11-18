'use client';

import Link from 'next/link';
import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  FileCode2,
  Loader2,
  RotateCcw,
  Save,
  Sparkles,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { HtmlEditorWorkspace } from '@/components/common/html-editor-workspace';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { OptionService, type OptionItem } from '@/lib/services/openflare';

import {
  invalidateResponseQueries,
  OPTIONS_QUERY_KEY,
} from '../../components/shared';
import {
  defaultSiteAuthHTML,
  previewSiteAuthHTML,
  SITE_AUTH_HTML_KEY,
  SITE_AUTH_HTML_MAX_BYTES,
} from '../../components/site-auth-page-html';
import { SiteAuthPageContent } from '../site-auth-page-content';

function SiteAuthEditor({ savedHTML }: { savedHTML: string }) {
  const t = useTranslations('responses');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<string | null>(null);
  const html = draft ?? savedHTML;
  const valueToSave = html.trim() ? html : '';
  const save = useMutation({
    mutationFn: async (value: string) => {
      if (new TextEncoder().encode(value).length > SITE_AUTH_HTML_MAX_BYTES) {
        throw new Error(t('htmlTooLarge'));
      }
      await OptionService.updateBatch([{ key: SITE_AUTH_HTML_KEY, value }]);
    },
    onSuccess: async (_, value) => {
      queryClient.setQueryData<OptionItem[]>(OPTIONS_QUERY_KEY, (options) => [
        ...(options ?? []).filter(({ key }) => key !== SITE_AUTH_HTML_KEY),
        { key: SITE_AUTH_HTML_KEY, value },
      ]);
      toast.success(t('siteAuthSaved'));
      await invalidateResponseQueries(queryClient);
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <div className='flex w-full flex-col gap-4 py-6 px-1'>
      <div className='flex items-center gap-3'>
        <Button variant='outline' size='icon' className='size-8' asChild>
          <Link href='/responses?tab=site-auth' aria-label={t('siteAuthBack')}>
            <ArrowLeft className='size-4' />
          </Link>
        </Button>
        <div className='flex items-center gap-2'>
          <FileCode2 className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('siteAuthEdit')}
          </h1>
        </div>
      </div>
      <p className='text-sm text-muted-foreground'>
        {t('siteAuthFormContract')}
      </p>

      <HtmlEditorWorkspace
        value={html}
        onChange={setDraft}
        preview={previewSiteAuthHTML}
        previewTitle={t('siteAuthLivePreview')}
        maxBytes={SITE_AUTH_HTML_MAX_BYTES}
        showPreviewLink={false}
        footerHint={null}
        toolbarRight={
          <>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='h-7 px-2 text-[11px]'
              onClick={() => setDraft(defaultSiteAuthHTML)}
            >
              <Sparkles className='size-3.5' />
              {t('loadTemplate')}
            </Button>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className='h-7 px-2 text-[11px]'
                  disabled={!html.trim()}
                >
                  <RotateCcw className='size-3.5' />
                  {t('restoreDefault')}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t('restoreTitle')}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t('restoreOfflineDesc')}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
                  <AlertDialogAction onClick={() => setDraft('')}>
                    {t('confirmRestore')}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
            <Button
              type='button'
              size='sm'
              className='h-7 px-3 text-[11px]'
              disabled={save.isPending || valueToSave === savedHTML}
              onClick={() => save.mutate(valueToSave)}
            >
              {save.isPending ? (
                <Loader2 className='size-3 animate-spin' />
              ) : (
                <Save className='size-3' />
              )}
              {tc('save')}
            </Button>
          </>
        }
      />
    </div>
  );
}

export default function SiteAuthPageEditPage() {
  return (
    <SiteAuthPageContent>
      {(html) => <SiteAuthEditor savedHTML={html} />}
    </SiteAuthPageContent>
  );
}
