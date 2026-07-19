'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Eye, EyeOff } from 'lucide-react';
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
  type PagesSourceActionReceipt,
  type PagesSourceUpdatePayload,
  PagesService,
} from '@/lib/services/openflare';

import {
  deploymentsQueryKey,
  projectQueryKey,
  projectsQueryKey,
  sourceQueryKey,
} from '../../components/pages-utils';
import {
  type PagesGitHubSourceFormErrors,
  type PagesGitHubSourceFormValue,
  PagesSourceGitHubFields,
} from './pages-source-github-fields';
import {
  validGitHubAssetName,
  validGitHubReleaseTag,
  validGitHubRepositoryURL,
} from './pages-source-validation';

export type PagesSourceMode = 'manual' | 'remote_url' | 'github_release';

type Confirmation = 'trusted_internal' | 'manual' | null;

interface PagesSourceDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: number;
  source: PagesSource;
  initialMode?: PagesSourceMode;
  onActionDispatched?: (receipt: PagesSourceActionReceipt) => void;
}

const DEFAULT_GITHUB_ASSET = 'dist.zip';
const DEFAULT_GITHUB_CHECK_INTERVAL = 60;
const EMPTY_GITHUB_ERRORS: PagesGitHubSourceFormErrors = {
  repository: '',
  releaseTag: '',
  assetName: '',
};

function githubRepositoryURL(repository: string) {
  const value = repository.trim();
  return value ? `https://github.com/${value}` : '';
}

