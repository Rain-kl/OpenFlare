'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { AppModal } from '@/components/ui/app-modal';
import {
  activatePagesDeployment,
  uploadPagesDeployment,
} from '@/features/pages/api/pages';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import {
  deploymentsQueryKey,
  projectQueryKey,
  projectsQueryKey,
} from '../utils';

interface PagesDeploymentUploadModalProps {
  isOpen: boolean;
  onClose: () => void;
  projectId: number;
}

export function PagesDeploymentUploadModal({
  isOpen,
  onClose,
  projectId,
}: PagesDeploymentUploadModalProps) {
  const queryClient = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [entryFile, setEntryFile] = useState('index.html');
  const [uploadProgress, setUploadProgress] = useState<number | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const resetForm = () => {
    setFile(null);
    setEntryFile('index.html');
    setUploadProgress(null);
    setErrorMessage(null);
  };

  const handleClose = () => {
    resetForm();
    onClose();
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
        entryFile,
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
      description="上传已构建的 zip 静态资源包，默认入口为 index.html。"
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
        <ResourceField
          label="部署包"
          hint="仅支持 zip，Server 会校验文件数量、体积、路径逃逸和入口文件。"
          error={errorMessage ?? undefined}
        >
          <ResourceInput
            type="file"
            accept=".zip,application/zip"
            onChange={(event) => setFile(event.target.files?.[0] ?? null)}
          />
        </ResourceField>
        <ResourceField label="入口文件">
          <ResourceInput
            value={entryFile}
            onChange={(event) => setEntryFile(event.target.value)}
          />
        </ResourceField>
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
