'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldTitle,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  type PagesRemoteNetworkPolicy,
  type PagesSource,
  PagesService,
} from '@/lib/services/openflare';

import {
  deploymentsQueryKey,
  projectQueryKey,
  projectsQueryKey,
  sourceQueryKey,
} from '../../components/pages-utils';

type SourceMode = 'manual' | 'remote_url';
type Confirmation = 'trusted_internal' | 'manual' | null;

interface PagesSourceDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: number;
  source: PagesSource;
  initialMode?: SourceMode;
}

export function PagesSourceDialog({
  open,
  onOpenChange,
  projectId,
  source,
  initialMode,
}: PagesSourceDialogProps) {
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<SourceMode>('manual');
  const [networkPolicy, setNetworkPolicy] =
    useState<PagesRemoteNetworkPolicy>('public');
  const [replaceURL, setReplaceURL] = useState(false);
  const [remoteURL, setRemoteURL] = useState('');
  const [urlError, setURLError] = useState('');
  const [confirmation, setConfirmation] = useState<Confirmation>(null);

  useEffect(() => {
    if (!open) return;
    const nextMode =
      initialMode ??
      (source.source_type === 'remote_url' ? 'remote_url' : 'manual');
    setMode(nextMode);
    setNetworkPolicy(
      source.source_type === 'remote_url'
        ? source.remote_network_policy
        : 'public',
    );
    setReplaceURL(source.source_type !== 'remote_url');
    setRemoteURL('');
    setURLError('');
    setConfirmation(null);
  }, [initialMode, open, source]);

  const invalidateSourceState = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: sourceQueryKey(projectId) }),
      queryClient.invalidateQueries({ queryKey: projectQueryKey(projectId) }),
      queryClient.invalidateQueries({
        queryKey: deploymentsQueryKey(projectId),
      }),
      queryClient.invalidateQueries({ queryKey: projectsQueryKey }),
    ]);
  };

  const updateMutation = useMutation({
    mutationFn: () =>
      PagesService.updateSource(projectId, {
        source_type: 'remote_url',
        remote_url_set: replaceURL,
        remote_url: replaceURL ? remoteURL.trim() : '',
        remote_network_policy: networkPolicy,
      }),
    onSuccess: async (result) => {
      queryClient.setQueryData(sourceQueryKey(projectId), result.source);
      await invalidateSourceState();
      toast.success('部署源已更新');
      if (result.warning) toast.warning(result.warning);
      setConfirmation(null);
      onOpenChange(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '部署源更新失败');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => PagesService.deleteSource(projectId),
    onSuccess: async (manualSource) => {
      queryClient.setQueryData(sourceQueryKey(projectId), manualSource);
      await invalidateSourceState();
      toast.success('已切换回手动部署');
      setConfirmation(null);
      onOpenChange(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '切换失败');
    },
  });

  const isPending = updateMutation.isPending || deleteMutation.isPending;

  const submitRemote = () => {
    if (replaceURL) {
      const value = remoteURL.trim();
      if (!value) {
        setURLError('请输入 Remote URL');
        return;
      }
      try {
        const parsed = new URL(value);
        if (!['http:', 'https:'].includes(parsed.protocol)) throw new Error();
      } catch {
        setURLError('请输入有效的 HTTP(S) URL');
        return;
      }
    }
    setURLError('');
    if (networkPolicy === 'trusted_internal') {
      setConfirmation('trusted_internal');
      return;
    }
    updateMutation.mutate();
  };

  const handleSubmit = () => {
    if (mode === 'manual') {
      if (source.source_type === 'manual') {
        onOpenChange(false);
      } else {
        setConfirmation('manual');
      }
      return;
    }
    submitRemote();
  };

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          if (!isPending) onOpenChange(nextOpen);
        }}
      >
        <DialogContent className='sm:max-w-xl'>
          <DialogHeader>
            <DialogTitle>部署源设置</DialogTitle>
            <DialogDescription>
              手动部署与 Remote URL
              各自保持独立配置；后续仓库构建来源会作为新的来源类型接入。
            </DialogDescription>
          </DialogHeader>

          <FieldGroup>
            <Field>
              <FieldTitle id='pages-source-mode'>来源类型</FieldTitle>
              <ToggleGroup
                type='single'
                variant='outline'
                value={mode}
                aria-labelledby='pages-source-mode'
                className='grid w-full grid-cols-2'
                onValueChange={(value) => {
                  if (value === 'manual' || value === 'remote_url') {
                    setMode(value);
                  }
                }}
              >
                <ToggleGroupItem value='manual' className='w-full'>
                  手动部署
                </ToggleGroupItem>
                <ToggleGroupItem value='remote_url' className='w-full'>
                  Remote URL
                </ToggleGroupItem>
              </ToggleGroup>
              <FieldDescription>
                手动部署由管理员上传本地包；Remote URL 通过显式同步下载并发布。
              </FieldDescription>
            </Field>

            {mode === 'manual' ? (
              <Field>
                <FieldLabel>手动部署</FieldLabel>
                <div className='rounded-lg border bg-muted/20 p-4 text-sm text-muted-foreground'>
                  保留现有部署与当前生产版本，后续通过“上传部署包”创建新部署。
                </div>
              </Field>
            ) : (
              <>
                <Field data-invalid={Boolean(urlError)}>
                  <FieldLabel htmlFor='pages-remote-url'>Remote URL</FieldLabel>
                  {source.source_type === 'remote_url' && !replaceURL ? (
                    <div className='flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between'>
                      <code className='min-w-0 truncate text-xs'>
                        {source.display_url}
                      </code>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => {
                          setReplaceURL(true);
                          setRemoteURL('');
                        }}
                      >
                        更换地址
                      </Button>
                    </div>
                  ) : (
                    <Input
                      id='pages-remote-url'
                      type='url'
                      placeholder='https://artifacts.example.com/site.zip?token=...'
                      value={remoteURL}
                      aria-invalid={Boolean(urlError)}
                      autoComplete='off'
                      onChange={(event) => {
                        setRemoteURL(event.target.value);
                        setURLError('');
                      }}
                    />
                  )}
                  <FieldDescription>
                    {urlError ||
                      (replaceURL
                        ? '保存后不会回显原始地址或 query token。'
                        : '界面只显示脱敏地址；留空表示保留当前地址。')}
                  </FieldDescription>
                  {source.source_type === 'remote_url' && replaceURL ? (
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      className='self-start'
                      onClick={() => {
                        setReplaceURL(false);
                        setRemoteURL('');
                        setURLError('');
                      }}
                    >
                      保留当前地址
                    </Button>
                  ) : null}
                </Field>

                <Field>
                  <FieldTitle id='pages-network-policy'>网络策略</FieldTitle>
                  <ToggleGroup
                    type='single'
                    variant='outline'
                    value={networkPolicy}
                    aria-labelledby='pages-network-policy'
                    className='grid w-full grid-cols-2'
                    onValueChange={(value) => {
                      if (value === 'public' || value === 'trusted_internal') {
                        setNetworkPolicy(value);
                      }
                    }}
                  >
                    <ToggleGroupItem value='public' className='w-full'>
                      公网安全模式
                    </ToggleGroupItem>
                    <ToggleGroupItem
                      value='trusted_internal'
                      className='w-full'
                    >
                      受信内网模式
                    </ToggleGroupItem>
                  </ToggleGroup>
                  <FieldDescription>
                    {networkPolicy === 'public'
                      ? '阻止内网地址、代理与不安全 TLS。'
                      : '允许访问内网地址与自签名证书，仅用于可信来源。'}
                  </FieldDescription>
                </Field>
              </>
            )}
          </FieldGroup>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={isPending}
              onClick={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button type='button' disabled={isPending} onClick={handleSubmit}>
              {isPending ? <Spinner data-icon='inline-start' /> : null}
              {mode === 'manual' ? '使用手动部署' : '保存 Remote 来源'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={confirmation !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen && !isPending) setConfirmation(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirmation === 'manual'
                ? '切换回手动部署'
                : '启用受信内网模式'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirmation === 'manual'
                ? '当前来源配置将被删除，但已有部署与当前生产版本会保留。'
                : '该模式允许访问私有网络并接受自签名证书。请确认此地址属于可信内部来源，且不会被非可信用户控制。'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isPending}>取消</AlertDialogCancel>
            <AlertDialogAction
              disabled={isPending}
              onClick={(event) => {
                event.preventDefault();
                if (confirmation === 'manual') {
                  deleteMutation.mutate();
                } else if (confirmation === 'trusted_internal') {
                  updateMutation.mutate();
                }
              }}
            >
              {isPending ? <Spinner data-icon='inline-start' /> : null}
              确认
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
