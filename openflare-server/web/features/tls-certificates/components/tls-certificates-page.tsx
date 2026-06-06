'use client';

import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  deleteTlsCertificate,
  getTlsCertificates,
  renewTlsCertificate,
} from '@/features/tls-certificates/api/tls-certificates';
import type { TlsCertificateItem } from '@/features/tls-certificates/types';
import { CertificateDetailModal } from '@/features/websites/components/certificate-detail-modal';
import { CertificateEditorModal } from '@/features/websites/components/certificate-editor-modal';
import { CertificateImportModal } from '@/features/websites/components/certificate-import-modal';
import { CertificateApplyModal } from '@/features/websites/components/certificate-apply-modal';
import {
  getCertificateStatus,
  getErrorMessage,
} from '@/features/websites/utils';
import {
  DangerButton,
  PrimaryButton,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

const certificatesQueryKey = ['tls-certificates', 'list'] as const;
type CertificateApplyMode = 'edit-acme' | 'convert-upload';

export function TlsCertificatesPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [isImportOpen, setIsImportOpen] = useState(false);
  const [isApplyOpen, setIsApplyOpen] = useState(false);
  const [selectedCertificateId, setSelectedCertificateId] = useState<
    number | null
  >(null);
  const [isDetailOpen, setIsDetailOpen] = useState(false);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [applyCertificate, setApplyCertificate] =
    useState<TlsCertificateItem | null>(null);
  const [applyMode, setApplyMode] = useState<CertificateApplyMode>('edit-acme');

  const certificatesQuery = useQuery({
    queryKey: certificatesQueryKey,
    queryFn: getTlsCertificates,
  });

  const deleteCertificateMutation = useMutation({
    mutationFn: deleteTlsCertificate,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '证书已删除。' });
      await queryClient.invalidateQueries({ queryKey: ['tls-certificates'] });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const renewCertificateMutation = useMutation({
    mutationFn: renewTlsCertificate,
    onSuccess: async (cert) => {
      setFeedback({
        tone: 'success',
        message: `证书 ${cert.name} 续期任务已提交。`,
      });
      await queryClient.invalidateQueries({ queryKey: ['tls-certificates'] });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const certificates = useMemo(
    () => certificatesQuery.data ?? [],
    [certificatesQuery.data],
  );

  const handleDeleteCertificate = (certificate: TlsCertificateItem) => {
    if (!window.confirm(`确认删除证书 ${certificate.name} 吗？`)) {
      return;
    }

    setFeedback(null);
    deleteCertificateMutation.mutate(certificate.id);
  };

  const handleRenewCertificate = (certificate: TlsCertificateItem) => {
    if (!window.confirm(`确认提交证书 ${certificate.name} 的续期申请吗？`)) {
      return;
    }

    setFeedback(null);
    renewCertificateMutation.mutate(certificate.id);
  };

  const handleOpenCertificateDetail = (certificate: TlsCertificateItem) => {
    setSelectedCertificateId(certificate.id);
    setIsDetailOpen(true);
  };

  const handleOpenCertificateEditor = (certificate: TlsCertificateItem) => {
    if (certificate.provider === 'acme') {
      setApplyMode('edit-acme');
      setApplyCertificate(certificate);
    } else {
      setSelectedCertificateId(certificate.id);
      setIsEditorOpen(true);
    }
  };

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="证书"
          description="统一查看、导入、编辑和删除已添加的 TLS 证书。"
          action={
            <div className="flex flex-wrap gap-3">
              <Link
                href="/website"
                className="inline-flex min-h-[46px] items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
              >
                返回网站
              </Link>
              <SecondaryButton
                type="button"
                onClick={() =>
                  void queryClient.invalidateQueries({
                    queryKey: ['tls-certificates'],
                  })
                }
              >
                刷新证书
              </SecondaryButton>
              <Link
                href="/website/dns-account"
                className="inline-flex min-h-[46px] items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
              >
                DNS 账号
              </Link>
              <PrimaryButton
                type="button"
                onClick={() => setIsImportOpen(true)}
              >
                导入证书
              </PrimaryButton>
              <PrimaryButton type="button" onClick={() => setIsApplyOpen(true)}>
                申请证书
              </PrimaryButton>
            </div>
          }
        />

        {feedback ? (
          <InlineMessage
            tone={feedback.tone}
            message={feedback.message}
            onClear={() => setFeedback(null)}
          />
        ) : null}

        <AppCard
          title="证书列表"
          description="展示证书有效期、备注和状态，支持直接查看详情、编辑内容或删除证书。"
        >
          {certificatesQuery.isLoading ? (
            <LoadingState />
          ) : certificatesQuery.isError ? (
            <ErrorState
              title="证书列表加载失败"
              description={getErrorMessage(certificatesQuery.error)}
            />
          ) : certificates.length === 0 ? (
            <EmptyState
              title="暂无证书"
              description="点击右上角“添加证书”开始录入。"
            />
          ) : (
            <div className="space-y-3">
              {certificates.map((certificate) => {
                const status = getCertificateStatus(certificate);

                return (
                  <div
                    key={certificate.id}
                    className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div className="space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                            {certificate.name}
                          </p>
                          <StatusBadge
                            label={status.label}
                            variant={status.variant}
                          />
                        </div>
                        <div className="text-xs leading-5 text-[var(--foreground-secondary)]">
                          <p>生效：{formatDateTime(certificate.not_before)}</p>
                          <p>到期：{formatDateTime(certificate.not_after)}</p>
                          <p>
                            来源：
                            {certificate.provider === 'acme'
                              ? 'ACME 申请'
                              : '手动上传'}
                          </p>
                          {certificate.provider === 'upload' &&
                          certificate.apply_status === 'applying' ? (
                            <p className="text-blue-500">状态：转换申请中...</p>
                          ) : certificate.apply_status === 'applying' ? (
                            <p className="text-blue-500">状态：申请中...</p>
                          ) : null}
                          {certificate.provider === 'upload' &&
                          certificate.apply_status === 'error' ? (
                            <p className="text-red-500">
                              状态：转换失败 ({certificate.apply_message})
                            </p>
                          ) : certificate.apply_status === 'error' ? (
                            <p className="text-red-500">
                              状态：申请失败 ({certificate.apply_message})
                            </p>
                          ) : null}
                          <p>备注：{certificate.remark || '暂无备注'}</p>
                        </div>
                      </div>

                      <div className="flex flex-wrap gap-2">
                        <SecondaryButton
                          type="button"
                          onClick={() =>
                            handleOpenCertificateDetail(certificate)
                          }
                          className="px-3 py-2 text-xs"
                        >
                          查看
                        </SecondaryButton>
                        <SecondaryButton
                          type="button"
                          onClick={() =>
                            handleOpenCertificateEditor(certificate)
                          }
                          className="px-3 py-2 text-xs"
                        >
                          编辑
                        </SecondaryButton>
                        {certificate.provider === 'acme' && (
                          <SecondaryButton
                            type="button"
                            onClick={() => handleRenewCertificate(certificate)}
                            disabled={renewCertificateMutation.isPending}
                            className="px-3 py-2 text-xs"
                          >
                            续期
                          </SecondaryButton>
                        )}
                        <DangerButton
                          type="button"
                          onClick={() => handleDeleteCertificate(certificate)}
                          disabled={deleteCertificateMutation.isPending}
                          className="px-3 py-2 text-xs"
                        >
                          删除
                        </DangerButton>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </AppCard>
      </div>

      {isImportOpen ? (
        <CertificateImportModal
          isOpen={isImportOpen}
          onClose={() => setIsImportOpen(false)}
          onImported={(certificate) => {
            setFeedback({
              tone: 'success',
              message: `证书 ${certificate.name} 已导入。`,
            });
          }}
        />
      ) : null}

      {isApplyOpen ? (
        <CertificateApplyModal
          isOpen={isApplyOpen}
          onClose={() => setIsApplyOpen(false)}
          onApplied={(certificate) => {
            setFeedback({
              tone: 'success',
              message: `证书 ${certificate.name} 申请任务已提交。`,
            });
          }}
        />
      ) : null}

      {applyCertificate ? (
        <CertificateApplyModal
          isOpen={true}
          onClose={() => setApplyCertificate(null)}
          mode={applyMode}
          certificate={applyCertificate}
          onApplied={(certificate) => {
            setFeedback({
              tone: 'success',
              message:
                applyMode === 'convert-upload'
                  ? `证书 ${certificate.name} 转换申请已提交。`
                  : `证书 ${certificate.name} 配置已更新，重新申请中...`,
            });
          }}
        />
      ) : null}

      {isDetailOpen ? (
        <CertificateDetailModal
          certificateId={selectedCertificateId}
          isOpen={isDetailOpen}
          onClose={() => setIsDetailOpen(false)}
          onEdit={() => {
            setIsDetailOpen(false);
            const certificate = certificates.find(
              (item) => item.id === selectedCertificateId,
            );
            if (certificate) {
              handleOpenCertificateEditor(certificate);
            }
          }}
          onDelete={() => {
            const certificate = certificates.find(
              (item) => item.id === selectedCertificateId,
            );
            if (certificate) {
              setIsDetailOpen(false);
              handleDeleteCertificate(certificate);
            }
          }}
          deleting={deleteCertificateMutation.isPending}
        />
      ) : null}

      {isEditorOpen ? (
        <CertificateEditorModal
          certificateId={selectedCertificateId}
          isOpen={isEditorOpen}
          onClose={() => setIsEditorOpen(false)}
          onSaved={(certificate) => {
            setFeedback({
              tone: 'success',
              message: `证书 ${certificate.name} 已更新。`,
            });
          }}
          onConvert={(certificate) => {
            setIsEditorOpen(false);
            setApplyMode('convert-upload');
            setApplyCertificate(certificate);
          }}
        />
      ) : null}
    </>
  );
}
