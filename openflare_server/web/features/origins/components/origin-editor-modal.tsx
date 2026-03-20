'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { AppModal } from '@/components/ui/app-modal';
import { createOrigin, updateOrigin } from '@/features/origins/api/origins';
import type {
  OriginItem,
  OriginMutationPayload,
} from '@/features/origins/types';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceTextarea,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';

const originSchema = z.object({
  name: z.string().max(255, '源站名不能超过 255 个字符'),
  address: z
    .string()
    .trim()
    .min(1, '请输入源站地址')
    .refine(
      (value) => !/[/?#]/.test(value) && !value.includes('://'),
      '源站地址格式不合法',
    ),
  remark: z.string().max(255, '备注不能超过 255 个字符'),
});

type OriginFormValues = z.infer<typeof originSchema>;

function toPayload(values: OriginFormValues): OriginMutationPayload {
  return {
    name: values.name.trim(),
    address: values.address.trim(),
    remark: values.remark.trim(),
  };
}

function toFormValues(origin?: OriginItem | null): OriginFormValues {
  if (!origin) {
    return {
      name: '',
      address: '',
      remark: '',
    };
  }
  return {
    name: origin.name,
    address: origin.address,
    remark: origin.remark || '',
  };
}

export function OriginEditorModal({
  isOpen,
  onClose,
  origin,
  onSaved,
}: {
  isOpen: boolean;
  onClose: () => void;
  origin?: OriginItem | null;
  onSaved?: (origin: OriginItem, mode: 'create' | 'update') => void;
}) {
  const queryClient = useQueryClient();
  const form = useForm<OriginFormValues>({
    resolver: zodResolver(originSchema),
    defaultValues: toFormValues(origin),
  });

  useEffect(() => {
    form.reset(toFormValues(origin));
  }, [form, origin, isOpen]);

  const mutation = useMutation({
    mutationFn: async (values: OriginFormValues) => {
      const payload = toPayload(values);
      return origin ? updateOrigin(origin.id, payload) : createOrigin(payload);
    },
    onSuccess: async (savedOrigin) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['origins'] }),
        queryClient.invalidateQueries({ queryKey: ['proxy-routes'] }),
      ]);
      onSaved?.(savedOrigin, origin ? 'update' : 'create');
      onClose();
    },
  });

  const handleSubmit = form.handleSubmit((values) => {
    mutation.mutate(values);
  });

  return (
    <AppModal
      isOpen={isOpen}
      onClose={onClose}
      title={origin ? '编辑源站' : '新增源站'}
      description="源站会作为规则里的可复用地址目录，协议和端口仍由规则决定。"
      footer={
        <div className="flex flex-wrap justify-end gap-3">
          <SecondaryButton type="button" onClick={onClose}>
            取消
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            form="origin-editor-form"
            disabled={mutation.isPending}
          >
            {mutation.isPending
              ? '保存中...'
              : origin
                ? '保存修改'
                : '新增源站'}
          </PrimaryButton>
        </div>
      }
    >
      <form
        id="origin-editor-form"
        className="space-y-5"
        onSubmit={handleSubmit}
      >
        <div className="grid gap-4 md:grid-cols-2">
          <ResourceField
            label="源站地址"
            hint="支持 IP、域名或主机名，例如 10.0.0.10、origin.internal。"
            error={form.formState.errors.address?.message}
          >
            <ResourceInput
              placeholder="origin.internal"
              {...form.register('address')}
            />
          </ResourceField>
          <ResourceField
            label="源站名"
            hint="可选，留空时默认使用源站地址。"
            error={form.formState.errors.name?.message}
          >
            <ResourceInput placeholder="主站源站" {...form.register('name')} />
          </ResourceField>
        </div>

        <ResourceField
          label="备注"
          error={form.formState.errors.remark?.message}
        >
          <ResourceTextarea
            placeholder="例如：主站内网入口"
            {...form.register('remark')}
          />
        </ResourceField>

        {mutation.isError ? (
          <p className="text-sm text-[var(--status-danger-foreground)]">
            {mutation.error instanceof Error
              ? mutation.error.message
              : '请求失败，请稍后重试。'}
          </p>
        ) : null}
      </form>
    </AppModal>
  );
}