export function PagesSourceDialog({
  open,
  onOpenChange,
  projectId,
  source,
  initialMode,
  onActionDispatched,
}: PagesSourceDialogProps) {
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<PagesSourceMode>('manual');
  const [networkPolicy, setNetworkPolicy] =
    useState<PagesRemoteNetworkPolicy>('public');
  const [replaceURL, setReplaceURL] = useState(false);
  const [remoteURL, setRemoteURL] = useState('');
  const [revealRemoteURL, setRevealRemoteURL] = useState(false);
  const [urlError, setURLError] = useState('');
  const [githubForm, setGitHubForm] = useState<PagesGitHubSourceFormValue>({
    repositoryURL: '',
    releaseSelector: 'latest',
    releaseTag: '',
    assetName: DEFAULT_GITHUB_ASSET,
  });
  const [githubErrors, setGitHubErrors] =
    useState<PagesGitHubSourceFormErrors>(EMPTY_GITHUB_ERRORS);
  const [confirmation, setConfirmation] = useState<Confirmation>(null);

  useEffect(() => {
    if (!open) {
      setRemoteURL('');
      setRevealRemoteURL(false);
      return;
    }
    const nextMode = initialMode ?? source.source_type;
    setMode(nextMode);
    setNetworkPolicy(
      source.source_type === 'remote_url'
        ? source.remote_network_policy
        : 'public',
    );
    setReplaceURL(source.source_type !== 'remote_url');
    setRemoteURL('');
    setRevealRemoteURL(false);
    setURLError('');
    setGitHubForm({
      repositoryURL:
        source.source_type === 'github_release'
          ? githubRepositoryURL(source.github_repository)
          : '',
      releaseSelector:
        source.source_type === 'github_release'
          ? source.release_selector
          : 'latest',
      releaseTag:
        source.source_type === 'github_release'
          ? (source.release_tag ?? '')
          : '',
      assetName:
        source.source_type === 'github_release'
          ? source.asset_name
          : DEFAULT_GITHUB_ASSET,
    });
    setGitHubErrors(EMPTY_GITHUB_ERRORS);
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
    mutationFn: (payload: PagesSourceUpdatePayload) =>
      PagesService.updateSource(projectId, payload),
    onSuccess: async (result) => {
      queryClient.setQueryData(sourceQueryKey(projectId), result.source);
      if (result.check_task) onActionDispatched?.(result.check_task);
      await invalidateSourceState();
      toast.success('部署源已更新');
      if (result.warning) toast.warning(result.warning);
      setConfirmation(null);
      setRemoteURL('');
      setRevealRemoteURL(false);
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
      setRemoteURL('');
      setRevealRemoteURL(false);
      onOpenChange(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '切换失败');
    },
  });

  const isPending = updateMutation.isPending || deleteMutation.isPending;

  const remotePayload = (): PagesSourceUpdatePayload => ({
    source_type: 'remote_url',
    remote_url_set: replaceURL,
    remote_url: replaceURL ? remoteURL.trim() : '',
    remote_network_policy: networkPolicy,
  });

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
    updateMutation.mutate(remotePayload());
  };

  const submitGitHub = () => {
    const normalizedRepositoryURL = githubForm.repositoryURL.trim();
    const nextRepositoryError = validGitHubRepositoryURL(
      normalizedRepositoryURL,
    )
      ? ''
      : '请输入 https://github.com/{owner}/{repo} 格式的公开仓库地址';
    const nextReleaseTagError =
      githubForm.releaseSelector === 'tag' &&
      !validGitHubReleaseTag(githubForm.releaseTag)
        ? 'Release tag 须为有效 Git ref（1–255 字节，可使用 /、#、&、=）'
        : '';
    const nextAssetNameError = validGitHubAssetName(githubForm.assetName)
      ? ''
      : 'Asset 文件名须为 1–255 字节，且不能是路径或包含控制、换行、双向文本字符';

    setGitHubErrors({
      repository: nextRepositoryError,
      releaseTag: nextReleaseTagError,
      assetName: nextAssetNameError,
    });
    if (nextRepositoryError || nextReleaseTagError || nextAssetNameError) {
      return;
    }

    const payload: PagesSourceUpdatePayload =
      githubForm.releaseSelector === 'latest'
        ? {
            source_type: 'github_release',
            repository_url: normalizedRepositoryURL,
            release_selector: 'latest',
            release_tag: '',
            asset_name: githubForm.assetName,
            auto_update_enabled: false,
            check_interval_minutes:
              source.source_type === 'github_release' &&
              source.release_selector === 'latest' &&
              source.check_interval_minutes
                ? source.check_interval_minutes
                : DEFAULT_GITHUB_CHECK_INTERVAL,
          }
        : {
            source_type: 'github_release',
            repository_url: normalizedRepositoryURL,
            release_selector: 'tag',
            release_tag: githubForm.releaseTag,
            asset_name: githubForm.assetName,
            auto_update_enabled: false,
            check_interval_minutes: 0,
          };
    updateMutation.mutate(payload);
  };

  const handleSubmit = () => {
    switch (mode) {
      case 'manual':
        if (source.source_type === 'manual') {
          onOpenChange(false);
        } else {
          setConfirmation('manual');
        }
        return;
      case 'remote_url':
        submitRemote();
        return;
      case 'github_release':
        submitGitHub();
    }
  };

  const submitLabel =
    mode === 'manual'
      ? '使用手动部署'
      : mode === 'remote_url'
        ? '保存 Remote 来源'
        : '保存 GitHub 来源';

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
              手动部署、Remote URL 与 GitHub Release 使用独立配置。
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
                className='grid w-full grid-cols-1 sm:grid-cols-3'
                onValueChange={(value) => {
                  if (
                    value === 'manual' ||
                    value === 'remote_url' ||
                    value === 'github_release'
                  ) {
                    setMode(value);
                    if (value !== 'remote_url') {
                      setRemoteURL('');
                      setRevealRemoteURL(false);
                      setURLError('');
                    }
                  }
                }}
              >
                <ToggleGroupItem value='manual' className='w-full'>
                  手动部署
                </ToggleGroupItem>
                <ToggleGroupItem value='remote_url' className='w-full'>
                  Remote URL
                </ToggleGroupItem>
                <ToggleGroupItem value='github_release' className='w-full'>
                  GitHub Release
                </ToggleGroupItem>
              </ToggleGroup>
              <FieldDescription>
                <span>
                  远端来源只负责发现内容，发布结果始终保留为不可变部署。
                </span>
                <span className='block'>
                  仓库源码构建将在后续作为独立来源类型提供。
                </span>
              </FieldDescription>
            </Field>

            {mode === 'manual' ? (
              <Field>
                <FieldLabel>手动部署</FieldLabel>
                <div className='rounded-lg border bg-muted/20 p-4 text-sm text-muted-foreground'>
                  保留现有部署与当前生产版本，后续通过“上传部署包”创建新部署。
                </div>
              </Field>
            ) : mode === 'remote_url' ? (
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
                          setRevealRemoteURL(false);
                        }}
                      >
                        更换地址
                      </Button>
                    </div>
                  ) : (
                    <div className='flex gap-2'>
                      <Input
                        id='pages-remote-url'
                        type={revealRemoteURL ? 'url' : 'password'}
                        placeholder='https://artifacts.example.com/site.zip?token=...'
                        value={remoteURL}
                        aria-invalid={Boolean(urlError)}
                        autoComplete='off'
                        className='min-w-0 flex-1'
                        onChange={(event) => {
                          setRemoteURL(event.target.value);
                          setURLError('');
                        }}
                      />
                      <Button
                        type='button'
                        variant='outline'
                        size='icon'
                        aria-label={
                          revealRemoteURL
                            ? '隐藏 Remote URL'
                            : '显示 Remote URL'
                        }
                        title={
                          revealRemoteURL
                            ? '隐藏 Remote URL'
                            : '显示 Remote URL'
                        }
                        onClick={() =>
                          setRevealRemoteURL((visible) => !visible)
                        }
                      >
                        {revealRemoteURL ? <EyeOff /> : <Eye />}
                      </Button>
                    </div>
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
                        setRevealRemoteURL(false);
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
            ) : (
              <PagesSourceGitHubFields
                value={githubForm}
                errors={githubErrors}
                defaultAssetName={DEFAULT_GITHUB_ASSET}
                onChange={setGitHubForm}
                onErrorsChange={setGitHubErrors}
              />
            )}
          </FieldGroup>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={isPending}
              onClick={() => {
                setRemoteURL('');
                setRevealRemoteURL(false);
                onOpenChange(false);
              }}
            >
              取消
            </Button>
            <Button type='button' disabled={isPending} onClick={handleSubmit}>
              {isPending ? <Spinner data-icon='inline-start' /> : null}
              {submitLabel}
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
                  updateMutation.mutate(remotePayload());
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
