'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { AppModal } from '@/components/ui/app-modal';
import {
  getTlsCertificateContent,
  updateTlsCertificate,
} from '@/features/tls-certificates/api/tls-certificates';
import type { TlsCertificateItem } from '@/features/tls-certificates/types';
import {
  defaultManualImportValues,
  manualImportSchema,
  type ManualImportFormValues,
} from '@/features/websites/schemas';
import { getErrorMessage, toManualPayload } from '@/features/websites/utils';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceTextarea,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';

interface CertificateEditorModalProps {
  certificateId: number | null;
  isOpen: boolean;
  onClose: () => void;
  onSaved?: (certificate: TlsCertificateItem) => void;
  onConvert?: (certificate: TlsCertificateItem) => void;
}

export function CertificateEditorModal({
  certificateId,
  isOpen,
  onClose,
  onSaved,
  onConvert,
}: CertificateEditorModalProps) {
  const queryClient = useQueryClient();
  const form = useForm<ManualImportFormValues>({
    resolver: zodResolver(manualImportSchema),
    defaultValues: defaultManualImportValues,
  });

  const certificateQuery = useQuery({
    queryKey: ['tls-certificates', 'content', certificateId],
    queryFn: () => getTlsCertificateContent(certificateId as number),
    enabled: isOpen && certificateId !== null,
  });

  const updateMutation = useMutation({
    mutationFn: async (values: ManualImportFormValues) =>
      updateTlsCertificate(certificateId as number, toManualPayload(values)),
    onSuccess: async (certificate) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['tls-certificates'] }),
        queryClient.invalidateQueries({ queryKey: ['managed-domains'] }),
      ]);
      onSaved?.(certificate);
      handleClose();
    },
  });

  useEffect(() => {
    if (!isOpen || !certificateQuery.data) {
      return;
    }

    form.reset({
      name: certificateQuery.data.name,
      cert_pem: certificateQuery.data.cert_pem,
      key_pem: certificateQuery.data.key_pem,
      remark: certificateQuery.data.remark || '',
    });
  }, [certificateQuery.data, form, isOpen]);

  const handleSubmit = form.handleSubmit((values) => {
    updateMutation.mutate(values);
  });

  const handleClose = () => {
    updateMutation.reset();
    form.reset(defaultManualImportValues);
    onClose();
  };

  const canConvert =
    certificateQuery.data?.provider === 'upload' &&
    certificateQuery.data.apply_status !== 'applying';

  return (
    <AppModal
      isOpen={isOpen}
      onClose={handleClose}
      title="编辑证书"
      description="可以修改证书名称、备注，以及重新上传 PEM 证书和私钥内容。"
      size="xl"
      footer={
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            {canConvert ? (
              <SecondaryButton
                type="button"
                onClick={() => {
                  if (certificateQuery.data) {
                    onConvert?.(certificateQuery.data);
                  }
                }}
                disabled={updateMutation.isPending}
              >
                转换来源
              </SecondaryButton>
            ) : null}
          </div>
          <div className="flex flex-wrap justify-end gap-3">
            <SecondaryButton
              type="button"
              onClick={handleClose}
              disabled={updateMutation.isPending}
            >
              取消
            </SecondaryButton>
            <PrimaryButton
              type="submit"
              form="certificate-editor-form"
              disabled={updateMutation.isPending || certificateQuery.isLoading}
            >
              {updateMutation.isPending ? '保存中...' : '保存证书'}
            </PrimaryButton>
          </div>
        </div>
      }
    >
      {certificateQuery.isLoading ? (
        <LoadingState />
      ) : certificateQuery.isError ? (
        <ErrorState
          title="证书内容加载失败"
          description={getErrorMessage(certificateQuery.error)}
        />
      ) : !certificateQuery.data ? (
        <EmptyState title="证书不存在" description="当前证书可能已被删除。" />
      ) : (
        <form
          id="certificate-editor-form"
          className="space-y-5"
          onSubmit={handleSubmit}
        >
          {updateMutation.isError ? (
            <InlineMessage
              tone="danger"
              message={getErrorMessage(updateMutation.error)}
            />
          ) : null}

          <div className="grid gap-4 md:grid-cols-2">
            <ResourceField
              label="证书名称"
              error={form.formState.errors.name?.message}
            >
              <ResourceInput {...form.register('name')} />
            </ResourceField>
            <ResourceField
              label="备注"
              error={form.formState.errors.remark?.message}
            >
              <ResourceInput {...form.register('remark')} />
            </ResourceField>
          </div>

          <ResourceField
            label="证书 PEM"
            error={form.formState.errors.cert_pem?.message}
          >
            <ResourceTextarea
              className="min-h-40 font-mono text-xs"
              {...form.register('cert_pem')}
            />
          </ResourceField>

          <ResourceField
            label="私钥 PEM"
            error={form.formState.errors.key_pem?.message}
          >
            <ResourceTextarea
              className="min-h-40 font-mono text-xs"
              {...form.register('key_pem')}
            />
          </ResourceField>
        </form>
      )}
    </AppModal>
  );
}
