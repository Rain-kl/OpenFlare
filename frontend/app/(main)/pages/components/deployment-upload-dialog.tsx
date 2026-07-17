'use client';

import { useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Link2, Loader2, UploadCloud } from 'lucide-react';
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
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
  const [mode, setMode] = useState<'file' | 'url'>('file');
  const [file, setFile] = useState<File | null>(null);
  const [packageURL, setPackageURL] = useState('');
  const [isDragActive, setIsDragActive] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);

  const resetForm = () => {
    setFile(null);
    setPackageURL('');
    setIsDragActive(false);
    setUploadProgress(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const handleClose = (nextOpen: boolean) => {
    if (!nextOpen) resetForm();
    onOpenChange(nextOpen);
  };

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: deploymentsQueryKey(projectId),
      }),
      queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) }),
      queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
    ]);
  };

  const uploadFileMutation = useMutation({
    mutationFn: () => {
      if (!file) throw new Error('请选择部署包');
      return PagesService.uploadDeployment(projectId, {
        file,
        onProgress: setUploadProgress,
      });
    },
    onSuccess: async () => {
      toast.success('部署包上传成功');
      await invalidate();
      handleClose(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '上传失败');
      setUploadProgress(null);
    },
  });

  const uploadURLMutation = useMutation({
    mutationFn: () => {
      const url = packageURL.trim();
      if (!url) throw new Error('请填写部署包下载链接');
      if (!/^https?:\/\//i.test(url)) {
        throw new Error('链接必须以 http:// 或 https:// 开头');
      }
      return PagesService.uploadDeploymentFromURL(projectId, { url });
    },
    onSuccess: async () => {
      toast.success('已从链接下载并创建部署');
      await invalidate();
      handleClose(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '从链接导入失败');
    },
  });

  const isPending = uploadFileMutation.isPending || uploadURLMutation.isPending;

  const handleFileSelect = (selected: File | null) => {
    if (!selected) return;
    if (!isSupportedPagesPackage(selected.name)) {
      toast.error('仅支持 zip、tar.gz、tar.xz、tar.bz2、tar、7z 格式的部署包');
      return;
    }
    setFile(selected);
  };

  const handleSubmit = () => {
    if (mode === 'file') {
      uploadFileMutation.mutate();
      return;
    }
    uploadURLMutation.mutate();
  };

  const canSubmit =
    mode === 'file' ? Boolean(file) : packageURL.trim().length > 0;

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>上传部署包</DialogTitle>
          <DialogDescription>
            支持本地上传或从 URL 下载静态资源压缩包（zip / tar.gz / tar.xz /
            tar.bz2 / tar / 7z），创建部署后可在列表中激活。
          </DialogDescription>
        </DialogHeader>

        <Tabs
          value={mode}
          onValueChange={(value) => setMode(value as 'file' | 'url')}
          className='w-full'
        >
          <TabsList className='grid w-full grid-cols-2'>
            <TabsTrigger value='file'>本地上传</TabsTrigger>
            <TabsTrigger value='url'>从 URL 下载</TabsTrigger>
          </TabsList>

          <TabsContent value='file' className='space-y-4 mt-4'>
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
                  <span>
                    {uploadProgress >= 100 ? '服务端处理中' : '上传进度'}
                  </span>
                  <span>
                    {uploadProgress >= 100 ? '请稍候' : `${uploadProgress}%`}
                  </span>
                </div>
                <Progress
                  value={uploadProgress >= 100 ? 100 : uploadProgress}
                />
              </div>
            ) : null}
          </TabsContent>

          <TabsContent value='url' className='space-y-4 mt-4'>
            <div className='space-y-2'>
              <Label htmlFor='package-url'>部署包下载链接</Label>
              <div className='relative'>
                <Link2 className='absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
                <Input
                  id='package-url'
                  className='pl-9'
                  placeholder='https://example.com/dist/site.zip'
                  value={packageURL}
                  onChange={(e) => setPackageURL(e.target.value)}
                  disabled={isPending}
                />
              </div>
              <p className='text-xs text-muted-foreground'>
                服务端将使用浏览器环境请求头从该链接下载压缩包，支持内网地址与自签证书
                HTTPS。
              </p>
            </div>
          </TabsContent>
        </Tabs>

        <div className='space-y-1.5'>
          <Label htmlFor='entryFile'>入口文件</Label>
          <Input id='entryFile' defaultValue='index.html' disabled />
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => handleClose(false)}>
            取消
          </Button>
          <Button onClick={handleSubmit} disabled={!canSubmit || isPending}>
            {isPending ? (
              <>
                <Loader2 className='size-4 animate-spin mr-1' />
                {mode === 'url' ? '下载中...' : '上传中...'}
              </>
            ) : mode === 'url' ? (
              '下载并创建部署'
            ) : (
              '上传并创建部署'
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
