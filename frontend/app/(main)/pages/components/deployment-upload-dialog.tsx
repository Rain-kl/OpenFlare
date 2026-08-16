'use client';

import { useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { UploadCloud } from 'lucide-react';
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
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Progress } from '@/components/ui/progress';
import { Spinner } from '@/components/ui/spinner';
import { PagesService } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import {
  deploymentsQueryKey,
  formatBytes,
  projectQueryKey,
  projectsQueryKey,
} from './pages-utils';

const PAGES_PACKAGE_ACCEPT =
  '.zip,.tar.gz,.tgz,.tar.xz,.txz,.tar.bz2,.tbz2,.tbz,.tar,.7z';

const PAGES_PACKAGE_EXTENSIONS = [
  '.zip',
  '.tar.gz',
  '.tgz',
  '.tar.xz',
  '.txz',
  '.tar.bz2',
  '.tbz2',
  '.tbz',
  '.tar',
  '.7z',
] as const;

function isSupportedPagesPackage(fileName: string) {
  const lower = fileName.toLowerCase();
  return PAGES_PACKAGE_EXTENSIONS.some((extension) =>
    lower.endsWith(extension),
  );
}

export function pagesEntryPath(rootDir: string, entryFile: string) {
  const root = rootDir.trim().replace(/^\/+|\/+$/g, '');
  const entry = entryFile.trim().replace(/^\/+/, '');
  return root ? `${root}/${entry}` : entry;
}

interface DeploymentUploadDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: number;
  rootDir: string;
  entryFile: string;
}

export function DeploymentUploadDialog({
  open,
  onOpenChange,
  projectId,
  rootDir,
  entryFile,
}: DeploymentUploadDialogProps) {
  const t = useTranslations('pages.upload');
  const tCommon = useTranslations('common');
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [isDragActive, setIsDragActive] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);

  const resetForm = () => {
    setFile(null);
    setIsDragActive(false);
    setUploadProgress(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) resetForm();
    onOpenChange(nextOpen);
  };

  const uploadMutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error(t('selectFile'));
      return PagesService.uploadDeployment(projectId, {
        file,
        onProgress: setUploadProgress,
      });
    },
    onSuccess: async () => {
      toast.success(t('success'));
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: deploymentsQueryKey(projectId),
        }),
        queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) }),
        queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
      ]);
      handleOpenChange(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('failed'));
      setUploadProgress(null);
    },
  });

  const handleFileSelect = (selected: File | null) => {
    if (!selected) return;
    if (!isSupportedPagesPackage(selected.name)) {
      toast.error(t('unsupported'));
      return;
    }
    setFile(selected);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('title')}</DialogTitle>
          <DialogDescription>{t('description')}</DialogDescription>
        </DialogHeader>

        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='pages-package'>{t('localPackage')}</FieldLabel>
            <button
              type='button'
              className={cn(
                'flex min-h-52 w-full flex-col items-center justify-center gap-3 rounded-lg border border-dashed p-8 text-center transition-colors',
                isDragActive ? 'border-primary bg-primary/5' : 'bg-muted/20',
              )}
              onClick={() => fileInputRef.current?.click()}
              onDragEnter={(event) => {
                event.preventDefault();
                setIsDragActive(true);
              }}
              onDragOver={(event) => event.preventDefault()}
              onDragLeave={(event) => {
                event.preventDefault();
                setIsDragActive(false);
              }}
              onDrop={(event) => {
                event.preventDefault();
                setIsDragActive(false);
                handleFileSelect(event.dataTransfer.files[0] ?? null);
              }}
            >
              <UploadCloud className='size-8 text-muted-foreground' />
              <span className='text-sm font-medium'>{t('dropHint')}</span>
              <span className='text-xs text-muted-foreground'>
                zip、tar.gz、tar.xz、tar.bz2、tar、7z
              </span>
            </button>
            <input
              ref={fileInputRef}
              id='pages-package'
              type='file'
              accept={PAGES_PACKAGE_ACCEPT}
              className='hidden'
              onChange={(event) =>
                handleFileSelect(event.target.files?.[0] ?? null)
              }
            />
            {file ? (
              <FieldDescription>
                {t('selected', {
                  name: file.name,
                  size: formatBytes(file.size),
                })}
              </FieldDescription>
            ) : (
              <FieldDescription>{t('chooseArchive')}</FieldDescription>
            )}
          </Field>

          <Field>
            <FieldLabel>{t('entry')}</FieldLabel>
            <div className='rounded-md border bg-muted/20 px-3 py-2 font-mono text-sm'>
              {pagesEntryPath(rootDir, entryFile)}
            </div>
            <FieldDescription>{t('entryHint')}</FieldDescription>
          </Field>

          {uploadProgress !== null ? (
            <Field>
              <div className='flex items-center justify-between text-xs text-muted-foreground'>
                <span>
                  {uploadProgress >= 100 ? t('processing') : t('progress')}
                </span>
                <span>
                  {uploadProgress >= 100
                    ? t('pleaseWait')
                    : `${uploadProgress}%`}
                </span>
              </div>
              <Progress value={Math.min(uploadProgress, 100)} />
            </Field>
          ) : null}
        </FieldGroup>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleOpenChange(false)}
          >
            {tCommon('cancel')}
          </Button>
          <Button
            type='button'
            disabled={!file || uploadMutation.isPending}
            onClick={() => uploadMutation.mutate()}
          >
            {uploadMutation.isPending ? (
              <Spinner data-icon='inline-start' />
            ) : (
              <UploadCloud data-icon='inline-start' />
            )}
            {uploadMutation.isPending ? t('uploading') : t('submit')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
