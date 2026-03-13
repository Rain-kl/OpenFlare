'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState, type FormEvent } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { AppModal } from '@/components/ui/app-modal';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  createManagedDomain,
  deleteManagedDomain,
  getManagedDomains,
  updateManagedDomain,
} from '@/features/managed-domains/api/managed-domains';
import type {
  ManagedDomainItem,
  ManagedDomainMutationPayload,
} from '@/features/managed-domains/types';
import {
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  ResourceTextarea,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
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
import { formatDateTime } from '@/lib/utils/date';

const managedDomainsQueryKey = ['managed-domains'] as const;
const managedDomainCertificatesQueryKey = [
  'managed-domains',
  'certificates',
] as const;
const tlsCertificatesQueryKey = ['tls-certificates', 'list'] as const;

const managedDomainSchema = z.object({
  domain: z
    .string()
    .trim()
    .min(1, '请输入域名')
    .max(255, '域名不能超过 255 个字符')
    .refine(
      (value) => !value.includes('://') && !value.includes('/'),
      '域名格式不合法',
    )
    .refine(
      (value) =>
        /^(?:\*\.)?(?=.{1,253}$)(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$/.test(
          value,
        ),
      '域名格式不合法',
    )
    .refine(
      (value) =>
        !value.includes('*') ||
        (value.startsWith('*.') && value.indexOf('*', 1) === -1),
      '通配符域名仅支持 *.example.com 格式',
    ),
  cert_id: z.string(),
  enabled: z.boolean(),
  remark: z.string().max(255, '备注不能超过 255 个字符'),
});

const manualImportSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, '请输入证书名称')
    .max(255, '证书名称不能超过 255 个字符'),
  cert_pem: z.string().trim().min(1, '请输入证书 PEM 内容'),
  key_pem: z.string().trim().min(1, '请输入私钥 PEM 内容'),
  remark: z.string().max(255, '备注不能超过 255 个字符'),
});

type ManagedDomainFormValues = z.infer<typeof managedDomainSchema>;
type ManualImportFormValues = z.infer<typeof manualImportSchema>;

type FileImportFormValues = {
  name: string;
  remark: string;
};

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

