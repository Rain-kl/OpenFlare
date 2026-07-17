'use client';

import { useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2, UploadCloud } from 'lucide-react';
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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Progress } from '@/components/ui/progress';
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
  return PAGES_PACKAGE_EXTENSIONS.some((ext) => lower.endsWith(ext));
}

interface DeploymentUploadDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: number;
}

export function DeploymentUploadDialog({
  open,
  onOpenChange,
  projectId,
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

  const handleClose = (nextOpen: boolean) => {
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
      handleClose(false);
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
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>上传部署包</DialogTitle>
          <DialogDescription>
            上传已构建的静态资源压缩包（zip / tar.gz / tar.xz / tar.bz2 / tar /
            7z），部署后可在列表中激活。
          </DialogDescription>
        </DialogHeader>

        <div
          className={cn(
            'rounded-lg border border-dashed p-8 text-center transition',
            isDragActive ? 'border-primary bg-primary/5' : 'bg-muted/20',
          )}
          onDragEnter={(e) => {
            e.preventDefault();
            setIsDragActive(true);
          }}
          onDragOver={(e) => e.preventDefault()}
          onDragLeave={(e) => {
            e.preventDefault();
            setIsDragActive(false);
          }}
          onDrop={(e) => {
            e.preventDefault();
            setIsDragActive(false);
            handleFileSelect(e.dataTransfer.files[0] ?? null);
          }}
        >
          <UploadCloud className='size-8 mx-auto text-muted-foreground' />
          <p className='mt-3 text-sm'>拖拽部署包到此处，或点击选择文件</p>
          <p className='mt-1 text-xs text-muted-foreground'>
            支持 zip、tar.gz、tar.xz、tar.bz2、tar、7z
          </p>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='mt-3'
            onClick={() => fileInputRef.current?.click()}
          >
            选择文件
          </Button>
          <input
            ref={fileInputRef}
            type='file'
            accept={PAGES_PACKAGE_ACCEPT}
            className='hidden'
            onChange={(e) => handleFileSelect(e.target.files?.[0] ?? null)}
          />
        </div>

        {file ? (
          <div className='rounded-lg border border-dashed px-4 py-3 text-sm'>
            <p className='font-medium'>{file.name}</p>
            <p className='text-xs text-muted-foreground mt-1'>
              {formatBytes(file.size)}
            </p>
          </div>
        ) : null}

        {uploadProgress !== null ? (
          <div className='space-y-1.5'>
            <div className='flex justify-between text-xs text-muted-foreground'>
              <span>{uploadProgress >= 100 ? '服务端处理中' : '上传进度'}</span>
              <span>
                {uploadProgress >= 100 ? '请稍候' : `${uploadProgress}%`}
              </span>
            </div>
            <Progress value={uploadProgress >= 100 ? 100 : uploadProgress} />
          </div>
        ) : null}

        <div className='space-y-1.5'>
          <Label htmlFor='entryFile'>入口文件</Label>
          <Input id='entryFile' defaultValue='index.html' disabled />
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => handleClose(false)}>
            取消
          </Button>
          <Button
            onClick={() => uploadMutation.mutate()}
            disabled={!file || uploadMutation.isPending}
          >
            {uploadMutation.isPending ? (
              <>
                <Loader2 className='size-4 animate-spin mr-1' />
                上传中...
              </>
            ) : (
              '上传并创建部署'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
