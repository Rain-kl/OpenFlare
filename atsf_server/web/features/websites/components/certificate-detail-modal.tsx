'use client';

import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { AppModal } from '@/components/ui/app-modal';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  getTlsCertificate,
  getTlsCertificateContent,
} from '@/features/tls-certificates/api/tls-certificates';
import { getCertificateStatus, getErrorMessage } from '@/features/websites/utils';
import {
  CodeBlock,
  DangerButton,
  PrimaryButton,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

interface CertificateDetailModalProps {
  certificateId: number | null;
  isOpen: boolean;
  onClose: () => void;
  onEdit: () => void;
  onDelete: () => void;
  deleting?: boolean;
}

export function CertificateDetailModal({
  certificateId,
  isOpen,
  onClose,
  onEdit,
  onDelete,
  deleting = false,
}: CertificateDetailModalProps) {
  const [copyMessage, setCopyMessage] = useState<string | null>(null);

  const certificateQuery = useQuery({
    queryKey: ['tls-certificates', 'detail', certificateId],
    queryFn: () => getTlsCertificate(certificateId as number),
    enabled: isOpen && certificateId !== null,
  });

  const contentQuery = useQuery({
    queryKey: ['tls-certificates', 'content', certificateId],
    queryFn: () => getTlsCertificateContent(certificateId as number),
    enabled: isOpen && certificateId !== null,
  });

  const certificate = certificateQuery.data;
  const content = contentQuery.data;
  const status = certificate ? getCertificateStatus(certificate) : null;

  const handleCopy = async (value: string, message: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopyMessage(message);
    } catch (error) {
      setCopyMessage(getErrorMessage(error));
    }
  };

  return (
    <AppModal
      isOpen={isOpen}
      onClose={() => {
        setCopyMessage(null);
        onClose();
      }}
      title="证书详情"
      description="查看证书元信息、备注以及当前保存的 PEM 内容。"
      size="xl"
      footer={
        <div className="flex flex-wrap justify-end gap-3">
          <SecondaryButton type="button" onClick={onClose}>
            关闭
          </SecondaryButton>
          <PrimaryButton
            type="button"
            onClick={onEdit}
            disabled={!certificate}
          >
            编辑证书
          </PrimaryButton>
          <DangerButton
            type="button"
            onClick={onDelete}
            disabled={!certificate || deleting}
          >
            {deleting ? '删除中...' : '删除证书'}
          </DangerButton>
        </div>
      }
    >
      {copyMessage ? (
        <InlineMessage tone="success" message={copyMessage} />
      ) : null}

      {certificateQuery.isLoading || contentQuery.isLoading ? (
        <LoadingState />
      ) : certificateQuery.isError || contentQuery.isError ? (
        <ErrorState
          title="证书详情加载失败"
          description={getErrorMessage(
            certificateQuery.error ?? contentQuery.error,
          )}
        />
      ) : !certificate || !content ? (
        <EmptyState
          title="证书不存在"
          description="当前证书可能已被删除。"
        />
      ) : (
        <div className="space-y-6">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs uppercase tracking-[0.18em] text-[var(--foreground-muted)]">
                证书名称
              </p>
              <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                {certificate.name}
              </p>
            </div>
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs uppercase tracking-[0.18em] text-[var(--foreground-muted)]">
                状态
              </p>
              <div className="mt-2">
                {status ? (
                  <StatusBadge label={status.label} variant={status.variant} />
                ) : null}
              </div>
            </div>
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs uppercase tracking-[0.18em] text-[var(--foreground-muted)]">
                生效时间
              </p>
              <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                {formatDateTime(certificate.not_before)}
              </p>
            </div>
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs uppercase tracking-[0.18em] text-[var(--foreground-muted)]">
                到期时间
              </p>
              <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                {formatDateTime(certificate.not_after)}
              </p>
            </div>
          </div>

          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
            <p className="text-xs uppercase tracking-[0.18em] text-[var(--foreground-muted)]">
              备注
            </p>
            <p className="mt-2 text-sm leading-6 text-[var(--foreground-primary)]">
              {certificate.remark || '暂无备注'}
            </p>
          </div>

          <div className="space-y-4">
            <div>
              <div className="mb-2 flex items-center justify-between gap-3">
                <p className="text-sm font-medium text-[var(--foreground-primary)]">
                  证书 PEM
                </p>
                <SecondaryButton
                  type="button"
                  className="px-3 py-2 text-xs"
                  onClick={() =>
                    void handleCopy(content.cert_pem, '证书 PEM 已复制。')
                  }
                >
                  复制
                </SecondaryButton>
              </div>
              <CodeBlock className="max-h-56 overflow-y-auto whitespace-pre-wrap break-all">
                {content.cert_pem}
              </CodeBlock>
            </div>
            <div>
              <div className="mb-2 flex items-center justify-between gap-3">
                <p className="text-sm font-medium text-[var(--foreground-primary)]">
                  私钥 PEM
                </p>
                <SecondaryButton
                  type="button"
                  className="px-3 py-2 text-xs"
                  onClick={() =>
                    void handleCopy(content.key_pem, '私钥 PEM 已复制。')
                  }
                >
                  复制
                </SecondaryButton>
              </div>
              <CodeBlock className="max-h-56 overflow-y-auto whitespace-pre-wrap break-all">
                {content.key_pem}
              </CodeBlock>
            </div>
          </div>
        </div>
      )}
    </AppModal>
  );
}