const defaultDomainValues: ManagedDomainFormValues = {
  domain: '',
  cert_id: '',
  enabled: true,
  remark: '',
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

function getMatchTypeMeta(domain: string) {
  return domain.startsWith('*.')
    ? { label: '通配符', variant: 'warning' as const }
    : { label: '精确匹配', variant: 'info' as const };
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

function buildCertificateLabel(certificate: TlsCertificateItem) {
  return certificate.not_after
    ? `${certificate.name}（到期：${formatDateTime(certificate.not_after)}）`
    : certificate.name;
}

function toManagedDomainPayload(
  values: ManagedDomainFormValues,
): ManagedDomainMutationPayload {
  return {
    domain: values.domain.trim().toLowerCase(),
    cert_id: values.cert_id ? Number(values.cert_id) : null,
    enabled: values.enabled,
    remark: values.remark.trim(),
  };
}

function toManagedDomainFormValues(
  domain: ManagedDomainItem,
): ManagedDomainFormValues {
  return {
    domain: domain.domain,
    cert_id: domain.cert_id ? String(domain.cert_id) : '',
    enabled: domain.enabled,
    remark: domain.remark || '',
  };
}

function toManualPayload(
  values: ManualImportFormValues,
): TlsCertificateMutationPayload {
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

export function WebsitesPage() {
  const queryClient = useQueryClient();
  const [editingDomainId, setEditingDomainId] = useState<number | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [isImportModalOpen, setIsImportModalOpen] = useState(false);
  const [importMode, setImportMode] = useState<'manual' | 'file'>('manual');
  const [fileForm, setFileForm] =
    useState<FileImportFormValues>(defaultFileValues);
  const [certFile, setCertFile] = useState<File | null>(null);
  const [keyFile, setKeyFile] = useState<File | null>(null);
  const [fileInputNonce, setFileInputNonce] = useState(0);

  const domainForm = useForm<ManagedDomainFormValues>({
    resolver: zodResolver(managedDomainSchema),
    defaultValues: defaultDomainValues,
  });
  const manualForm = useForm<ManualImportFormValues>({
    resolver: zodResolver(manualImportSchema),
    defaultValues: defaultManualValues,
  });

  const watchedDomain = useWatch({
    control: domainForm.control,
    name: 'domain',
  });
  const watchedCertId = useWatch({
    control: domainForm.control,
    name: 'cert_id',
  });
  const watchedEnabled = useWatch({
    control: domainForm.control,
    name: 'enabled',
  });

  const managedDomainsQuery = useQuery({
    queryKey: managedDomainsQueryKey,
    queryFn: getManagedDomains,
  });
  const certificatesQuery = useQuery({
    queryKey: managedDomainCertificatesQueryKey,
    queryFn: getTlsCertificates,
  });

  const saveDomainMutation = useMutation({
    mutationFn: async (values: ManagedDomainFormValues) => {
      const payload = toManagedDomainPayload(values);
      return editingDomainId
        ? updateManagedDomain(editingDomainId, payload)
        : createManagedDomain(payload);
    },
    onSuccess: async () => {
      setFeedback({
        tone: 'success',
        message: editingDomainId ? '网站已更新。' : '网站已创建。',
      });
      setEditingDomainId(null);
      domainForm.reset(defaultDomainValues);
      await queryClient.invalidateQueries({ queryKey: managedDomainsQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteDomainMutation = useMutation({
    mutationFn: deleteManagedDomain,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '网站已删除。' });
      await queryClient.invalidateQueries({ queryKey: managedDomainsQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const manualImportMutation = useMutation({
    mutationFn: async (values: ManualImportFormValues) =>
      createTlsCertificate(toManualPayload(values)),
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '证书已导入。' });
      setImportMode('manual');
      setIsImportModalOpen(false);
      manualForm.reset(defaultManualValues);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: managedDomainCertificatesQueryKey,
        }),
        queryClient.invalidateQueries({
          queryKey: tlsCertificatesQueryKey,
        }),
      ]);
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
      setImportMode('manual');
      setIsImportModalOpen(false);
      setFileForm(defaultFileValues);
      setCertFile(null);
      setKeyFile(null);
      setFileInputNonce((value) => value + 1);
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: managedDomainCertificatesQueryKey,
        }),
        queryClient.invalidateQueries({
          queryKey: tlsCertificatesQueryKey,
        }),
      ]);
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteCertificateMutation = useMutation({
    mutationFn: deleteTlsCertificate,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '证书已删除。' });
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: managedDomainCertificatesQueryKey,
        }),
        queryClient.invalidateQueries({
          queryKey: tlsCertificatesQueryKey,
        }),
        queryClient.invalidateQueries({ queryKey: managedDomainsQueryKey }),
      ]);
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
  const currentCertificate = watchedCertId
    ? certificateMap.get(Number(watchedCertId))
    : null;

  const handleDomainSubmit = domainForm.handleSubmit((values) => {
    setFeedback(null);
    saveDomainMutation.mutate(values);
  });

  const handleDomainReset = () => {
    setEditingDomainId(null);
    domainForm.reset(defaultDomainValues);
  };

  const handleEditDomain = (domain: ManagedDomainItem) => {
    setEditingDomainId(domain.id);
    setFeedback(null);
    domainForm.reset(toManagedDomainFormValues(domain));
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const handleDeleteDomain = (domain: ManagedDomainItem) => {
    if (!window.confirm(`确认删除网站 ${domain.domain} 吗？`)) {
      return;
    }
    setFeedback(null);
    deleteDomainMutation.mutate(domain.id);
  };

  const handleManualSubmit = manualForm.handleSubmit((values) => {
    setFeedback(null);
    manualImportMutation.mutate(values);
  });

  const handleFileSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFeedback(null);
    fileImportMutation.mutate(fileForm);
  };

  const handleDeleteCertificate = (certificate: TlsCertificateItem) => {
    if (!window.confirm(`确认删除证书 ${certificate.name} 吗？`)) {
      return;
    }
    setFeedback(null);
    deleteCertificateMutation.mutate(certificate.id);
  };

  const handleCloseImportModal = () => {
    setImportMode('manual');
    setIsImportModalOpen(false);
  };

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="网站"
          description="管理域名和TLS证书。"
        />

        {feedback ? (
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        <div className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
          <AppCard
            title={editingDomainId ? '编辑网站' : '快速添加网站'}
            action={
              <div className="flex flex-wrap gap-2">
                {editingDomainId ? (
                  <SecondaryButton type="button" onClick={handleDomainReset}>
                    取消编辑
                  </SecondaryButton>
                ) : null}
                <PrimaryButton
                  type="button"
                  onClick={() => void handleDomainSubmit()}
                  disabled={saveDomainMutation.isPending}
                >
                  {saveDomainMutation.isPending
                    ? '保存中...'
                    : editingDomainId
                      ? '保存网站'
                      : '创建网站'}
                </PrimaryButton>
              </div>
            }
          >
            <form className="space-y-5" onSubmit={handleDomainSubmit}>
              <div className="grid gap-4 md:grid-cols-2">
                <ResourceField
                  label="域名"
                  hint="示例：example.com 或 *.example.com"
                  error={domainForm.formState.errors.domain?.message}
                >
                  <ResourceInput
                    placeholder="example.com 或 *.example.com"
                    {...domainForm.register('domain')}
                  />
                </ResourceField>
                <ResourceField
                  label="默认证书"
                  hint="证书用于域名自动匹配与规则推荐，可不选择。"
                  error={domainForm.formState.errors.cert_id?.message}
                >
                  <ResourceSelect
                    value={watchedCertId}
                    disabled={certificatesQuery.isLoading}
                    onChange={(event) =>
                      domainForm.setValue('cert_id', event.target.value, {
                        shouldDirty: true,
                        shouldValidate: true,
                      })
                    }
                  >
                    <option value="">不绑定证书</option>
                    {certificates.map((certificate) => (
                      <option key={certificate.id} value={certificate.id}>
                        {buildCertificateLabel(certificate)}
                      </option>
                    ))}
                  </ResourceSelect>
                </ResourceField>
              </div>

              <ToggleField
                label="启用网站"
                description="停用后该域名不会参与自动匹配，但记录会保留。"
                checked={watchedEnabled}
                onChange={(checked) =>
                  domainForm.setValue('enabled', checked, {
                    shouldDirty: true,
                    shouldValidate: true,
                  })
                }
              />

              <ResourceField
                label="备注"
                hint="可选，用于记录归属、用途或生效说明。"
                error={domainForm.formState.errors.remark?.message}
              >
                <ResourceInput
                  placeholder="例如：主站 / 泛域名 / 生产"
                  {...domainForm.register('remark')}
                />
              </ResourceField>
            </form>
          </AppCard>

          <AppCard
            title="证书"
            action={
              <div className="flex flex-wrap gap-2">
                <SecondaryButton
                  type="button"
                  onClick={() =>
                    void queryClient.invalidateQueries({
                      queryKey: managedDomainCertificatesQueryKey,
                    })
                  }
                >
                  刷新证书
                </SecondaryButton>
                <PrimaryButton
                  type="button"
                  onClick={() => {
                    setImportMode('manual');
                    setIsImportModalOpen(true);
                  }}
                >
                  导入证书
                </PrimaryButton>
              </div>
            }
          >
            <div className="space-y-4">
              <div className="grid gap-4 md:grid-cols-3 xl:grid-cols-1">
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    当前网站输入
                  </p>
                  <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                    {watchedDomain.trim() || '未填写域名'}
                  </p>
                </div>
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    匹配类型
                  </p>
                  <div className="mt-2">
                    {watchedDomain.trim() ? (
                      <StatusBadge
                        {...getMatchTypeMeta(watchedDomain.trim())}
                      />
                    ) : (
                      <StatusBadge label="等待输入" variant="warning" />
                    )}
                  </div>
                </div>
                <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
                  <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                    当前证书
                  </p>
                  <div className="mt-2">
                    <StatusBadge
                      label={
                        currentCertificate
                          ? currentCertificate.name
                          : '未绑定证书'
                      }
                      variant={currentCertificate ? 'success' : 'warning'}
                    />
                  </div>
                </div>
              </div>

              <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4 text-sm leading-6 text-[var(--foreground-secondary)]">
                <p>
                  建议流程：先导入证书，再在左侧创建网站并直接绑定。若同一
                  hostname 同时命中精确和通配符， 系统会优先使用精确匹配。
                </p>
              </div>
            </div>
          </AppCard>
        </div>

        <div className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
          <AppCard
            title="网站列表"
            description="维护精确域名与通配符域名，并查看当前绑定的默认证书。"
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
                description="先在上方快速添加一个网站，或导入证书后再绑定。"
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                  <thead>
                    <tr className="text-[var(--foreground-secondary)]">
                      <th className="px-3 py-3 font-medium">域名</th>
                      <th className="px-3 py-3 font-medium">类型</th>
                      <th className="px-3 py-3 font-medium">绑定证书</th>
                      <th className="px-3 py-3 font-medium">状态</th>
                      <th className="px-3 py-3 font-medium">更新时间</th>
                      <th className="px-3 py-3 font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--border-default)]">
                    {domains.map((domain) => {
                      const certificate = domain.cert_id
                        ? certificateMap.get(domain.cert_id)
                        : null;
                      const matchType = getMatchTypeMeta(domain.domain);

                      return (
                        <tr key={domain.id} className="align-top">
                          <td className="px-3 py-4 font-medium text-[var(--foreground-primary)]">
                            <div className="space-y-1">
                              <p>{domain.domain}</p>
                              <p className="text-xs text-[var(--foreground-secondary)]">
                                {domain.remark || '—'}
                              </p>
                            </div>
                          </td>
                          <td className="px-3 py-4">
                            <StatusBadge
                              label={matchType.label}
                              variant={matchType.variant}
                            />
                          </td>
                          <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                            {certificate ? certificate.name : '未绑定'}
                          </td>
                          <td className="px-3 py-4">
                            <StatusBadge
                              label={domain.enabled ? '启用' : '停用'}
                              variant={domain.enabled ? 'success' : 'warning'}
                            />
                          </td>
                          <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                            {formatDateTime(domain.updated_at)}
                          </td>
                          <td className="px-3 py-4">
                            <div className="flex flex-wrap gap-2">
                              <SecondaryButton
                                type="button"
                                onClick={() => handleEditDomain(domain)}
                                className="px-3 py-2 text-xs"
                              >
                                编辑
                              </SecondaryButton>
                              <DangerButton
                                type="button"
                                onClick={() => handleDeleteDomain(domain)}
                                disabled={deleteDomainMutation.isPending}
                                className="px-3 py-2 text-xs"
                              >
                                删除
                              </DangerButton>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </AppCard>

          <AppCard
            title="证书列表"
            description="查看有效期和到期风险，并直接删除不再使用的证书。"
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
                description="点击上方“导入证书”即可开始录入。"
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
                      <div className="flex items-start justify-between gap-3">
                        <div className="space-y-2">
                          <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                            {certificate.name}
                          </p>
                          <div className="flex flex-wrap gap-2">
                            <StatusBadge
                              label={status.label}
                              variant={status.variant}
                            />
                          </div>
                          <div className="text-xs leading-5 text-[var(--foreground-secondary)]">
                            <p>
                              生效：{formatDateTime(certificate.not_before)}
                            </p>
                            <p>到期：{formatDateTime(certificate.not_after)}</p>
                            <p>备注：{certificate.remark || '—'}</p>
                          </div>
                        </div>
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
                  );
                })}
              </div>
            )}
          </AppCard>
        </div>
      </div>

      <AppModal
        isOpen={isImportModalOpen}
        onClose={handleCloseImportModal}
        title="导入证书"
        description="手动导入和文件导入都在这里完成，导入成功后左侧网站表单会立刻可选。"
        size="xl"
      >
        <div className="space-y-6">
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
                      keyFile
                        ? `已选择：${keyFile.name}`
                        : '请选择 KEY/PEM 文件'
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
          )}
        </div>
      </AppModal>
    </>
  );
}
