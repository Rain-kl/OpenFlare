'use client';

import {useMutation, useQueryClient} from '@tanstack/react-query';
import {useRef, useState} from 'react';
import {File as FileIcon, UploadCloud, X} from 'lucide-react';

import {AppModal} from '@/components/ui/app-modal';
import {activatePagesDeployment, uploadPagesDeployment,} from '@/features/pages/api/pages';
import {PrimaryButton, SecondaryButton,} from '@/features/shared/components/resource-primitives';
import {cn} from '@/lib/utils/cn';
import {deploymentsQueryKey, projectQueryKey, projectsQueryKey,} from '../utils';

interface PagesDeploymentUploadModalProps {
  isOpen: boolean;
  onClose: () => void;
  projectId: number;
}

function formatBytes(bytes: number, decimals = 2) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

export function PagesDeploymentUploadModal({
  isOpen,
  onClose,
  projectId,
}: PagesDeploymentUploadModalProps) {
  const queryClient = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [isDragActive, setIsDragActive] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const resetForm = () => {
    setFile(null);
    setIsDragActive(false);
    setUploadProgress(null);
    setErrorMessage(null);
    if (fileInputRef.current) {
      fileInputRef.current.value = '';
    }
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setIsDragActive(true);
    } else if (e.type === 'dragleave') {
      setIsDragActive(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      const droppedFile = e.dataTransfer.files[0];
      if (droppedFile.name.toLowerCase().endsWith('.zip')) {
        setFile(droppedFile);
        setErrorMessage(null);
      } else {
        setErrorMessage('仅支持 zip 格式的文件');
      }
    }
  };

  const uploadMutation = useMutation({
    mutationFn: async ({ shouldActivate }: { shouldActivate: boolean }) => {
      if (!file) {
        throw new Error('请选择 zip 文件');
      }
      setUploadProgress(0);
      setErrorMessage(null);

      const deployment = await uploadPagesDeployment(
        projectId,
        file,
        '',
        'index.html',
        (percent) => {
          setUploadProgress(percent);
        },
      );

      if (shouldActivate) {
        await activatePagesDeployment(projectId, deployment.id);
      }
      return deployment;
    },
    onSuccess: () => {
      resetForm();
      queryClient.invalidateQueries({
        queryKey: deploymentsQueryKey(projectId),
      });
      queryClient.invalidateQueries({
        queryKey: projectQueryKey(projectId),
      });
      queryClient.invalidateQueries({
        queryKey: projectsQueryKey,
      });
      onClose();
    },
    onError: (error) => {
      setUploadProgress(null);
      setErrorMessage(error instanceof Error ? error.message : '上传失败');
    },
  });

  const handleUploadOnly = () => {
    uploadMutation.mutate({ shouldActivate: false });
  };

  const handleUploadAndDeploy = () => {
    uploadMutation.mutate({ shouldActivate: true });
  };

  return (
    <AppModal
      isOpen={isOpen}
      onClose={handleClose}
      title="上传部署包"
      description="上传已构建的 zip 静态资源包。项目将使用配置的根目录与入口文件路径。"
      footer={
        <div className="flex flex-wrap justify-end gap-3">
          <SecondaryButton
            type="button"
            onClick={handleClose}
            disabled={uploadMutation.isPending}
          >
            取消
          </SecondaryButton>
          <SecondaryButton
            type="button"
            disabled={!file || uploadMutation.isPending}
            onClick={handleUploadOnly}
          >
            {uploadMutation.isPending &&
            !uploadMutation.variables?.shouldActivate
              ? `上传中 (${uploadProgress ?? 0}%)...`
              : '上传'}
          </SecondaryButton>
          <PrimaryButton
            type="button"
            disabled={!file || uploadMutation.isPending}
            onClick={handleUploadAndDeploy}
          >
            {uploadMutation.isPending &&
            uploadMutation.variables?.shouldActivate
              ? `上传并部署中 (${uploadProgress ?? 0}%)...`
              : '上传并部署'}
          </PrimaryButton>
        </div>
      }
    >
      <div className="space-y-4">
        {/* Hidden File Input */}
        <input
          ref={fileInputRef}
          type="file"
          accept=".zip,application/zip"
          onChange={(event) => {
            const selectedFile = event.target.files?.[0] ?? null;
            if (selectedFile) {
              if (selectedFile.name.toLowerCase().endsWith('.zip')) {
                setFile(selectedFile);
                setErrorMessage(null);
              } else {
                setErrorMessage('仅支持 zip 格式的文件');
              }
            }
          }}
          className="hidden"
        />

        <div className="block space-y-2">
          <span className="flex items-center gap-2 text-sm font-medium text-[var(--foreground-primary)]">
            <span>部署包</span>
          </span>

          <div
            onDragEnter={handleDrag}
            onDragOver={handleDrag}
            onDragLeave={handleDrag}
            onDrop={handleDrop}
            onClick={() => fileInputRef.current?.click()}
            className={cn(
              'group relative flex min-h-48 cursor-pointer flex-col items-center justify-center rounded-2xl border-2 border-dashed border-[var(--border-default)] bg-[var(--surface-elevated)] p-6 text-center transition-all duration-200 hover:border-[var(--brand-primary)] hover:bg-[var(--surface-hover)]',
              isDragActive &&
                'scale-[0.99] border-[var(--brand-primary)] bg-[var(--accent-soft)]/20 shadow-inner',
              file && 'border-solid border-[var(--brand-primary)]',
            )}
          >
            {file ? (
              <div
                className="flex flex-col items-center space-y-3"
                onClick={(e) => e.stopPropagation()}
              >
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-[var(--accent-soft)] text-[var(--brand-primary)] transition duration-200 group-hover:scale-110">
                  <FileIcon className="h-6 w-6" />
                </div>
                <div className="space-y-1">
                  <p className="max-w-[300px] truncate text-sm font-medium text-[var(--foreground-primary)]">
                    {file.name}
                  </p>
                  <p className="text-xs text-[var(--foreground-secondary)]">
                    大小: {formatBytes(file.size)}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => {
                      setFile(null);
                      if (fileInputRef.current) {
                        fileInputRef.current.value = '';
                      }
                    }}
                    className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-[var(--border-default)] bg-[var(--control-background)] px-3 text-xs font-medium text-[var(--foreground-primary)] transition hover:border-[var(--status-danger-border)] hover:bg-[var(--status-danger-soft)] hover:text-[var(--status-danger-foreground)]"
                  >
                    <X className="h-3 w-3" />
                    清除文件
                  </button>
                </div>
              </div>
            ) : (
              <div className="pointer-events-none flex flex-col items-center space-y-3">
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-[var(--surface-muted)] text-[var(--foreground-muted)] transition duration-200 group-hover:scale-110 group-hover:bg-[var(--accent-soft)] group-hover:text-[var(--brand-primary)]">
                  <UploadCloud className="h-6 w-6" />
                </div>
                <div className="space-y-1">
                  <p className="text-sm font-medium text-[var(--foreground-primary)]">
                    点击或拖拽 zip 部署包到此处
                  </p>
                  <p className="text-xs text-[var(--foreground-secondary)]">
                    仅支持 zip，Server 会校验文件数量、体积、路径逃逸和入口文件
                  </p>
                </div>
              </div>
            )}
          </div>
          {errorMessage && (
            <span className="mt-1 block text-xs text-[var(--status-danger-foreground)]">
              {errorMessage}
            </span>
          )}
        </div>

        {uploadProgress !== null && (
          <div className="space-y-2">
            <div className="flex items-center justify-between text-xs font-medium text-[var(--foreground-secondary)]">
              <span>上传进度</span>
              <span>{uploadProgress}%</span>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--surface-muted)]">
              <div
                className="h-full rounded-full bg-[var(--brand-primary)] transition-all duration-300 ease-out"
                style={{ width: `${uploadProgress}%` }}
              />
            </div>
          </div>
        )}
      </div>
    </AppModal>
  );
}
