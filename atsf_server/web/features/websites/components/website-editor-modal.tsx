'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';
import { useForm, useWatch } from 'react-hook-form';

import { InlineMessage } from '@/components/feedback/inline-message';
import { AppModal } from '@/components/ui/app-modal';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  createManagedDomain,
  updateManagedDomain,
} from '@/features/managed-domains/api/managed-domains';
import type { ManagedDomainItem } from '@/features/managed-domains/types';
import type { TlsCertificateItem } from '@/features/tls-certificates/types';
import {
  defaultManagedDomainValues,
  managedDomainSchema,
  type ManagedDomainFormValues,
} from '@/features/websites/schemas';
import {
  buildCertificateLabel,
  getErrorMessage,
  getMatchTypeMeta,
  toManagedDomainFormValues,
  toManagedDomainPayload,
} from '@/features/websites/utils';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';

interface WebsiteEditorModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved?: (domain: ManagedDomainItem, mode: 'create' | 'update') => void;
  onRequestImportCertificate: () => void;
  certificates: TlsCertificateItem[];
  certificatesLoading?: boolean;
  initialDomain?: ManagedDomainItem | null;
  preferredCertificateId?: number | null;
}

export function WebsiteEditorModal({
  isOpen,
  onClose,
  onSaved,
  onRequestImportCertificate,
  certificates,
  certificatesLoading = false,
  initialDomain = null,
  preferredCertificateId = null,
}: WebsiteEditorModalProps) {
  const queryClient = useQueryClient();
  const form = useForm<ManagedDomainFormValues>({
    resolver: zodResolver(managedDomainSchema),
    defaultValues: defaultManagedDomainValues,
  });

  const watchedDomain = useWatch({
    control: form.control,
    name: 'domain',
  });
  const watchedCertId = useWatch({
    control: form.control,
    name: 'cert_id',
  });
  const watchedEnabled = useWatch({
    control: form.control,
    name: 'enabled',
  });

  const saveMutation = useMutation({
    mutationFn: async (values: ManagedDomainFormValues) => {
      const payload = toManagedDomainPayload(values);
      return initialDomain
        ? updateManagedDomain(initialDomain.id, payload)
        : createManagedDomain(payload);
    },
    onSuccess: async (domain) => {
      await queryClient.invalidateQueries({ queryKey: ['managed-domains'] });
      onSaved?.(domain, initialDomain ? 'update' : 'create');
      handleClose();
    },
  });

  const currentCertificate = watchedCertId
    ? certificates.find((item) => item.id === Number(watchedCertId)) ?? null
    : null;

  const handleSubmit = form.handleSubmit((values) => {
    saveMutation.mutate(values);
  });

  const handleClose = () => {
    saveMutation.reset();
    form.reset(defaultManagedDomainValues);
    onClose();
  };

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    saveMutation.reset();
    form.reset(
      initialDomain
        ? toManagedDomainFormValues(initialDomain)
        : defaultManagedDomainValues,
    );
  }, [form, initialDomain, isOpen]);

  useEffect(() => {
    if (!isOpen || !preferredCertificateId) {
      return;
    }

    form.setValue('cert_id', String(preferredCertificateId), {
      shouldDirty: true,
      shouldValidate: true,
    });
  }, [form, isOpen, preferredCertificateId]);

  return (
    <AppModal
      isOpen={isOpen}
      onClose={handleClose}
      title={initialDomain ? '编辑网站' : '新增网站'}
      description="录入域名并选择绑定证书。证书可在弹窗内直接新增，导入成功后会自动回填。"
      footer={
        <div className="flex flex-wrap justify-end gap-3">
          <SecondaryButton
            type="button"
            onClick={handleClose}
            disabled={saveMutation.isPending}
          >
            取消
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            form="website-editor-form"
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending
              ? '保存中...'
              : initialDomain
                ? '保存网站'
                : '创建网站'}
          </PrimaryButton>
        </div>
      }
    >
      <form
        id="website-editor-form"
        className="space-y-5"
        onSubmit={handleSubmit}
      >
        {saveMutation.isError ? (
          <InlineMessage
            tone="danger"
            message={getErrorMessage(saveMutation.error)}
          />
        ) : null}

        <div className="grid gap-4 md:grid-cols-2">
          <ResourceField
            label="域名"
            hint="示例：example.com 或 *.example.com"
            error={form.formState.errors.domain?.message}
          >
            <ResourceInput
              placeholder="example.com 或 *.example.com"
              {...form.register('domain')}
            />
          </ResourceField>

          <ResourceField
            label="绑定证书"
            hint="默认证书会用于该网站的自动匹配与规则推荐。"
            error={form.formState.errors.cert_id?.message}
          >
            <div className="space-y-3">
              <ResourceSelect
                value={watchedCertId}
                disabled={certificatesLoading}
                onChange={(event) =>
                  form.setValue('cert_id', event.target.value, {
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
              <SecondaryButton
                type="button"
                onClick={onRequestImportCertificate}
                className="w-full sm:w-auto"
              >
                添加证书
              </SecondaryButton>
            </div>
          </ResourceField>
        </div>

        <div className="grid gap-4 md:grid-cols-3">
          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]">
              当前域名
            </p>
            <p className="mt-2 text-sm text-[var(--foreground-primary)]">
              {watchedDomain?.trim() || '未填写域名'}
            </p>
          </div>
          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]">
              匹配类型
            </p>
            <div className="mt-2">
              {watchedDomain?.trim() ? (
                <StatusBadge {...getMatchTypeMeta(watchedDomain.trim())} />
              ) : (
                <StatusBadge label="等待输入" variant="warning" />
              )}
            </div>
          </div>
          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
            <p className="text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]">
              当前证书
            </p>
            <div className="mt-2">
              <StatusBadge
                label={currentCertificate ? currentCertificate.name : '未绑定证书'}
                variant={currentCertificate ? 'success' : 'warning'}
              />
            </div>
          </div>
        </div>

        <ToggleField
          label="启用网站"
          description="停用后该网站不会参与自动匹配，但记录会保留。"
          checked={watchedEnabled}
          onChange={(checked) =>
            form.setValue('enabled', checked, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }
        />

        <ResourceField
          label="备注"
          error={form.formState.errors.remark?.message}
        >
          <ResourceInput
            placeholder="例如：主站 / 泛域名 / 生产"
            {...form.register('remark')}
          />
        </ResourceField>
      </form>
    </AppModal>
  );
}
