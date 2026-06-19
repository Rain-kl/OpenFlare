'use client';

import {zodResolver} from '@hookform/resolvers/zod';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {useEffect} from 'react';
import {useForm, useWatch} from 'react-hook-form';
import {Loader2} from 'lucide-react';

import {Button} from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from '@/components/ui/select';
import {Switch} from '@/components/ui/switch';
import type {ManagedDomainItem, TlsCertificateItem} from '@/lib/services/openflare';
import {WebsiteService} from '@/lib/services/openflare';

import {defaultManagedDomainValues, type ManagedDomainFormValues, managedDomainSchema,} from './schemas';
import {WebsiteStatusBadge} from './status-badge';
import {
  buildCertificateLabel,
  getErrorMessage,
  getMatchTypeMeta,
  toManagedDomainFormValues,
  toManagedDomainPayload,
} from './website-utils';

const domainsQueryKey = ['openflare', 'managed-domains'];

interface WebsiteEditorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved?: (domain: ManagedDomainItem, mode: 'create' | 'update') => void;
  onRequestImportCertificate: () => void;
  certificates: TlsCertificateItem[];
  certificatesLoading?: boolean;
  initialDomain?: ManagedDomainItem | null;
  preferredCertificateId?: number | null;
}

export function WebsiteEditorDialog({
  open,
  onOpenChange,
  onSaved,
  onRequestImportCertificate,
  certificates,
  certificatesLoading = false,
  initialDomain = null,
  preferredCertificateId = null,
}: WebsiteEditorDialogProps) {
  const queryClient = useQueryClient();
  const form = useForm<ManagedDomainFormValues>({
    resolver: zodResolver(managedDomainSchema),
    defaultValues: defaultManagedDomainValues,
  });

  const watchedDomain = useWatch({control: form.control, name: 'domain'});
  const watchedCertId = useWatch({control: form.control, name: 'cert_id'});
  const watchedEnabled = useWatch({control: form.control, name: 'enabled'});

  const saveMutation = useMutation({
    mutationFn: async (values: ManagedDomainFormValues) => {
      const payload = toManagedDomainPayload(values);
      return initialDomain
        ? WebsiteService.update(initialDomain.id, payload)
        : WebsiteService.create(payload);
    },
    onSuccess: async (domain) => {
      await queryClient.invalidateQueries({queryKey: domainsQueryKey});
      onSaved?.(domain, initialDomain ? 'update' : 'create');
      handleClose();
    },
  });

  const currentCertificate = watchedCertId
    ? (certificates.find((item) => item.id === Number(watchedCertId)) ?? null)
    : null;

  const handleSubmit = form.handleSubmit((values) => {
    saveMutation.mutate(values);
  });

  const handleClose = () => {
    saveMutation.reset();
    form.reset(defaultManagedDomainValues);
    onOpenChange(false);
  };

  useEffect(() => {
    if (!open) return;
    form.reset(
      initialDomain
        ? toManagedDomainFormValues(initialDomain)
        : defaultManagedDomainValues,
    );
  }, [form, initialDomain, open]);

  useEffect(() => {
    if (!open || !preferredCertificateId) return;
    form.setValue('cert_id', String(preferredCertificateId), {
      shouldDirty: true,
      shouldValidate: true,
    });
  }, [form, open, preferredCertificateId]);

  return (
    <Dialog open={open} onOpenChange={(next) => !next && handleClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{initialDomain ? '编辑网站' : '新增网站'}</DialogTitle>
          <DialogDescription>
            录入域名并选择绑定证书。证书可在弹窗内直接新增，导入成功后会自动回填。
          </DialogDescription>
        </DialogHeader>

        <form className="space-y-4" onSubmit={handleSubmit}>
          {saveMutation.isError ? (
            <p className="text-sm text-destructive">{getErrorMessage(saveMutation.error)}</p>
          ) : null}

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="domain">域名</Label>
              <Input
                id="domain"
                placeholder="example.com 或 *.example.com"
                {...form.register('domain')}
              />
              {form.formState.errors.domain ? (
                <p className="text-xs text-destructive">
                  {form.formState.errors.domain.message}
                </p>
              ) : null}
            </div>

            <div className="space-y-2">
              <Label>绑定证书</Label>
              <Select
                value={watchedCertId || 'none'}
                disabled={certificatesLoading}
                onValueChange={(value) =>
                  form.setValue('cert_id', value === 'none' ? '' : value, {
                    shouldDirty: true,
                    shouldValidate: true,
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="不绑定证书" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">不绑定证书</SelectItem>
                  {certificates.map((certificate) => (
                    <SelectItem key={certificate.id} value={String(certificate.id)}>
                      {buildCertificateLabel(certificate)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 text-xs"
                onClick={onRequestImportCertificate}
              >
                添加证书
              </Button>
            </div>
          </div>

          <div className="grid gap-3 rounded-lg border border-dashed p-3 md:grid-cols-3">
            <div>
              <p className="text-[10px] uppercase tracking-wider text-muted-foreground">当前域名</p>
              <p className="mt-1 text-sm">{watchedDomain?.trim() || '未填写域名'}</p>
            </div>
            <div>
              <p className="text-[10px] uppercase tracking-wider text-muted-foreground">匹配类型</p>
              <div className="mt-1">
                {watchedDomain?.trim() ? (
                  <WebsiteStatusBadge {...getMatchTypeMeta(watchedDomain.trim())} />
                ) : (
                  <WebsiteStatusBadge label="等待输入" tone="warning" />
                )}
              </div>
            </div>
            <div>
              <p className="text-[10px] uppercase tracking-wider text-muted-foreground">当前证书</p>
              <div className="mt-1">
                <WebsiteStatusBadge
                  label={currentCertificate ? currentCertificate.name : '未绑定证书'}
                  tone={currentCertificate ? 'success' : 'warning'}
                />
              </div>
            </div>
          </div>

          <div className="flex items-center justify-between rounded-lg border px-3 py-2">
            <div>
              <p className="text-sm font-medium">启用网站</p>
              <p className="text-xs text-muted-foreground">
                停用后该网站不会参与自动匹配，但记录会保留。
              </p>
            </div>
            <Switch
              checked={watchedEnabled}
              onCheckedChange={(checked) =>
                form.setValue('enabled', checked, {shouldDirty: true, shouldValidate: true})
              }
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="remark">备注</Label>
            <Input
              id="remark"
              placeholder="例如：主站 / 泛域名 / 生产"
              {...form.register('remark')}
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={handleClose}>
              取消
            </Button>
            <Button type="submit" disabled={saveMutation.isPending}>
              {saveMutation.isPending ? (
                <>
                  <Loader2 className="mr-1 size-3.5 animate-spin" />
                  保存中...
                </>
              ) : initialDomain ? (
                '保存网站'
              ) : (
                '创建网站'
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
