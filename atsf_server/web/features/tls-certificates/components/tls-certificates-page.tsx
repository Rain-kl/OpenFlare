'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState, type FormEvent } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppModal } from '@/components/ui/app-modal';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  createTlsCertificate,
  deleteTlsCertificate,
  getTlsCertificates,
  importTlsCertificateFiles,
} from '@/features/tls-certificates/api/tls-certificates';
import type {
  TlsCertificateFileImportPayload,
  TlsCertificateItem,
  TlsCertificateMutationPayload,
} from '@/features/tls-certificates/types';
import {
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceTextarea,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

const tlsCertificatesQueryKey = ['tls-certificates', 'list'] as const;

const manualImportSchema = z.object({
  name: z.string().trim().min(1, '请输入证书名称').max(255, '证书名称不能超过 255 个字符'),
  cert_pem: z.string().trim().min(1, '请输入证书 PEM 内容'),
  key_pem: z.string().trim().min(1, '请输入私钥 PEM 内容'),
  remark: z.string().max(255, '备注不能超过 255 个字符'),
});

type ManualImportFormValues = z.infer<typeof manualImportSchema>;

type FileImportFormValues = {
  name: string;
  remark: string;
};

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

const defaultManualValues: ManualImportFormValues = {
  name: '',
  cert_pem: '',
  key_pem: '',
  remark: '',
};

const defaultFileValues: FileImportFormValues = {
  name: '',
  remark: '',
};

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function getCertificateStatus(certificate: TlsCertificateItem) {
  const expiresAt = new Date(certificate.not_after).getTime();
  const diffMs = expiresAt - Date.now();
  const days = Math.ceil(diffMs / (1000 * 60 * 60 * 24));

  if (Number.isNaN(expiresAt)) {
    return { label: '有效期未知', variant: 'warning' as const };
  }

  if (days < 0) {
    return { label: '已过期', variant: 'danger' as const };
  }

  if (days <= 30) {
    return { label: `${days} 天内到期`, variant: 'warning' as const };
  }

  return { label: '有效', variant: 'success' as const };
}

function toManualPayload(values: ManualImportFormValues): TlsCertificateMutationPayload {
  return {
    name: values.name.trim(),
    cert_pem: values.cert_pem.trim(),
    key_pem: values.key_pem.trim(),
    remark: values.remark.trim(),
  };
}

function toFilePayload(
  values: FileImportFormValues,
  certFile: File | null,
  keyFile: File | null,
): TlsCertificateFileImportPayload {
  if (!certFile || !keyFile) {
    throw new Error('请选择证书文件和私钥文件。');
  }

  return {
    name: values.name.trim(),
    remark: values.remark.trim(),
    certFile,
    keyFile,
  };
}

export function TlsCertificatesPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [isImportModalOpen, setIsImportModalOpen] = useState(false);
  const [fileForm, setFileForm] = useState<FileImportFormValues>(defaultFileValues);
  const [certFile, setCertFile] = useState<File | null>(null);
  const [keyFile, setKeyFile] = useState<File | null>(null);
  const [fileInputNonce, setFileInputNonce] = useState(0);

  const manualForm = useForm<ManualImportFormValues>({
    resolver: zodResolver(manualImportSchema),
    defaultValues: defaultManualValues,
  });

  const certificatesQuery = useQuery({
    queryKey: tlsCertificatesQueryKey,
    queryFn: getTlsCertificates,
  });

  const manualImportMutation = useMutation({
    mutationFn: async (values: ManualImportFormValues) => createTlsCertificate(toManualPayload(values)),
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '证书已导入。' });
      setIsImportModalOpen(false);
      manualForm.reset(defaultManualValues);
      await queryClient.invalidateQueries({ queryKey: tlsCertificatesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const fileImportMutation = useMutation({
    mutationFn: async (values: FileImportFormValues) =>
      importTlsCertificateFiles(toFilePayload(values, certFile, keyFile)),
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '证书文件已导入。' });
      setIsImportModalOpen(false);
      setFileForm(defaultFileValues);
      setCertFile(null);
      setKeyFile(null);
      setFileInputNonce((value) => value + 1);
      await queryClient.invalidateQueries({ queryKey: tlsCertificatesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteTlsCertificate,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '证书已删除。' });
      await queryClient.invalidateQueries({ queryKey: tlsCertificatesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const certificates = useMemo(() => certificatesQuery.data ?? [], [certificatesQuery.data]);

  const handleManualSubmit = manualForm.handleSubmit((values) => {
    setFeedback(null);
    manualImportMutation.mutate(values);
  });

  const handleFileSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFeedback(null);
    fileImportMutation.mutate(fileForm);
  };

  const handleDelete = (certificate: TlsCertificateItem) => {
    if (!window.confirm(`确认删除证书 ${certificate.name} 吗？`)) {
      return;
    }

    setFeedback(null);
    deleteMutation.mutate(certificate.id);
  };

  const handleCloseImportModal = () => {
    setIsImportModalOpen(false);
  };

  return (
    <>
    <div className='space-y-6'>
      <PageHeader
        title='TLS 证书'
        description='支持手动粘贴 PEM 导入和文件上传导入，并展示证书有效期与到期风险。'
        action={
          <PrimaryButton type='button' onClick={() => setIsImportModalOpen(true)}>
            导入证书
          </PrimaryButton>
        }
      />

      {feedback ? <InlineMessage tone={feedback.tone} message={feedback.message} /> : null}

      <AppCard
        title='证书列表'
        description='展示证书有效期、到期状态与最近更新时间。删除前会提示确认。'
        action={
          <SecondaryButton
            type='button'
            onClick={() => void queryClient.invalidateQueries({ queryKey: tlsCertificatesQueryKey })}
          >
            刷新
          </SecondaryButton>
        }
      >
        {certificatesQuery.isLoading ? (
          <LoadingState />
        ) : certificatesQuery.isError ? (
          <ErrorState title='证书列表加载失败' description={getErrorMessage(certificatesQuery.error)} />
        ) : certificates.length === 0 ? (
          <EmptyState title='暂无证书' description='请先导入至少一张证书，再为 HTTPS 规则或域名绑定使用。' />
        ) : (
          <div className='overflow-x-auto'>
            <table className='min-w-full divide-y divide-[var(--border-default)] text-left text-sm'>
              <thead>
                <tr className='text-[var(--foreground-secondary)]'>
                  <th className='px-3 py-3 font-medium'>名称</th>
                  <th className='px-3 py-3 font-medium'>状态</th>
                  <th className='px-3 py-3 font-medium'>有效期</th>
                  <th className='px-3 py-3 font-medium'>备注</th>
                  <th className='px-3 py-3 font-medium'>更新时间</th>
                  <th className='px-3 py-3 font-medium'>操作</th>
                </tr>
              </thead>
              <tbody className='divide-y divide-[var(--border-default)]'>
                {certificates.map((certificate) => {
                  const status = getCertificateStatus(certificate);

                  return (
                    <tr key={certificate.id} className='align-top'>
                      <td className='px-3 py-4 font-medium text-[var(--foreground-primary)]'>{certificate.name}</td>
                      <td className='px-3 py-4'>
                        <StatusBadge label={status.label} variant={status.variant} />
                      </td>
                      <td className='px-3 py-4 text-[var(--foreground-secondary)]'>
                        <div className='space-y-1'>
                          <p>{formatDateTime(certificate.not_before)}</p>
                          <p>{formatDateTime(certificate.not_after)}</p>
                        </div>
                      </td>
                      <td className='px-3 py-4 text-[var(--foreground-secondary)]'>{certificate.remark || '—'}</td>
                      <td className='px-3 py-4 text-[var(--foreground-secondary)]'>
                        {formatDateTime(certificate.updated_at)}
                      </td>
                      <td className='px-3 py-4'>
                        <DangerButton
                          type='button'
                          onClick={() => handleDelete(certificate)}
                          disabled={deleteMutation.isPending}
                          className='px-3 py-2 text-xs'
                        >
                          删除
                        </DangerButton>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </AppCard>
    </div>
    <AppModal
      isOpen={isImportModalOpen}
      onClose={handleCloseImportModal}
      title='导入证书'
      description='手动粘贴和文件导入都保留在同一个弹窗中，避免压缩主列表空间。'
      size='xl'
    >
      <div className='grid gap-6 xl:grid-cols-2'>
        <AppCard title='手动导入' description='直接粘贴 PEM 证书和私钥内容，适合快速录入已有证书。'>
          <form className='space-y-5' onSubmit={handleManualSubmit}>
            <div className='grid gap-4 md:grid-cols-2'>
              <ResourceField label='证书名称' error={manualForm.formState.errors.name?.message}>
                <ResourceInput placeholder='example-com' {...manualForm.register('name')} />
              </ResourceField>
              <ResourceField
                label='备注'
                hint='可选，用于记录证书用途或来源。'
                error={manualForm.formState.errors.remark?.message}
              >
                <ResourceInput placeholder='例如：主站生产证书' {...manualForm.register('remark')} />
              </ResourceField>
            </div>

            <ResourceField label='证书 PEM' error={manualForm.formState.errors.cert_pem?.message}>
              <ResourceTextarea
                placeholder='-----BEGIN CERTIFICATE-----'
                className='min-h-40 font-mono text-xs'
                {...manualForm.register('cert_pem')}
              />
            </ResourceField>

            <ResourceField label='私钥 PEM' error={manualForm.formState.errors.key_pem?.message}>
              <ResourceTextarea
                placeholder='-----BEGIN PRIVATE KEY-----'
                className='min-h-40 font-mono text-xs'
                {...manualForm.register('key_pem')}
              />
            </ResourceField>

            <PrimaryButton type='submit' disabled={manualImportMutation.isPending}>
              {manualImportMutation.isPending ? '导入中...' : '导入证书'}
            </PrimaryButton>
          </form>
        </AppCard>

        <AppCard title='文件导入' description='上传证书文件和私钥文件，适合直接复用现有 PEM 文件。'>
          <form className='space-y-5' onSubmit={handleFileSubmit}>
            <div className='grid gap-4 md:grid-cols-2'>
              <ResourceField label='证书名称'>
                <ResourceInput
                  value={fileForm.name}
                  onChange={(event) =>
                    setFileForm((current) => ({ ...current, name: event.target.value }))
                  }
                  placeholder='wildcard-example'
                />
              </ResourceField>
              <ResourceField label='备注' hint='可选，用于记录部署环境或域名范围。'>
                <ResourceInput
                  value={fileForm.remark}
                  onChange={(event) =>
                    setFileForm((current) => ({ ...current, remark: event.target.value }))
                  }
                  placeholder='例如：泛域名生产证书'
                />
              </ResourceField>
            </div>

            <div className='grid gap-4 md:grid-cols-2'>
              <ResourceField
                label='证书文件'
                hint={certFile ? `已选择：${certFile.name}` : '请选择 PEM/CRT 文件'}
              >
                <ResourceInput
                  key={`cert-${fileInputNonce}`}
                  type='file'
                  accept='.pem,.crt,.cer'
                  onChange={(event) => setCertFile(event.target.files?.[0] ?? null)}
                />
              </ResourceField>
              <ResourceField
                label='私钥文件'
                hint={keyFile ? `已选择：${keyFile.name}` : '请选择 KEY/PEM 文件'}
              >
                <ResourceInput
                  key={`key-${fileInputNonce}`}
                  type='file'
                  accept='.key,.pem'
                  onChange={(event) => setKeyFile(event.target.files?.[0] ?? null)}
                />
              </ResourceField>
            </div>

            <div className='flex flex-wrap gap-3'>
              <PrimaryButton type='submit' disabled={fileImportMutation.isPending}>
                {fileImportMutation.isPending ? '上传中...' : '上传文件'}
              </PrimaryButton>
              <SecondaryButton
                type='button'
                onClick={() => {
                  setFileForm(defaultFileValues);
                  setCertFile(null);
                  setKeyFile(null);
                  setFileInputNonce((value) => value + 1);
                }}
                disabled={fileImportMutation.isPending}
              >
                清空文件
              </SecondaryButton>
            </div>
          </form>
        </AppCard>
      </div>
    </AppModal>
    </>
  );
}
