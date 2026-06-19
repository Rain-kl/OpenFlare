'use client';

import {zodResolver} from '@hookform/resolvers/zod';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {type FormEvent, useEffect, useState} from 'react';
import {useForm} from 'react-hook-form';
import {Loader2} from 'lucide-react';

import {Button} from '@/components/ui/button';
import {Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle,} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {Textarea} from '@/components/ui/textarea';
import type {TlsCertificateItem} from '@/lib/services/openflare';
import {TlsCertificateService} from '@/lib/services/openflare';

import {
  defaultFileImportValues,
  defaultManualImportValues,
  type FileImportFormValues,
  type ManualImportFormValues,
  manualImportSchema,
} from './schemas';
import {getErrorMessage, toFilePayload, toManualPayload} from './website-utils';

const certificatesQueryKey = ['openflare', 'tls-certificates'];

interface CertificateImportDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onImported?: (certificate: TlsCertificateItem) => void;
}

export function CertificateImportDialog({
  open,
  onOpenChange,
  onImported,
}: CertificateImportDialogProps) {
  const queryClient = useQueryClient();
  const [importMode, setImportMode] = useState<'manual' | 'file'>('manual');
  const [error, setError] = useState('');
  const [fileForm, setFileForm] = useState<FileImportFormValues>(defaultFileImportValues);
  const [certFile, setCertFile] = useState<File | null>(null);
  const [keyFile, setKeyFile] = useState<File | null>(null);
  const [fileInputNonce, setFileInputNonce] = useState(0);

  const manualForm = useForm<ManualImportFormValues>({
    resolver: zodResolver(manualImportSchema),
    defaultValues: defaultManualImportValues,
  });

  useEffect(() => {
    if (!open) return;
    setError('');
  }, [open]);

  const invalidateQueries = async () => {
    await Promise.all([
      queryClient.invalidateQueries({queryKey: certificatesQueryKey}),
      queryClient.invalidateQueries({queryKey: ['openflare', 'managed-domains']}),
    ]);
  };

  const resetFileForm = () => {
    setFileForm(defaultFileImportValues);
    setCertFile(null);
    setKeyFile(null);
    setFileInputNonce((value) => value + 1);
  };

  const resetAll = () => {
    setImportMode('manual');
    setError('');
    manualForm.reset(defaultManualImportValues);
    resetFileForm();
  };

  const handleClose = () => {
    resetAll();
    onOpenChange(false);
  };

  const manualImportMutation = useMutation({
    mutationFn: (values: ManualImportFormValues) =>
      TlsCertificateService.create(toManualPayload(values)),
    onSuccess: async (certificate) => {
      await invalidateQueries();
      onImported?.(certificate);
      handleClose();
    },
    onError: (err) => setError(getErrorMessage(err)),
  });

  const fileImportMutation = useMutation({
    mutationFn: (values: FileImportFormValues) =>
      TlsCertificateService.importFile(toFilePayload(values, certFile, keyFile)),
    onSuccess: async (certificate) => {
      await invalidateQueries();
      onImported?.(certificate);
      handleClose();
    },
    onError: (err) => setError(getErrorMessage(err)),
  });

  const handleManualSubmit = manualForm.handleSubmit((values) => {
    setError('');
    manualImportMutation.mutate(values);
  });

  const handleFileSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError('');
    fileImportMutation.mutate(fileForm);
  };

  const pending = manualImportMutation.isPending || fileImportMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={(next) => !next && handleClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>添加证书</DialogTitle>
          <DialogDescription>
            支持手动粘贴 PEM 或上传证书文件。导入成功后可立即在网站表单里选择。
          </DialogDescription>
        </DialogHeader>

        {error ? <p className="text-sm text-destructive">{error}</p> : null}

        <Tabs value={importMode} onValueChange={(value) => setImportMode(value as 'manual' | 'file')}>
          <TabsList className="grid w-full grid-cols-2">
            <TabsTrigger value="manual">手动导入</TabsTrigger>
            <TabsTrigger value="file">文件导入</TabsTrigger>
          </TabsList>

          <TabsContent value="manual" className="space-y-4">
            <form className="space-y-4" onSubmit={handleManualSubmit}>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label>证书名称</Label>
                  <Input placeholder="example-com" {...manualForm.register('name')} />
                  {manualForm.formState.errors.name ? (
                    <p className="text-xs text-destructive">
                      {manualForm.formState.errors.name.message}
                    </p>
                  ) : null}
                </div>
                <div className="space-y-2">
                  <Label>备注</Label>
                  <Input placeholder="例如：主站生产证书" {...manualForm.register('remark')} />
                </div>
              </div>

              <div className="space-y-2">
                <Label>证书 PEM</Label>
                <Textarea
                  className="min-h-32 font-mono text-xs"
                  placeholder="-----BEGIN CERTIFICATE-----"
                  {...manualForm.register('cert_pem')}
                />
              </div>

              <div className="space-y-2">
                <Label>私钥 PEM</Label>
                <Textarea
                  className="min-h-32 font-mono text-xs"
                  placeholder="-----BEGIN PRIVATE KEY-----"
                  {...manualForm.register('key_pem')}
                />
              </div>

              <Button type="submit" disabled={pending}>
                {pending ? (
                  <>
                    <Loader2 className="mr-1 size-3.5 animate-spin" />
                    导入中...
                  </>
                ) : (
                  '导入证书'
                )}
              </Button>
            </form>
          </TabsContent>

          <TabsContent value="file" className="space-y-4">
            <form className="space-y-4" onSubmit={handleFileSubmit}>
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label>证书名称</Label>
                  <Input
                    value={fileForm.name}
                    onChange={(event) =>
                      setFileForm((current) => ({...current, name: event.target.value}))
                    }
                    placeholder="wildcard-example"
                  />
                </div>
                <div className="space-y-2">
                  <Label>备注</Label>
                  <Input
                    value={fileForm.remark}
                    onChange={(event) =>
                      setFileForm((current) => ({...current, remark: event.target.value}))
                    }
                    placeholder="例如：泛域名生产证书"
                  />
                </div>
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label>证书文件</Label>
                  <Input
                    key={`cert-${fileInputNonce}`}
                    type="file"
                    accept=".pem,.crt,.cer"
                    onChange={(event) => setCertFile(event.target.files?.[0] ?? null)}
                  />
                  <p className="text-xs text-muted-foreground">
                    {certFile ? `已选择：${certFile.name}` : '请选择 PEM/CRT 文件'}
                  </p>
                </div>
                <div className="space-y-2">
                  <Label>私钥文件</Label>
                  <Input
                    key={`key-${fileInputNonce}`}
                    type="file"
                    accept=".key,.pem"
                    onChange={(event) => setKeyFile(event.target.files?.[0] ?? null)}
                  />
                  <p className="text-xs text-muted-foreground">
                    {keyFile ? `已选择：${keyFile.name}` : '请选择 KEY/PEM 文件'}
                  </p>
                </div>
              </div>

              <div className="flex gap-2">
                <Button type="submit" disabled={pending}>
                  {pending ? (
                    <>
                      <Loader2 className="mr-1 size-3.5 animate-spin" />
                      上传中...
                    </>
                  ) : (
                    '上传文件'
                  )}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={pending}
                  onClick={resetFileForm}
                >
                  清空文件
                </Button>
              </div>
            </form>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
