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
  deleteManagedDomain,
  getManagedDomains,
} from '@/features/managed-domains/api/managed-domains';
import type { ManagedDomainItem } from '@/features/managed-domains/types';
import { getTlsCertificates } from '@/features/tls-certificates/api/tls-certificates';
import { CertificateImportModal } from '@/features/websites/components/certificate-import-modal';
import { WebsiteEditorModal } from '@/features/websites/components/website-editor-modal';
import {
  buildCertificateLabel,
  getErrorMessage,
  getMatchTypeMeta,
} from '@/features/websites/utils';
import {
  DangerButton,
  PrimaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

export function WebsitesPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [isWebsiteModalOpen, setIsWebsiteModalOpen] = useState(false);
  const [isCertificateImportOpen, setIsCertificateImportOpen] = useState(false);
  const [preferredCertificateId, setPreferredCertificateId] = useState<
    number | null
  >(null);

  const managedDomainsQuery = useQuery({
    queryKey: ['managed-domains'],
    queryFn: getManagedDomains,
  });
  const certificatesQuery = useQuery({
    queryKey: ['tls-certificates', 'list'],
    queryFn: getTlsCertificates,
  });

  const deleteDomainMutation = useMutation({
    mutationFn: deleteManagedDomain,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '网站已删除。' });
      await queryClient.invalidateQueries({ queryKey: ['managed-domains'] });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const domains = useMemo(
    () => managedDomainsQuery.data ?? [],
    [managedDomainsQuery.data],
  );
  const certificates = useMemo(
    () => certificatesQuery.data ?? [],
    [certificatesQuery.data],
  );
  const certificateMap = useMemo(
    () => new Map(certificates.map((item) => [item.id, item])),
    [certificates],
  );

  const handleOpenWebsiteModal = () => {
    setPreferredCertificateId(null);
    setFeedback(null);
    setIsWebsiteModalOpen(true);
  };

  const handleDeleteDomain = (domain: ManagedDomainItem) => {
    if (!window.confirm(`确认删除网站 ${domain.domain} 吗？`)) {
      return;
    }

    setFeedback(null);
    deleteDomainMutation.mutate(domain.id);
  };

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="网站"
          description="主界面只保留网站列表卡片。新增网站时可直接绑定证书，也可以在弹窗内先添加证书后再应用。"
          action={
            <div className="flex flex-wrap gap-3">
              <Link
                href="/website/certificate"
                className="inline-flex min-h-[46px] items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
              >
                证书列表
              </Link>
              <PrimaryButton type="button" onClick={handleOpenWebsiteModal}>
                新增网站
              </PrimaryButton>
            </div>
          }
        />

        {feedback ? (
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        <AppCard
          title="网站列表"
          description="查看网站绑定的证书、启用状态和更新时间。进入详情页可继续查看关联规则并编辑网站。"
        >
          {managedDomainsQuery.isLoading ? (
            <LoadingState />
          ) : managedDomainsQuery.isError ? (
            <ErrorState
              title="网站列表加载失败"
              description={getErrorMessage(managedDomainsQuery.error)}
            />
          ) : domains.length === 0 ? (
            <EmptyState
              title="暂无网站"
              description="点击右上角“新增网站”开始录入。录入时如还没有证书，也可以直接在弹窗里添加。"
            />
          ) : (
            <div className="grid gap-4 lg:grid-cols-2">
              {domains.map((domain) => {
                const certificate = domain.cert_id
                  ? certificateMap.get(domain.cert_id) ?? null
                  : null;
                const matchType = getMatchTypeMeta(domain.domain);

                return (
                  <article
                    key={domain.id}
                    className="rounded-[28px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div className="space-y-3">
                        <div className="space-y-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <h2 className="text-lg font-semibold text-[var(--foreground-primary)]">
                              {domain.domain}
                            </h2>
                            <StatusBadge
                              label={matchType.label}
                              variant={matchType.variant}
                            />
                            <StatusBadge
                              label={domain.enabled ? '启用' : '停用'}
                              variant={domain.enabled ? 'success' : 'warning'}
                            />
                          </div>
                          <p className="text-sm text-[var(--foreground-secondary)]">
                            {domain.remark || '暂无备注'}
                          </p>
                        </div>

                        <div className="grid gap-3 md:grid-cols-1">
                          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] px-4 py-3">
                            <p className="text-xs uppercase tracking-[0.18em] text-[var(--foreground-muted)]">
                              绑定证书
                            </p>
                            <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                              {certificate
                                ? buildCertificateLabel(certificate)
                                : '未绑定证书'}
                            </p>
                          </div>
                        </div>
                      </div>

                      <div className="flex flex-col gap-2">
                        <Link
                          href={`/website/detail?id=${domain.id}`}
                          className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
                        >
                          详情
                        </Link>
                        <DangerButton
                          type="button"
                          onClick={() => handleDeleteDomain(domain)}
                          disabled={deleteDomainMutation.isPending}
                        >
                          删除
                        </DangerButton>
                      </div>
                    </div>
                  </article>
                );
              })}
            </div>
          )}
        </AppCard>
      </div>

      <WebsiteEditorModal
        isOpen={isWebsiteModalOpen}
        onClose={() => {
          setIsWebsiteModalOpen(false);
          setPreferredCertificateId(null);
        }}
        certificates={certificates}
        certificatesLoading={certificatesQuery.isLoading}
        preferredCertificateId={preferredCertificateId}
        onRequestImportCertificate={() => setIsCertificateImportOpen(true)}
        onSaved={(_, mode) => {
          setPreferredCertificateId(null);
          setFeedback({
            tone: 'success',
            message: mode === 'create' ? '网站已创建。' : '网站已更新。',
          });
        }}
      />

      <CertificateImportModal
        isOpen={isCertificateImportOpen}
        onClose={() => setIsCertificateImportOpen(false)}
        onImported={(certificate) => {
          setPreferredCertificateId(certificate.id);
          setFeedback({
            tone: 'success',
            message: `证书 ${certificate.name} 已导入，可直接用于当前网站。`,
          });
        }}
      />
    </>
  );
}
