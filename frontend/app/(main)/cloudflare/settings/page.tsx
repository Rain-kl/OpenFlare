'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Cloud, Save, ShieldCheck, Trash2 } from 'lucide-react';
import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ErrorInline } from '@/components/layout/error';
import {
  CloudflareService,
  cloudflareQueryKey,
  DnsAccountService,
  type CloudflareConnectionSource,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';

const dnsAccountsQueryKey = ['openflare', 'dns-accounts'] as const;

export default function CloudflareSettingsPage() {
  const queryClient = useQueryClient();
  const [source, setSource] =
    useState<CloudflareConnectionSource>('dns_account');
  const [dnsAccountID, setDNSAccountID] = useState('');
  const [apiToken, setAPIToken] = useState('');

  const connectionQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'connection'],
    queryFn: () => CloudflareService.getConnection(),
  });
  const accountsQuery = useQuery({
    queryKey: dnsAccountsQueryKey,
    queryFn: () => DnsAccountService.list(),
  });

  const cloudflareAccounts = useMemo(
    () =>
      (accountsQuery.data ?? []).filter(
        (account) => account.type === 'cloudflare',
      ),
    [accountsQuery.data],
  );

  useEffect(() => {
    const connection = connectionQuery.data;
    if (!connection?.configured) return;
    if (connection.source) setSource(connection.source);
    if (connection.dns_account_id)
      setDNSAccountID(String(connection.dns_account_id));
  }, [connectionQuery.data]);

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: cloudflareQueryKey });
  };

  const saveMutation = useMutation({
    mutationFn: () =>
      CloudflareService.saveConnection({
        source,
        dns_account_id: source === 'dns_account' ? Number(dnsAccountID) : 0,
        api_token: source === 'standalone' ? apiToken : '',
      }),
    onSuccess: async () => {
      toast.success('Cloudflare 连接配置已保存');
      setAPIToken('');
      await refresh();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const verifyMutation = useMutation({
    mutationFn: () => CloudflareService.verifyConnection(),
    onSuccess: async () => {
      toast.success('Cloudflare 连接验证成功');
      await refresh();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const clearMutation = useMutation({
    mutationFn: () => CloudflareService.clearConnection(),
    onSuccess: async () => {
      toast.success('Cloudflare 连接已清除');
      setDNSAccountID('');
      setAPIToken('');
      await refresh();
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Cloud className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            Cloudflare 连接设置
          </h1>
        </div>
        <Button asChild variant='outline' size='sm'>
          <Link href='/cloudflare'>返回总览</Link>
        </Button>
      </div>

      {connectionQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(connectionQuery.error)}
          onRetry={() => void connectionQuery.refetch()}
        />
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>凭据来源</CardTitle>
          <CardDescription>
            Token 建议授予 Zone:Read 与 DNS:Edit 权限；Token 不会在 API
            或页面中回显。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor='cf-source'>连接来源</FieldLabel>
              <Select
                value={source}
                onValueChange={(value) =>
                  setSource(value as CloudflareConnectionSource)
                }
              >
                <SelectTrigger id='cf-source' className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='dns_account'>
                      导入现有 DNS 账号
                    </SelectItem>
                    <SelectItem value='standalone'>独立 API Token</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>

            {source === 'dns_account' ? (
              <Field>
                <FieldLabel htmlFor='cf-dns-account'>
                  Cloudflare DNS 账号
                </FieldLabel>
                <Select value={dnsAccountID} onValueChange={setDNSAccountID}>
                  <SelectTrigger id='cf-dns-account' className='w-full'>
                    <SelectValue placeholder='选择 DNS 账号' />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {cloudflareAccounts.map((account) => (
                        <SelectItem key={account.id} value={String(account.id)}>
                          {account.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FieldDescription>
                  仅显示类型为 cloudflare 的 DNS 账号。可前往{' '}
                  <Link href='/dns-accounts'>DNS 账号</Link> 新增。
                </FieldDescription>
              </Field>
            ) : (
              <Field>
                <FieldLabel htmlFor='cf-api-token'>API Token</FieldLabel>
                <Input
                  id='cf-api-token'
                  type='password'
                  autoComplete='new-password'
                  value={apiToken}
                  onChange={(event) => setAPIToken(event.target.value)}
                  placeholder={
                    connectionQuery.data?.configured
                      ? '留空不会回显现有 Token；保存将替换'
                      : '输入 Cloudflare API Token'
                  }
                />
              </Field>
            )}

            <div className='flex flex-wrap items-center gap-2'>
              <Button
                onClick={() => saveMutation.mutate()}
                disabled={
                  saveMutation.isPending ||
                  (source === 'dns_account' ? !dnsAccountID : !apiToken.trim())
                }
              >
                <Save data-icon='inline-start' />
                保存配置
              </Button>
              <Button
                variant='outline'
                onClick={() => verifyMutation.mutate()}
                disabled={
                  !connectionQuery.data?.configured || verifyMutation.isPending
                }
              >
                <ShieldCheck data-icon='inline-start' />
                测试连接
              </Button>
              <Button
                variant='destructive'
                onClick={() => clearMutation.mutate()}
                disabled={
                  !connectionQuery.data?.configured || clearMutation.isPending
                }
              >
                <Trash2 data-icon='inline-start' />
                清除连接
              </Button>
            </div>
          </FieldGroup>
        </CardContent>
      </Card>

      <Alert>
        <ShieldCheck />
        <AlertTitle>当前状态</AlertTitle>
        <AlertDescription>
          {connectionQuery.data?.ready
            ? 'Token 已验证，Cloudflare 指向同步可以执行。'
            : '保存配置后仍需测试连接；未验证状态下同步任务会被拒绝。'}
        </AlertDescription>
      </Alert>
    </div>
  );
}
