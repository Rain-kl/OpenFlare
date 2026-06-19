'use client';

import {zodResolver} from '@hookform/resolvers/zod';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useEffect} from 'react';
import {useForm} from 'react-hook-form';
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
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {ErrorInline} from '@/components/layout/error';
import {LoadingStateWithBorder} from '@/components/layout/loading';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Textarea} from '@/components/ui/textarea';
import type {TlsCertificateItem} from '@/lib/services/openflare';
import {TlsCertificateService} from '@/lib/services/openflare';

import {defaultManualImportValues, type ManualImportFormValues, manualImportSchema,} from './schemas';
import {getErrorMessage, toManualPayload} from './website-utils';

const certificatesQueryKey = ['openflare', 'tls-certificates'];

interface CertificateEditorDialogProps {
  certificateId: number | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved?: (certificate: TlsCertificateItem) => void;
  onConvert?: (certificate: TlsCertificateItem) => void;
}

export function CertificateEditorDialog({
  certificateId,
  open,
  onOpenChange,
  onSaved,
  onConvert,
}: CertificateEditorDialogProps) {
  const queryClient = useQueryClient();
  const form = useForm<ManualImportFormValues>({
    resolver: zodResolver(manualImportSchema),
    defaultValues: defaultManualImportValues,
  });

  const certificateQuery = useQuery({
    queryKey: ['openflare', 'tls-certificates', 'content', certificateId],
    queryFn: () => TlsCertificateService.getContent(certificateId as number),
    enabled: open && certificateId !== null,
  });

  const updateMutation = useMutation({
    mutationFn: (values: ManualImportFormValues) =>
      TlsCertificateService.update(certificateId as number, toManualPayload(values)),
    onSuccess: async (certificate) => {
      await Promise.all([
        queryClient.invalidateQueries({queryKey: certificatesQueryKey}),
        queryClient.invalidateQueries({queryKey: ['openflare', 'managed-domains']}),
      ]);
      onSaved?.(certificate);
      handleClose();
    },
  });

  useEffect(() => {
    if (!open || !certificateQuery.data) return;
    form.reset({
      name: certificateQuery.data.name,
      cert_pem: certificateQuery.data.cert_pem,
      key_pem: certificateQuery.data.key_pem,
      remark: certificateQuery.data.remark || '',
    });
  }, [certificateQuery.data, form, open]);

  const handleSubmit = form.handleSubmit((values) => {
    updateMutation.mutate(values);
  });

  const handleClose = () => {
    updateMutation.reset();
    form.reset(defaultManualImportValues);
    onOpenChange(false);
  };

  const canConvert =
    certificateQuery.data?.provider === 'upload' &&
    certificateQuery.data.apply_status !== 'applying';

  return (
    <Dialog open={open} onOpenChange={(next) => !next && handleClose()}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>编辑证书</DialogTitle>
          <DialogDescription>
            可以修改证书名称、备注，以及重新上传 PEM 证书和私钥内容。
          </DialogDescription>
        </DialogHeader>

        {certificateQuery.isLoading ? (
          <LoadingStateWithBorder description="加载证书内容中..." />
        ) : certificateQuery.isError ? (
          <ErrorInline
            message={getErrorMessage(certificateQuery.error)}
            className="justify-center"
          />
        ) : !certificateQuery.data ? (
          <EmptyStateWithBorder description="证书不存在，可能已被删除。" />
        ) : (
          <form id="certificate-editor-form" className="space-y-4" onSubmit={handleSubmit}>
            {updateMutation.isError ? (
              <p className="text-sm text-destructive">
                {getErrorMessage(updateMutation.error)}
              </p>
            ) : null}

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label>证书名称</Label>
                <Input {...form.register('name')} />
              </div>
              <div className="space-y-2">
                <Label>备注</Label>
                <Input {...form.register('remark')} />
              </div>
            </div>

            <div className="space-y-2">
              <Label>证书 PEM</Label>
              <Textarea className="min-h-32 font-mono text-xs" {...form.register('cert_pem')} />
            </div>

            <div className="space-y-2">
              <Label>私钥 PEM</Label>
              <Textarea className="min-h-32 font-mono text-xs" {...form.register('key_pem')} />
            </div>

            <DialogFooter className="sm:justify-between">
              <div>
                {canConvert ? (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      if (certificateQuery.data) {
                        onConvert?.(certificateQuery.data);
                      }
                    }}
                    disabled={updateMutation.isPending}
                  >
                    转换来源
                  </Button>
                ) : null}
              </div>
              <div className="flex gap-2">
                <Button type="button" variant="outline" onClick={handleClose}>
                  取消
                </Button>
                <Button type="submit" disabled={updateMutation.isPending}>
                  {updateMutation.isPending ? (
                    <>
                      <Loader2 className="mr-1 size-3.5 animate-spin" />
                      保存中...
                    </>
                  ) : (
                    '保存证书'
                  )}
                </Button>
              </div>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
