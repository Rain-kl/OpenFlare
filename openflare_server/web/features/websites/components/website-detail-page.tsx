'use client';

import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useMemo, useState} from 'react';

import {EmptyState} from '@/components/feedback/empty-state';
import {ErrorState} from '@/components/feedback/error-state';
import {InlineMessage} from '@/components/feedback/inline-message';
import {LoadingState} from '@/components/feedback/loading-state';
import {PageHeader} from '@/components/layout/page-header';
import {AppCard} from '@/components/ui/app-card';
import {StatusBadge} from '@/components/ui/status-badge';
import {deleteManagedDomain, getManagedDomains,} from '@/features/managed-domains/api/managed-domains';
import {getProxyRoutes,} from '@/features/proxy-routes/api/proxy-routes';
import {deleteTlsCertificate, getTlsCertificates,} from '@/features/tls-certificates/api/tls-certificates';
import {CertificateDetailModal} from '@/features/websites/components/certificate-detail-modal';
import {CertificateEditorModal} from '@/features/websites/components/certificate-editor-modal';
import {CertificateImportModal} from '@/features/websites/components/certificate-import-modal';
import {WebsiteEditorModal} from '@/features/websites/components/website-editor-modal';
import {
    buildCertificateLabel,
    getCertificateStatus,
    getErrorMessage,
    getMatchTypeMeta,
    isRouteRelatedToManagedDomain,
} from '@/features/websites/utils';
import {DangerButton, PrimaryButton, SecondaryButton,} from '@/features/shared/components/resource-primitives';
import {formatDateTime} from '@/lib/utils/date';

type FeedbackState = {
    tone: 'success' | 'danger';
    message: string;
};

