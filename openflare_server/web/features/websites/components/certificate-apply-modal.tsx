'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';

import { InlineMessage } from '@/components/feedback/inline-message';
import { AppModal } from '@/components/ui/app-modal';
import { applyTlsCertificate, updateAcmeCertificate } from '@/features/tls-certificates/api/tls-certificates';
import type { TlsCertificateItem } from '@/features/tls-certificates/types';
import { getDnsAccounts } from '@/features/dns-accounts/api/dns-accounts';
import { getDefaultAcmeAccount } from '@/features/acme-accounts/api/acme-accounts';
import {
  acmeApplySchema,
  defaultAcmeApplyValues,
  type AcmeApplyFormValues,
} from '@/features/websites/schemas';
import { getErrorMessage } from '@/features/websites/utils';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  ToggleField,
} from '@/features/shared/components/resource-primitives';

interface CertificateApplyModalProps {
  isOpen: boolean;
  onClose: () => void;
  onApplied?: (certificate: TlsCertificateItem) => void;
  editCertificate?: TlsCertificateItem | null;
}

export function CertificateApplyModal({
  isOpen,
  onClose,
  onApplied,
  editCertificate,
}: CertificateApplyModalProps) {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<{ tone: 'success' | 'danger'; message: string } | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);

  const dnsAccountsQuery = useQuery({
    queryKey: ['dns-accounts'],
    queryFn: getDnsAccounts,
    enabled: isOpen,
  });

  const defaultAcmeAccountQuery = useQuery({
    queryKey: ['acme-accounts', 'default'],
    queryFn: getDefaultAcmeAccount,
    enabled: isOpen,
  });

  const form = useForm<AcmeApplyFormValues>({
    resolver: zodResolver(acmeApplySchema),
    defaultValues: defaultAcmeApplyValues,
  });

  useEffect(() => {
    if (!isOpen) return;
    setFeedback(null);
    setShowAdvanced(false);
    
    if (editCertificate) {
      form.reset({
        name: editCertificate.name,
        primary_domain: editCertificate.primary_domain || '',
        other_domains: editCertificate.other_domains || '',
        remark: editCertificate.remark || '',
        acme_account_id: editCertificate.acme_account_id,
        dns_account_id: editCertificate.dns_account_id,
        key_algorithm: editCertificate.key_algorithm as any || 'EC256',
        auto_renew: editCertificate.auto_renew,
        dns1: editCertificate.dns1 || '',
        dns2: editCertificate.dns2 || '',
        disable_cname: editCertificate.disable_cname,
        skip_dns: editCertificate.skip_dns,
      });
      if (editCertificate.dns1 || editCertificate.dns2 || editCertificate.disable_cname || editCertificate.skip_dns) {
        setShowAdvanced(true);
      }
    } else {
      form.reset(defaultAcmeApplyValues);
    }
  }, [isOpen, form, editCertificate]);

  useEffect(() => {
    if (defaultAcmeAccountQuery.data) {
      form.setValue('acme_account_id', defaultAcmeAccountQuery.data.id);
    }
  }, [defaultAcmeAccountQuery.data, form]);

  const applyMutation = useMutation({
    mutationFn: (values: AcmeApplyFormValues) => 
      editCertificate 
        ? updateAcmeCertificate(editCertificate.id, values)
        : applyTlsCertificate(values),
    onSuccess: async (certificate) => {
      await queryClient.invalidateQueries({ queryKey: ['tls-certificates'] });
      onApplied?.(certificate);
      onClose();
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const onSubmit = form.handleSubmit((values) => {
    setFeedback(null);
    applyMutation.mutate(values);
  });

  return (
    <AppModal
      isOpen={isOpen}
      onClose={onClose}
      title={editCertificate ? "编辑并重新申请证书" : "申请证书"}
      description={editCertificate ? "修改 ACME 证书配置。保存后将使用新配置重新申请证书。" : "使用 ACME (Let's Encrypt 等) 自动申请和续期证书，支持通配符域名。"}
      size="xl"
    >
      <form className="space-y-5" onSubmit={onSubmit}>
        {feedback ? (
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        <div className="grid gap-4 md:grid-cols-2">
          <ResourceField
            label="证书名称"
            error={form.formState.errors.name?.message}
          >
            <ResourceInput placeholder="例如：主站证书" {...form.register('name')} />
          </ResourceField>
          <ResourceField
            label="主域名"
            error={form.formState.errors.primary_domain?.message}
          >
            <ResourceInput placeholder="example.com 或 *.example.com" {...form.register('primary_domain')} />
          </ResourceField>
        </div>

        <ResourceField
          label="其他域名"
          hint="每行一个域名。如申请通配符证书，请填写对应的根域名以便一并签发。"
          error={form.formState.errors.other_domains?.message}
        >
          <textarea
            className="w-full rounded-xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm text-[var(--foreground-primary)] outline-none transition focus:border-[var(--brand-primary)]"
            rows={3}
            placeholder="example.net"
            {...form.register('other_domains')}
          />
        </ResourceField>

        <div className="grid gap-4 md:grid-cols-2">
          <ResourceField
            label="DNS 服务商账号"
            error={form.formState.errors.dns_account_id?.message}
          >
            <ResourceSelect {...form.register('dns_account_id')}>
              <option value={0}>请选择 DNS 账号</option>
              {dnsAccountsQuery.data?.map((acc) => (
                <option key={acc.id} value={acc.id}>
                  {acc.name} ({acc.type})
                </option>
              ))}
            </ResourceSelect>
          </ResourceField>

          <ResourceField
            label="密钥算法"
            error={form.formState.errors.key_algorithm?.message}
          >
            <ResourceSelect {...form.register('key_algorithm')}>
              <option value="RSA2048">RSA 2048</option>
              <option value="RSA4096">RSA 4096</option>
              <option value="EC256">ECC 256</option>
              <option value="EC384">ECC 384</option>
            </ResourceSelect>
          </ResourceField>
        </div>

        <ResourceField
          label="备注"
          error={form.formState.errors.remark?.message}
        >
          <ResourceInput placeholder="可选，用于记录证书用途。" {...form.register('remark')} />
        </ResourceField>

        <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] overflow-hidden">
          <button
            type="button"
            className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--surface-muted)]"
            onClick={() => setShowAdvanced(!showAdvanced)}
          >
            <span>高级选项</span>
            <svg
              className={`h-4 w-4 transition-transform duration-200 ${showAdvanced ? 'rotate-180' : ''}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          {showAdvanced && (
            <div className="space-y-4 border-t border-[var(--border-default)] px-4 py-4">
              <div className="grid gap-4 md:grid-cols-2">
                <ResourceField
                  label="DNS 验证服务器 1"
                  hint="可选，如 8.8.8.8"
                  error={form.formState.errors.dns1?.message}
                >
                  <ResourceInput placeholder="为空则使用默认权威 DNS" {...form.register('dns1')} />
                </ResourceField>
                <ResourceField
                  label="DNS 验证服务器 2"
                  hint="可选，如 1.1.1.1"
                  error={form.formState.errors.dns2?.message}
                >
                  <ResourceInput placeholder="为空则使用默认权威 DNS" {...form.register('dns2')} />
                </ResourceField>
              </div>
              <div className="grid gap-4 md:grid-cols-2">
                <ToggleField
                  label="跳过 CNAME 检查"
                  description="在执行 DNS-01 验证时不追踪 CNAME 记录。"
                  checked={form.watch('disable_cname')}
                  onChange={(checked) => form.setValue('disable_cname', checked)}
                />
                <ToggleField
                  label="跳过 DNS 前置检查"
                  description="直接请求 Let's Encrypt 验证而不做本地校验。"
                  checked={form.watch('skip_dns')}
                  onChange={(checked) => form.setValue('skip_dns', checked)}
                />
              </div>
            </div>
          )}
        </div>

        <PrimaryButton type="submit" disabled={applyMutation.isPending}>
          {applyMutation.isPending ? '提交中...' : (editCertificate ? '保存并申请' : '开始申请')}
        </PrimaryButton>
      </form>
    </AppModal>
  );
}
