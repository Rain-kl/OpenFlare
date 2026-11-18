'use client';

import Link from 'next/link';
import { useState } from 'react';
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

import { useSaveResponsePageOption } from './use-save-response-page-option';

type PageEditorProps = {
  optionKey: string;
  savedHTML: string;
  template:
    | string
    | {
        defaultId: string;
        options: { id: string; label: string; html: string }[];
      };
  maxBytes: number;
  title: string;
  backPath: string;
  backLabel: string;
  help: string;
  savedMessage: string;
  preview: (html: string) => string;
  previewTitle: string;
  previewSandbox?: string;
  previewHint?: string;
  mergeHTML?: (latestOption: string, html: string) => string;
};

export function ResponsePageHtmlEditor({
  optionKey,
  savedHTML,
  template,
  maxBytes,
  title,
  backPath,
  backLabel,
  help,
  savedMessage,
  preview,
  previewTitle,
  previewSandbox,
  previewHint,
  mergeHTML,
}: PageEditorProps) {
  const t = useTranslations('responses');
  const tc = useTranslations('common');
  const [draft, setDraft] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string>();
  const templates = typeof template === 'string' ? undefined : template;
  const templateId = selectedId ?? templates?.defaultId;
  const templateHTML =
    typeof template === 'string'
      ? template
      : template.options.find(({ id }) => id === templateId)?.html;
  const html = draft ?? savedHTML;
  const valueToSave = html.trim() ? html : '';
  const save = useSaveResponsePageOption(optionKey, savedMessage);

  const saveHTML = () => {
    if (new TextEncoder().encode(valueToSave).length > maxBytes) {
      toast.error(t('htmlTooLarge'));
      return;
    }
    save.mutate(
      mergeHTML ? (latest) => mergeHTML(latest, valueToSave) : valueToSave,
    );
  };

  return (
    <div className='flex w-full flex-col gap-4 py-6 px-1'>
      <div className='flex items-center gap-3'>
        <Button variant='outline' size='icon' className='size-8' asChild>
          <Link href={backPath} aria-label={backLabel}>
            <ArrowLeft className='size-4' />
          </Link>
        </Button>
        <div className='flex items-center gap-2'>
          <FileCode2 className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>{title}</h1>
        </div>
      </div>
      <p className='text-sm text-muted-foreground'>{help}</p>

      <HtmlEditorWorkspace
        value={html}
        onChange={setDraft}
        preview={preview}
        previewTitle={previewTitle}
        previewSandbox={previewSandbox}
        maxBytes={maxBytes}
        showPreviewLink={false}
        footerHint={previewHint ?? null}
        toolbarRight={
          <>
            {templates && (
              <div className='flex items-center gap-1.5'>
                <span className='hidden text-[11px] text-muted-foreground sm:inline'>
                  {t('builtinTemplates')}
                </span>
                <Select value={templateId} onValueChange={setSelectedId}>
                  <SelectTrigger
                    aria-label={t('selectTemplate')}
                    className='h-7 w-40 bg-background text-[11px]'
                  >
                    <SelectValue placeholder={t('selectTemplate')} />
                  </SelectTrigger>
                  <SelectContent>
                    {templates.options.map(({ id, label }) => (
                      <SelectItem key={id} value={id} className='text-xs'>
                        {label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='h-7 px-2 text-[11px]'
              disabled={templateHTML === undefined}
              onClick={() => setDraft(templateHTML ?? null)}
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
              onClick={saveHTML}
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