export function WebsiteDetailPage({websiteId}: { websiteId: string }) {
    const router = useRouter();
    const queryClient = useQueryClient();
    const [feedback, setFeedback] = useState<FeedbackState | null>(null);
    const [isEditorOpen, setIsEditorOpen] = useState(false);
    const [isCertificateImportOpen, setIsCertificateImportOpen] = useState(false);
    const [isCertificateDetailOpen, setIsCertificateDetailOpen] = useState(false);
    const [isCertificateEditorOpen, setIsCertificateEditorOpen] = useState(false);
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
    const proxyRoutesQuery = useQuery({
        queryKey: ['proxy-routes'],
        queryFn: getProxyRoutes,
    });

    const deleteDomainMutation = useMutation({
        mutationFn: deleteManagedDomain,
        onSuccess: async () => {
            await queryClient.invalidateQueries({queryKey: ['managed-domains']});
            router.push('/website');
        },
        onError: (error) => {
            setFeedback({tone: 'danger', message: getErrorMessage(error)});
        },
    });

    const deleteCertificateMutation = useMutation({
        mutationFn: deleteTlsCertificate,
        onSuccess: async () => {
            setFeedback({tone: 'success', message: '证书已删除。'});
            await Promise.all([
                queryClient.invalidateQueries({queryKey: ['tls-certificates']}),
                queryClient.invalidateQueries({queryKey: ['managed-domains']}),
            ]);
        },
        onError: (error) => {
            setFeedback({tone: 'danger', message: getErrorMessage(error)});
        },
    });

    const website = useMemo(() => {
        return (
            (managedDomainsQuery.data ?? []).find(
                (item) => String(item.id) === websiteId,
            ) ?? null
        );
    }, [managedDomainsQuery.data, websiteId]);

    const certificates = useMemo(
        () => certificatesQuery.data ?? [],
        [certificatesQuery.data],
    );
    const certificateMap = useMemo(
        () => new Map(certificates.map((item) => [item.id, item])),
        [certificates],
    );
    const relatedRoutes = useMemo(() => {
        if (!website) {
            return [];
        }

        return (proxyRoutesQuery.data ?? []).filter((route) =>
            isRouteRelatedToManagedDomain(website.domain, route.domain),
        );
    }, [proxyRoutesQuery.data, website]);

    const certificate = website?.cert_id
        ? certificateMap.get(website.cert_id) ?? null
        : null;
    const enabledRoutesCount = relatedRoutes.filter((route) => route.enabled).length;

    const handleDeleteWebsite = () => {
        if (!website) {
            return;
        }

        if (!window.confirm(`确认删除网站 ${website.domain} 吗？`)) {
            return;
        }

        setFeedback(null);
        deleteDomainMutation.mutate(website.id);
    };

    const handleDeleteCertificate = () => {
        if (!certificate) {
            return;
        }

        if (!window.confirm(`确认删除证书 ${certificate.name} 吗？`)) {
            return;
        }

        setFeedback(null);
        deleteCertificateMutation.mutate(certificate.id);
    };

    if (
        managedDomainsQuery.isLoading ||
        certificatesQuery.isLoading ||
        proxyRoutesQuery.isLoading
    ) {
        return <LoadingState/>;
    }

    if (managedDomainsQuery.isError) {
        return (
            <ErrorState
                title="网站详情加载失败"
                description={getErrorMessage(managedDomainsQuery.error)}
            />
        );
    }

    if (certificatesQuery.isError) {
        return (
            <ErrorState
                title="证书信息加载失败"
                description={getErrorMessage(certificatesQuery.error)}
            />
        );
    }

    if (proxyRoutesQuery.isError) {
        return (
            <ErrorState
                title="关联规则加载失败"
                description={getErrorMessage(proxyRoutesQuery.error)}
            />
        );
    }

    if (!website) {
        return (
            <EmptyState
                title="网站不存在"
                description="该网站可能已被删除，或当前 ID 无法匹配到网站记录。"
            />
        );
    }

    const matchType = getMatchTypeMeta(website.domain);
    const certificateStatus = certificate ? getCertificateStatus(certificate) : null;

    return (
        <>
            <div className="space-y-6">
                <PageHeader
                    title={website.domain}
                    description="网站详情"
                    action={
                        <>
                            <Link
                                href="/website"
                                className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
                            >
                                返回
                            </Link>
                            <SecondaryButton
                                type="button"
                                onClick={() => setIsEditorOpen(true)}
                            >
                                编辑网站
                            </SecondaryButton>
                            <PrimaryButton
                                type="button"
                                onClick={() => setIsCertificateImportOpen(true)}
                            >
                                添加证书
                            </PrimaryButton>
                            <DangerButton
                                type="button"
                                onClick={handleDeleteWebsite}
                                disabled={deleteDomainMutation.isPending}
                            >
                                删除网站
                            </DangerButton>
                        </>
                    }
                />

                {feedback ? (
                    <InlineMessage tone={feedback.tone} message={feedback.message}/>
                ) : null}

                <div className="grid gap-4 xl:grid-cols-4">
                    <AppCard title="匹配类型">
                        <div className="space-y-3">
                            <StatusBadge label={matchType.label} variant={matchType.variant}/>
                            <p className="text-sm text-[var(--foreground-secondary)]">
                                {website.domain.startsWith('*.')
                                    ? '当前网站会覆盖该后缀下的子域名规则。'
                                    : '当前网站只匹配同名精确域名。'}
                            </p>
                        </div>
                    </AppCard>

                    <AppCard title="网站状态">
                        <div className="space-y-3">
                            <StatusBadge
                                label={website.enabled ? '启用' : '停用'}
                                variant={website.enabled ? 'success' : 'warning'}
                            />
                            <p className="text-sm text-[var(--foreground-secondary)]">
                                更新时间：{formatDateTime(website.updated_at)}
                            </p>
                        </div>
                    </AppCard>

                    <AppCard title="关联规则">
                        <div className="space-y-2 text-sm text-[var(--foreground-secondary)]">
                            <p>规则总数：{relatedRoutes.length}</p>
                            <p>已启用规则：{enabledRoutesCount}</p>
                        </div>
                    </AppCard>

                    <AppCard title="绑定证书">
                        <div className="space-y-3">
                            <StatusBadge
                                label={certificate ? certificate.name : '未绑定证书'}
                                variant={certificate ? 'success' : 'warning'}
                            />
                            <p className="text-sm text-[var(--foreground-secondary)]">
                                {certificateStatus
                                    ? `证书状态：${certificateStatus.label}`
                                    : '当前网站未设置默认证书。'}
                            </p>
                        </div>
                    </AppCard>
                </div>

                <div className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
                    <AppCard title="网站信息" description="当前网站的基础配置与托管信息。">
                        <div className="grid gap-4 md:grid-cols-2">
                            <div
                                className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                                <p className="text-xs uppercase tracking-[0.18em] text-[var(--foreground-muted)]">
                                    域名
                                </p>
                                <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                                    {website.domain}
                                </p>
                            </div>
                            <div
                                className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                                <p className="text-xs uppercase tracking-[0.18em] text-[var(--foreground-muted)]">
                                    创建时间
                                </p>
                                <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                                    {formatDateTime(website.created_at)}
                                </p>
                            </div>
                            <div
                                className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4 md:col-span-2">
                                <p className="text-xs uppercase tracking-[0.18em] text-[var(--foreground-muted)]">
                                    备注
                                </p>
                                <p className="mt-2 text-sm leading-6 text-[var(--foreground-primary)]">
                                    {website.remark || '暂无备注'}
                                </p>
                            </div>
                        </div>
                    </AppCard>
                    <AppCard
                        title="证书信息"
                        description="当前网站绑定的默认证书信息。若还没有证书，可直接添加后回填。"
                    >
                        {certificate ? (
                            <div className="space-y-4">
                                <div className="flex flex-wrap items-center gap-2">
                                    <StatusBadge
                                        label={certificateStatus?.label ?? '有效'}
                                        variant={certificateStatus?.variant ?? 'success'}
                                    />
                                </div>
                                <div className="space-y-2 text-sm text-[var(--foreground-secondary)]">
                                    <p>证书名称：{buildCertificateLabel(certificate)}</p>
                                    <p>生效时间：{formatDateTime(certificate.not_before)}</p>
                                    <p>到期时间：{formatDateTime(certificate.not_after)}</p>
                                    <p>备注：{certificate.remark || '暂无备注'}</p>
                                </div>
                                <div className="flex flex-wrap gap-3">
                                    <SecondaryButton
                                        type="button"
                                        onClick={() => setIsCertificateDetailOpen(true)}
                                    >
                                        查看
                                    </SecondaryButton>
                                    <DangerButton
                                        type="button"
                                        onClick={handleDeleteCertificate}
                                        disabled={deleteCertificateMutation.isPending}
                                    >
                                        删除
                                    </DangerButton>
                                </div>
                            </div>
                        ) : (
                            <EmptyState
                                title="未绑定证书"
                                description="点击右上角“添加证书”后，可以在编辑网站时直接选择并应用。"
                            />
                        )}
                    </AppCard>
                </div>

                <AppCard
                    title="关联规则"
                >
                    {relatedRoutes.length === 0 ? (
                        <EmptyState
                            title="暂无关联规则"
                            description="当前网站还没有命中任何代理规则。创建或调整规则后，这里会自动展示。"
                        />
                    ) : (
                        <div className="overflow-x-auto">
                            <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                                <thead>
                                <tr className="text-[var(--foreground-secondary)]">
                                    <th className="px-3 py-3 font-medium">规则域名</th>
                                    <th className="px-3 py-3 font-medium">源站</th>
                                    <th className="px-3 py-3 font-medium">HTTPS</th>
                                    <th className="px-3 py-3 font-medium">证书</th>
                                    <th className="px-3 py-3 font-medium">状态</th>
                                    <th className="px-3 py-3 font-medium">备注</th>
                                </tr>
                                </thead>
                                <tbody className="divide-y divide-[var(--border-default)]">
                                {relatedRoutes.map((route) => {
                                    const routeCertificate = route.cert_id
                                        ? certificateMap.get(route.cert_id) ?? null
                                        : null;

                                    return (
                                        <tr key={route.id} className="align-top">
                                            <td className="px-3 py-4 text-[var(--foreground-primary)]">
                                                <div className="space-y-2">
                                                    <p>{route.domain}</p>
                                                    <StatusBadge
                                                        label={
                                                            route.domain === website.domain
                                                                ? '直接关联'
                                                                : '被网站覆盖'
                                                        }
                                                        variant={
                                                            route.domain === website.domain
                                                                ? 'info'
                                                                : 'warning'
                                                        }
                                                    />
                                                </div>
                                            </td>
                                            <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                                                <div className="max-w-72 space-y-1 break-all">
                                                    <p>{route.origin_url}</p>
                                                    <p className="text-xs text-[var(--foreground-muted)]">
                                                        回源主机名: {route.origin_host || '$host'}
                                                    </p>
                                                </div>
                                            </td>
                                            <td className="px-3 py-4">
                                                <div className="flex flex-wrap gap-2">
                                                    <StatusBadge
                                                        label={route.enable_https ? '启用 HTTPS' : 'HTTP'}
                                                        variant={route.enable_https ? 'success' : 'warning'}
                                                    />
                                                    {route.redirect_http ? (
                                                        <StatusBadge label="HTTP 跳转" variant="info"/>
                                                    ) : null}
                                                </div>
                                            </td>
                                            <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                                                {routeCertificate
                                                    ? routeCertificate.name
                                                    : '未绑定证书'}
                                            </td>
                                            <td className="px-3 py-4">
                                                <StatusBadge
                                                    label={route.enabled ? '启用' : '停用'}
                                                    variant={route.enabled ? 'success' : 'warning'}
                                                />
                                            </td>
                                            <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                                                {route.remark || '暂无备注'}
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

            {isEditorOpen ? (
                <WebsiteEditorModal
                    isOpen={isEditorOpen}
                    onClose={() => {
                        setIsEditorOpen(false);
                        setPreferredCertificateId(null);
                    }}
                    initialDomain={website}
                    certificates={certificates}
                    certificatesLoading={certificatesQuery.isLoading}
                    preferredCertificateId={preferredCertificateId}
                    onRequestImportCertificate={() => setIsCertificateImportOpen(true)}
                    onSaved={() => {
                        setPreferredCertificateId(null);
                        setFeedback({tone: 'success', message: '网站已更新。'});
                    }}
                />
            ) : null}

            {isCertificateImportOpen ? (
                <CertificateImportModal
                    isOpen={isCertificateImportOpen}
                    onClose={() => setIsCertificateImportOpen(false)}
                    onImported={(importedCertificate) => {
                        setPreferredCertificateId(importedCertificate.id);
                        setFeedback({
                            tone: 'success',
                            message: `证书 ${importedCertificate.name} 已导入，可在编辑网站时直接应用。`,
                        });
                        setIsEditorOpen(true);
                    }}
                />
            ) : null}

            {isCertificateDetailOpen ? (
                <CertificateDetailModal
                    certificateId={certificate?.id ?? null}
                    isOpen={isCertificateDetailOpen}
                    onClose={() => setIsCertificateDetailOpen(false)}
                    onEdit={() => {
                        setIsCertificateDetailOpen(false);
                        setIsCertificateEditorOpen(true);
                    }}
                    onDelete={() => {
                        setIsCertificateDetailOpen(false);
                        handleDeleteCertificate();
                    }}
                    deleting={deleteCertificateMutation.isPending}
                />
            ) : null}

            {isCertificateEditorOpen ? (
                <CertificateEditorModal
                    certificateId={certificate?.id ?? null}
                    isOpen={isCertificateEditorOpen}
                    onClose={() => setIsCertificateEditorOpen(false)}
                    onSaved={(updatedCertificate) => {
                        setFeedback({
                            tone: 'success',
                            message: `证书 ${updatedCertificate.name} 已更新。`,
                        });
                    }}
                />
            ) : null}
        </>
    );
}
