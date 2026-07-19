'use client';

import { useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { UploadCloud } from 'lucide-react';
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
      if (!file) throw new Error('请选择部署包');
      return PagesService.uploadDeployment(projectId, {
        file,
        onProgress: setUploadProgress,
      });
    },
    onSuccess: async () => {
      toast.success('部署包上传成功');
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
      toast.error(error instanceof Error ? error.message : '上传失败');
      setUploadProgress(null);
    },
  });

  const handleFileSelect = (selected: File | null) => {
    if (!selected) return;
    if (!isSupportedPagesPackage(selected.name)) {
      toast.error('仅支持 zip、tar.gz、tar.xz、tar.bz2、tar、7z 格式的部署包');
      return;
    }
    setFile(selected);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>上传部署包</DialogTitle>
          <DialogDescription>
            上传不可变的静态资源压缩包，完成后可在部署历史中激活。
          </DialogDescription>
        </DialogHeader>

        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='pages-package'>本地部署包</FieldLabel>
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
              <span className='text-sm font-medium'>
                拖拽部署包到此处，或点击选择文件
              </span>
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
                已选择 {file.name}（{formatBytes(file.size)}）
              </FieldDescription>
            ) : (
              <FieldDescription>请选择一个受支持的压缩包。</FieldDescription>
            )}
          </Field>

          <Field>
            <FieldLabel>部署入口</FieldLabel>
            <div className='rounded-md border bg-muted/20 px-3 py-2 font-mono text-sm'>
              {pagesEntryPath(rootDir, entryFile)}
            </div>
            <FieldDescription>
              入口来自项目设置；部署包上传不会覆盖该配置。
            </FieldDescription>
          </Field>

          {uploadProgress !== null ? (
            <Field>
              <div className='flex items-center justify-between text-xs text-muted-foreground'>
                <span>
                  {uploadProgress >= 100 ? '服务端处理中' : '上传进度'}
                </span>
                <span>
                  {uploadProgress >= 100 ? '请稍候' : `${uploadProgress}%`}
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
            取消
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
            {uploadMutation.isPending ? '上传中...' : '上传并创建部署'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
