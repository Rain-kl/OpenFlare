'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
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
  createManagedDomain,
  deleteManagedDomain,
  getManagedDomainCertificates,
  getManagedDomains,
  updateManagedDomain,
} from '@/features/managed-domains/api/managed-domains';
import type {
  ManagedDomainCertificateOption,
  ManagedDomainItem,
  ManagedDomainMutationPayload,
} from '@/features/managed-domains/types';
import {
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime } from '@/lib/utils/date';

const managedDomainsQueryKey = ['managed-domains'] as const;
const managedDomainCertificatesQueryKey = ['managed-domains', 'certificates'] as const;

const managedDomainSchema = z.object({
  domain: z
    .string()
    .trim()
    .min(1, '请输入域名')
    .max(255, '域名不能超过 255 个字符')
    .refine((value) => !value.includes('://') && !value.includes('/'), '域名格式不合法')
    .refine(
      (value) => /^(?:\*\.)?(?=.{1,253}$)(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$/.test(value),
      '域名格式不合法',
    )
    .refine(
      (value) => !value.includes('*') || (value.startsWith('*.') && value.indexOf('*', 1) === -1),
      '通配符域名仅支持 *.example.com 格式',
    ),
  cert_id: z.string(),
  enabled: z.boolean(),
  remark: z.string().max(255, '备注不能超过 255 个字符'),
});

type ManagedDomainFormValues = z.infer<typeof managedDomainSchema>;

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

const defaultValues: ManagedDomainFormValues = {
  domain: '',
  cert_id: '',
  enabled: true,
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

function buildCertificateLabel(certificate: ManagedDomainCertificateOption) {
  return certificate.not_after
    ? `${certificate.name}（到期：${formatDateTime(certificate.not_after)}）`
    : certificate.name;
}

function toPayload(values: ManagedDomainFormValues): ManagedDomainMutationPayload {
  return {
    domain: values.domain.trim().toLowerCase(),
    cert_id: values.cert_id ? Number(values.cert_id) : null,
    enabled: values.enabled,
    remark: values.remark.trim(),
  };
}

function toFormValues(domain: ManagedDomainItem): ManagedDomainFormValues {
  return {
    domain: domain.domain,
    cert_id: domain.cert_id ? String(domain.cert_id) : '',
    enabled: domain.enabled,
    remark: domain.remark || '',
  };
}

export function ManagedDomainsPage() {
  const queryClient = useQueryClient();
  const [editingDomainId, setEditingDomainId] = useState<number | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);

  const form = useForm<ManagedDomainFormValues>({
    resolver: zodResolver(managedDomainSchema),
    defaultValues,
  });

  const watchedDomain = useWatch({ control: form.control, name: 'domain' });
  const watchedCertId = useWatch({ control: form.control, name: 'cert_id' });
  const watchedEnabled = useWatch({ control: form.control, name: 'enabled' });

  const managedDomainsQuery = useQuery({
    queryKey: managedDomainsQueryKey,
    queryFn: getManagedDomains,
  });

  const certificatesQuery = useQuery({
    queryKey: managedDomainCertificatesQueryKey,
    queryFn: getManagedDomainCertificates,
  });

  const saveMutation = useMutation({
    mutationFn: async (values: ManagedDomainFormValues) => {
      const payload = toPayload(values);
      return editingDomainId
        ? updateManagedDomain(editingDomainId, payload)
        : createManagedDomain(payload);
    },
    onSuccess: async () => {
      setFeedback({
        tone: 'success',
        message: editingDomainId ? '域名规则已更新。' : '域名规则已创建。',
      });
      setEditingDomainId(null);
      setIsEditorOpen(false);
      form.reset(defaultValues);
      await queryClient.invalidateQueries({ queryKey: managedDomainsQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteManagedDomain,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '域名规则已删除。' });
      await queryClient.invalidateQueries({ queryKey: managedDomainsQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const certificates = useMemo(() => certificatesQuery.data ?? [], [certificatesQuery.data]);
  const domains = useMemo(() => managedDomainsQuery.data ?? [], [managedDomainsQuery.data]);

  const certificateMap = useMemo(
    () => new Map(certificates.map((item) => [item.id, item])),
    [certificates],
  );

  const currentCertificate = watchedCertId ? certificateMap.get(Number(watchedCertId)) : null;

  const handleReset = () => {
    setEditingDomainId(null);
    setIsEditorOpen(false);
    setFeedback(null);
    form.reset(defaultValues);
  };

  const handleCreate = () => {
    setEditingDomainId(null);
    setFeedback(null);
    form.reset(defaultValues);
    setIsEditorOpen(true);
  };

  const handleEdit = (domain: ManagedDomainItem) => {
    setEditingDomainId(domain.id);
    setFeedback(null);
    form.reset(toFormValues(domain));
    setIsEditorOpen(true);
  };

  const handleDelete = (domain: ManagedDomainItem) => {
    if (!window.confirm(`确认删除域名规则 ${domain.domain} 吗？`)) {
      return;
    }

    setFeedback(null);
    deleteMutation.mutate(domain.id);
  };

  const handleSubmit = form.handleSubmit((values) => {
    setFeedback(null);
    saveMutation.mutate(values);
  });

  return (
    <>
    <div className='space-y-6'>
      <PageHeader
        title='域名管理'
        description='维护精确域名与通配符域名，为其绑定默认证书，并控制是否参与自动匹配与发布。'
        action={
          <PrimaryButton type='button' onClick={handleCreate}>
            新增域名
          </PrimaryButton>
        }
      />

      {feedback ? <InlineMessage tone={feedback.tone} message={feedback.message} /> : null}

      <AppCard
        title='域名规则列表'
        description='支持快速查看绑定证书、启用状态与最近更新时间。'
        action={
          <SecondaryButton
            type='button'
            onClick={() => void queryClient.invalidateQueries({ queryKey: managedDomainsQueryKey })}
          >
            刷新
          </SecondaryButton>
        }
      >
        {managedDomainsQuery.isLoading ? (
          <LoadingState />
        ) : managedDomainsQuery.isError ? (
          <ErrorState title='域名规则加载失败' description={getErrorMessage(managedDomainsQuery.error)} />
        ) : domains.length === 0 ? (
          <EmptyState title='暂无域名规则' description='请先新增精确或通配符域名规则。' />
        ) : (
          <div className='overflow-x-auto'>
            <table className='min-w-full divide-y divide-[var(--border-default)] text-left text-sm'>
              <thead>
                <tr className='text-[var(--foreground-secondary)]'>
                  <th className='px-3 py-3 font-medium'>域名</th>
                  <th className='px-3 py-3 font-medium'>类型</th>
                  <th className='px-3 py-3 font-medium'>绑定证书</th>
                  <th className='px-3 py-3 font-medium'>状态</th>
                  <th className='px-3 py-3 font-medium'>备注</th>
                  <th className='px-3 py-3 font-medium'>更新时间</th>
                  <th className='px-3 py-3 font-medium'>操作</th>
                </tr>
              </thead>
              <tbody className='divide-y divide-[var(--border-default)]'>
                {domains.map((domain) => {
                  const certificate = domain.cert_id ? certificateMap.get(domain.cert_id) : null;
                  const matchType = getMatchTypeMeta(domain.domain);

                  return (
                    <tr key={domain.id} className='align-top'>
                      <td className='px-3 py-4 font-medium text-[var(--foreground-primary)]'>{domain.domain}</td>
                      <td className='px-3 py-4'>
                        <StatusBadge label={matchType.label} variant={matchType.variant} />
                      </td>
                      <td className='px-3 py-4 text-[var(--foreground-secondary)]'>
                        {certificate ? certificate.name : '未绑定'}
                      </td>
                      <td className='px-3 py-4'>
                        <StatusBadge
                          label={domain.enabled ? '启用' : '停用'}
                          variant={domain.enabled ? 'success' : 'warning'}
                        />
                      </td>
                      <td className='px-3 py-4 text-[var(--foreground-secondary)]'>{domain.remark || '—'}</td>
                      <td className='px-3 py-4 text-[var(--foreground-secondary)]'>
                        {formatDateTime(domain.updated_at)}
                      </td>
                      <td className='px-3 py-4'>
                        <div className='flex flex-wrap gap-2'>
                          <SecondaryButton
                            type='button'
                            onClick={() => handleEdit(domain)}
                            className='px-3 py-2 text-xs'
                          >
                            编辑
                          </SecondaryButton>
                          <DangerButton
                            type='button'
                            onClick={() => handleDelete(domain)}
                            disabled={deleteMutation.isPending}
                            className='px-3 py-2 text-xs'
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
    </div>
    <AppModal
      isOpen={isEditorOpen}
      onClose={handleReset}
      title={editingDomainId ? '编辑域名规则' : '新增域名规则'}
      description='支持精确域名与单层通配符域名，证书可留空以仅维护资产关系。'
      footer={
        <div className='flex flex-wrap justify-end gap-3'>
          <SecondaryButton type='button' onClick={handleReset} disabled={saveMutation.isPending}>
            取消
          </SecondaryButton>
          <PrimaryButton type='submit' form='managed-domain-editor-form' disabled={saveMutation.isPending}>
            {saveMutation.isPending ? '保存中...' : editingDomainId ? '保存修改' : '新增域名'}
          </PrimaryButton>
        </div>
      }
    >
      <form id='managed-domain-editor-form' className='space-y-5' onSubmit={handleSubmit}>
        <div className='grid gap-4 md:grid-cols-2'>
          <ResourceField
            label='域名'
            hint='示例：example.com 或 *.example.com'
            error={form.formState.errors.domain?.message}
          >
            <ResourceInput placeholder='example.com 或 *.example.com' {...form.register('domain')} />
          </ResourceField>
          <ResourceField
            label='默认证书'
            hint='证书用于域名自动匹配与规则推荐，可不选择。'
            error={form.formState.errors.cert_id?.message}
          >
            <ResourceSelect
              value={watchedCertId}
              disabled={certificatesQuery.isLoading}
              onChange={(event) =>
                form.setValue('cert_id', event.target.value, {
                  shouldDirty: true,
                  shouldValidate: true,
                })
              }
            >
              <option value=''>不绑定证书</option>
              {certificates.map((certificate) => (
                <option key={certificate.id} value={certificate.id}>
                  {buildCertificateLabel(certificate)}
                </option>
              ))}
            </ResourceSelect>
          </ResourceField>
        </div>

        <ToggleField
          label='启用规则'
          description='停用后该域名不会参与证书自动匹配，但记录会保留。'
          checked={watchedEnabled}
          onChange={(checked) =>
            form.setValue('enabled', checked, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }
        />

        <AppCard title='匹配提示' description='自动证书推荐时，精确域名优先级高于通配符域名。'>
          <div className='space-y-3 text-sm leading-6 text-[var(--foreground-secondary)]'>
            <p>
              当前输入：
              <span className='ml-2 font-medium text-[var(--foreground-primary)]'>
                {watchedDomain.trim() || '未填写域名'}
              </span>
            </p>
            <div className='flex flex-wrap gap-2'>
              {watchedDomain.trim() ? (
                <StatusBadge {...getMatchTypeMeta(watchedDomain.trim())} />
              ) : (
                <StatusBadge label='等待输入' variant='warning' />
              )}
              <StatusBadge
                label={currentCertificate ? `证书：${currentCertificate.name}` : '未绑定证书'}
                variant={currentCertificate ? 'success' : 'warning'}
              />
            </div>
            <p>
              建议只为真正需要默认推荐的域名绑定证书；如果同一 hostname 同时命中精确和通配符规则，
              系统会优先使用精确匹配。
            </p>
          </div>
        </AppCard>

        <ResourceField
          label='备注'
          hint='可选，用于记录归属、用途或生效说明。'
          error={form.formState.errors.remark?.message}
        >
          <ResourceInput placeholder='例如：主站泛域名证书' {...form.register('remark')} />
        </ResourceField>
      </form>
    </AppModal>
    </>
  );
}
