'use client';

import {zodResolver} from '@hookform/resolvers/zod';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useEffect, useState} from 'react';
import {useForm} from 'react-hook-form';
import {ChevronDown, Loader2} from 'lucide-react';

import {Button} from '@/components/ui/button';
import {Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle,} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue,} from '@/components/ui/select';
import {Switch} from '@/components/ui/switch';
import {Textarea} from '@/components/ui/textarea';
import type {TlsCertificateItem} from '@/lib/services/openflare';
import {DnsAccountService, TlsCertificateService} from '@/lib/services/openflare';

import {type AcmeApplyFormValues, acmeApplySchema, defaultAcmeApplyValues,} from './schemas';
import {getErrorMessage} from './website-utils';

const certificatesQueryKey = ['openflare', 'tls-certificates'];

type CertificateApplyMode = 'create' | 'edit-acme' | 'convert-upload';

interface CertificateApplyDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onApplied?: (certificate: TlsCertificateItem) => void;
  mode?: CertificateApplyMode;
  certificate?: TlsCertificateItem | null;
}

export function CertificateApplyDialog({
  open,
  onOpenChange,
  onApplied,
  mode = 'create',
  certificate,
}: CertificateApplyDialogProps) {
  const queryClient = useQueryClient();
  const [error, setError] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);

  const dnsAccountsQuery = useQuery({
    queryKey: ['openflare', 'dns-accounts'],
    queryFn: () => DnsAccountService.list(),
    enabled: open,
  });

  const defaultAcmeAccountQuery = useQuery({
    queryKey: ['openflare', 'acme-accounts', 'default'],
    queryFn: () => TlsCertificateService.getDefaultAcmeAccount(),
    enabled: open,
  });

  const form = useForm<AcmeApplyFormValues>({
    resolver: zodResolver(acmeApplySchema),
    defaultValues: defaultAcmeApplyValues,
  });

  useEffect(() => {
    if (!open) return;
    setError('');
    setShowAdvanced(false);

    if (certificate) {
      form.reset({
        name: certificate.name,
        primary_domain:
          mode === 'convert-upload' ? '' : certificate.primary_domain || '',
        other_domains:
          mode === 'convert-upload' ? '' : certificate.other_domains || '',
        remark: certificate.remark || '',
        acme_account_id:
          mode === 'convert-upload' ? 0 : certificate.acme_account_id,
        dns_account_id:
          mode === 'convert-upload' ? 0 : certificate.dns_account_id,
        key_algorithm: certificate.key_algorithm || 'EC256',
        auto_renew: mode === 'convert-upload' ? true : certificate.auto_renew,
        dns1: mode === 'convert-upload' ? '' : certificate.dns1 || '',
        dns2: mode === 'convert-upload' ? '' : certificate.dns2 || '',
        disable_cname:
          mode === 'convert-upload' ? false : certificate.disable_cname,
        skip_dns: mode === 'convert-upload' ? false : certificate.skip_dns,
      });
      if (
        mode !== 'convert-upload' &&
        (certificate.dns1 ||
          certificate.dns2 ||
          certificate.disable_cname ||
          certificate.skip_dns)
      ) {
        setShowAdvanced(true);
      }
    } else {
      form.reset(defaultAcmeApplyValues);
    }
  }, [certificate, form, mode, open]);

  useEffect(() => {
    if (defaultAcmeAccountQuery.data && form.getValues('acme_account_id') === 0) {
      form.setValue('acme_account_id', defaultAcmeAccountQuery.data.id);
    }
  }, [defaultAcmeAccountQuery.data, form, open]);

  const applyMutation = useMutation({
    mutationFn: (values: AcmeApplyFormValues) => {
      if (mode === 'edit-acme' && certificate) {
        return TlsCertificateService.updateAcme(certificate.id, values);
      }
      if (mode === 'convert-upload' && certificate) {
        return TlsCertificateService.convertToAcme(certificate.id, values);
      }
      return TlsCertificateService.apply(values);
    },
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({queryKey: certificatesQueryKey});
      onApplied?.(result);
      onOpenChange(false);
    },
    onError: (err) => setError(getErrorMessage(err)),
  });

  const title =
    mode === 'edit-acme'
      ? '编辑并重新申请证书'
      : mode === 'convert-upload'
        ? '转换为申请证书'
        : '申请证书';

  const description =
    mode === 'edit-acme'
      ? '修改 ACME 证书配置。保存后将使用新配置重新申请证书。'
      : mode === 'convert-upload'
        ? '填写 ACME 申请资料。申请成功后，当前手动证书会原地转换为可自动续签的申请证书。'
        : "使用 ACME (Let's Encrypt 等) 自动申请和续期证书，支持通配符域名。";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <form
          className="space-y-4"
          onSubmit={form.handleSubmit((values) => {
            setError('');
            applyMutation.mutate(values);
          })}
        >
          {error ? <p className="text-sm text-destructive">{error}</p> : null}

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>证书名称</Label>
              <Input placeholder="例如：主站证书" {...form.register('name')} />
            </div>
            <div className="space-y-2">
              <Label>主域名</Label>
              <Input
                placeholder="example.com 或 *.example.com"
                {...form.register('primary_domain')}
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label>其他域名</Label>
            <Textarea
              rows={3}
              placeholder="example.net"
              {...form.register('other_domains')}
            />
            <p className="text-xs text-muted-foreground">
              每行一个域名。如申请通配符证书，请填写对应的根域名以便一并签发。
            </p>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>DNS 服务商账号</Label>
              <Select
                value={String(form.watch('dns_account_id') || 0)}
                onValueChange={(value) =>
                  form.setValue('dns_account_id', Number(value), {shouldValidate: true})
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="请选择 DNS 账号" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="0">请选择 DNS 账号</SelectItem>
                  {dnsAccountsQuery.data?.map((account) => (
                    <SelectItem key={account.id} value={String(account.id)}>
                      {account.name} ({account.type})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>密钥算法</Label>
              <Select
                value={form.watch('key_algorithm')}
                onValueChange={(value) => form.setValue('key_algorithm', value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="RSA2048">RSA 2048</SelectItem>
                  <SelectItem value="RSA4096">RSA 4096</SelectItem>
                  <SelectItem value="EC256">ECC 256</SelectItem>
                  <SelectItem value="EC384">ECC 384</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="space-y-2">
            <Label>备注</Label>
            <Input placeholder="可选，用于记录证书用途。" {...form.register('remark')} />
          </div>

          <div className="flex items-center justify-between rounded-lg border px-3 py-2">
            <div>
              <p className="text-sm font-medium">开启自动续签</p>
              <p className="text-xs text-muted-foreground">
                开启后，将在证书过期前 7 天自动续期。
              </p>
            </div>
            <Switch
              checked={form.watch('auto_renew')}
              onCheckedChange={(checked) => form.setValue('auto_renew', checked)}
            />
          </div>

          <div className="overflow-hidden rounded-lg border">
            <Button
              type="button"
              variant="ghost"
              className="w-full justify-between rounded-none"
              onClick={() => setShowAdvanced((current) => !current)}
            >
              高级选项
              <ChevronDown
                className={`size-4 transition-transform ${showAdvanced ? 'rotate-180' : ''}`}
              />
            </Button>
            {showAdvanced ? (
              <div className="space-y-4 border-t p-3">
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label>DNS 验证服务器 1</Label>
                    <Input placeholder="为空则使用默认权威 DNS" {...form.register('dns1')} />
                  </div>
                  <div className="space-y-2">
                    <Label>DNS 验证服务器 2</Label>
                    <Input placeholder="为空则使用默认权威 DNS" {...form.register('dns2')} />
                  </div>
                </div>
                <div className="grid gap-4 md:grid-cols-2">
                  <div className="flex items-center justify-between rounded-lg border px-3 py-2">
                    <div>
                      <p className="text-sm font-medium">跳过 CNAME 检查</p>
                      <p className="text-xs text-muted-foreground">
                        在执行 DNS-01 验证时不追踪 CNAME 记录。
                      </p>
                    </div>
                    <Switch
                      checked={form.watch('disable_cname')}
                      onCheckedChange={(checked) => form.setValue('disable_cname', checked)}
                    />
                  </div>
                  <div className="flex items-center justify-between rounded-lg border px-3 py-2">
                    <div>
                      <p className="text-sm font-medium">跳过 DNS 前置检查</p>
                      <p className="text-xs text-muted-foreground">
                        直接请求 Let&apos;s Encrypt 验证而不做本地校验。
                      </p>
                    </div>
                    <Switch
                      checked={form.watch('skip_dns')}
                      onCheckedChange={(checked) => form.setValue('skip_dns', checked)}
                    />
                  </div>
                </div>
              </div>
            ) : null}
          </div>

          <Button type="submit" disabled={applyMutation.isPending}>
            {applyMutation.isPending ? (
              <>
                <Loader2 className="mr-1 size-3.5 animate-spin" />
                提交中...
              </>
            ) : mode === 'edit-acme' ? (
              '保存并申请'
            ) : mode === 'convert-upload' ? (
              '开始转换'
            ) : (
              '开始申请'
            )}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
