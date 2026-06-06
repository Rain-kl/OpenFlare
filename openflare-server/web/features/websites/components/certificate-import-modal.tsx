'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState, type FormEvent } from 'react';
import { useForm } from 'react-hook-form';

import { InlineMessage } from '@/components/feedback/inline-message';
import { AppCard } from '@/components/ui/app-card';
import { AppModal } from '@/components/ui/app-modal';
import {
  createTlsCertificate,
  importTlsCertificateFiles,
} from '@/features/tls-certificates/api/tls-certificates';
import type { TlsCertificateItem } from '@/features/tls-certificates/types';
import {
  defaultFileImportValues,
  defaultManualImportValues,
  type FileImportFormValues,
  manualImportSchema,
  type ManualImportFormValues,
} from '@/features/websites/schemas';
import {
  getErrorMessage,
  toFilePayload,
  toManualPayload,
} from '@/features/websites/utils';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceTextarea,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';

type FeedbackState = {
  tone: 'success' | 'danger';
  message: string;
};

interface CertificateImportModalProps {
  isOpen: boolean;
  onClose: () => void;
  onImported?: (certificate: TlsCertificateItem) => void;
}

export function CertificateImportModal({
  isOpen,
  onClose,
  onImported,
}: CertificateImportModalProps) {
  const queryClient = useQueryClient();
  const [importMode, setImportMode] = useState<'manual' | 'file'>('manual');
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [fileForm, setFileForm] = useState<FileImportFormValues>(
    defaultFileImportValues,
  );
  const [certFile, setCertFile] = useState<File | null>(null);
  const [keyFile, setKeyFile] = useState<File | null>(null);
  const [fileInputNonce, setFileInputNonce] = useState(0);

  const manualForm = useForm<ManualImportFormValues>({
    resolver: zodResolver(manualImportSchema),
    defaultValues: defaultManualImportValues,
  });

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    setFeedback(null);
  }, [isOpen]);

  const resetFileForm = () => {
    setFileForm(defaultFileImportValues);
    setCertFile(null);
    setKeyFile(null);
    setFileInputNonce((value) => value + 1);
  };

  const resetAll = () => {
    setImportMode('manual');
    setFeedback(null);
    manualForm.reset(defaultManualImportValues);
    resetFileForm();
  };

  const invalidateCertificateQueries = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['tls-certificates'] }),
      queryClient.invalidateQueries({ queryKey: ['managed-domains'] }),
    ]);
  };

  const manualImportMutation = useMutation({
    mutationFn: async (values: ManualImportFormValues) =>
      createTlsCertificate(toManualPayload(values)),
    onSuccess: async (certificate) => {
      await invalidateCertificateQueries();
      onImported?.(certificate);
      resetAll();
      onClose();
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const fileImportMutation = useMutation({
    mutationFn: async (values: FileImportFormValues) =>
      importTlsCertificateFiles(toFilePayload(values, certFile, keyFile)),
    onSuccess: async (certificate) => {
      await invalidateCertificateQueries();
      onImported?.(certificate);
      resetAll();
      onClose();
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const handleManualSubmit = manualForm.handleSubmit((values) => {
    setFeedback(null);
    manualImportMutation.mutate(values);
  });

  const handleFileSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFeedback(null);
    fileImportMutation.mutate(fileForm);
  };

  const handleClose = () => {
    resetAll();
    onClose();
  };

  return (
    <AppModal
      isOpen={isOpen}
      onClose={handleClose}
      title="添加证书"
      description="支持手动粘贴 PEM 或上传证书文件。导入成功后可立即在网站表单里选择。"
      size="xl"
    >
      <div className="space-y-6">
        {feedback ? (
          <InlineMessage
            tone={feedback.tone}
            message={feedback.message}
            onClear={() => setFeedback(null)}
          />
        ) : null}

        <div className="inline-flex rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] p-1">
          <button
            type="button"
            onClick={() => setImportMode('manual')}
            className={`rounded-xl px-4 py-2 text-sm font-medium transition ${
              importMode === 'manual'
                ? 'bg-[var(--brand-primary)] text-[var(--foreground-inverse)]'
                : 'text-[var(--foreground-secondary)] hover:text-[var(--foreground-primary)]'
            }`}
          >
            手动导入
          </button>
          <button
            type="button"
            onClick={() => setImportMode('file')}
            className={`rounded-xl px-4 py-2 text-sm font-medium transition ${
              importMode === 'file'
                ? 'bg-[var(--brand-primary)] text-[var(--foreground-inverse)]'
                : 'text-[var(--foreground-secondary)] hover:text-[var(--foreground-primary)]'
            }`}
          >
            文件导入
          </button>
        </div>

        {importMode === 'manual' ? (
          <AppCard description="直接粘贴 PEM 证书和私钥内容，适合快速录入已有证书。">
            <form className="space-y-5" onSubmit={handleManualSubmit}>
              <div className="grid gap-4 md:grid-cols-2">
                <ResourceField
                  label="证书名称"
                  error={manualForm.formState.errors.name?.message}
                >
                  <ResourceInput
                    placeholder="example-com"
                    {...manualForm.register('name')}
                  />
                </ResourceField>
                <ResourceField
                  label="备注"
                  hint="可选，用于记录证书用途或来源。"
                  error={manualForm.formState.errors.remark?.message}
                >
                  <ResourceInput
                    placeholder="例如：主站生产证书"
                    {...manualForm.register('remark')}
                  />
                </ResourceField>
              </div>

              <ResourceField
                label="证书 PEM"
                error={manualForm.formState.errors.cert_pem?.message}
              >
                <ResourceTextarea
                  placeholder="-----BEGIN CERTIFICATE-----"
                  className="min-h-40 font-mono text-xs"
                  {...manualForm.register('cert_pem')}
                />
              </ResourceField>

              <ResourceField
                label="私钥 PEM"
                error={manualForm.formState.errors.key_pem?.message}
              >
                <ResourceTextarea
                  placeholder="-----BEGIN PRIVATE KEY-----"
                  className="min-h-40 font-mono text-xs"
                  {...manualForm.register('key_pem')}
                />
              </ResourceField>

              <PrimaryButton
                type="submit"
                disabled={manualImportMutation.isPending}
              >
                {manualImportMutation.isPending ? '导入中...' : '导入证书'}
              </PrimaryButton>
            </form>
          </AppCard>
        ) : (
          <AppCard description="上传证书文件和私钥文件，适合直接复用现有 PEM 文件。">
            <form className="space-y-5" onSubmit={handleFileSubmit}>
              <div className="grid gap-4 md:grid-cols-2">
                <ResourceField label="证书名称">
                  <ResourceInput
                    value={fileForm.name}
                    onChange={(event) =>
                      setFileForm((current) => ({
                        ...current,
                        name: event.target.value,
                      }))
                    }
                    placeholder="wildcard-example"
                  />
                </ResourceField>
                <ResourceField label="备注">
                  <ResourceInput
                    value={fileForm.remark}
                    onChange={(event) =>
                      setFileForm((current) => ({
                        ...current,
                        remark: event.target.value,
                      }))
                    }
                    placeholder="例如：泛域名生产证书"
                  />
                </ResourceField>
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <ResourceField
                  label="证书文件"
                  hint={
                    certFile
                      ? `已选择：${certFile.name}`
                      : '请选择 PEM/CRT 文件'
                  }
                >
                  <ResourceInput
                    key={`cert-${fileInputNonce}`}
                    type="file"
                    accept=".pem,.crt,.cer"
                    onChange={(event) =>
                      setCertFile(event.target.files?.[0] ?? null)
                    }
                  />
                </ResourceField>
                <ResourceField
                  label="私钥文件"
                  hint={
                    keyFile ? `已选择：${keyFile.name}` : '请选择 KEY/PEM 文件'
                  }
                >
                  <ResourceInput
                    key={`key-${fileInputNonce}`}
                    type="file"
                    accept=".key,.pem"
                    onChange={(event) =>
                      setKeyFile(event.target.files?.[0] ?? null)
                    }
                  />
                </ResourceField>
              </div>

              <div className="flex flex-wrap gap-3">
                <PrimaryButton
                  type="submit"
                  disabled={fileImportMutation.isPending}
                >
                  {fileImportMutation.isPending ? '上传中...' : '上传文件'}
                </PrimaryButton>
                <SecondaryButton
                  type="button"
                  onClick={resetFileForm}
                  disabled={fileImportMutation.isPending}
                >
                  清空文件
                </SecondaryButton>
              </div>
            </form>
          </AppCard>
        )}
      </div>
    </AppModal>
  );
}
