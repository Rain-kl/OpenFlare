'use client';

import { useEffect, useRef, useState } from 'react';
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
import { Switch } from '@/components/ui/switch';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
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

type Confirmation = 'manual' | null;

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
  checkInterval: '',
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
  const [allowInsecure, setAllowInsecure] = useState(false);
  const [remoteURL, setRemoteURL] = useState('');
  const [urlError, setURLError] = useState('');
  const [githubForm, setGitHubForm] = useState<PagesGitHubSourceFormValue>({
    repositoryURL: '',
    releaseSelector: 'latest',
    releaseTag: '',
    assetName: DEFAULT_GITHUB_ASSET,
    autoUpdateEnabled: false,
    checkIntervalMinutes: String(DEFAULT_GITHUB_CHECK_INTERVAL),
  });
  const [githubErrors, setGitHubErrors] =
    useState<PagesGitHubSourceFormErrors>(EMPTY_GITHUB_ERRORS);
  const [confirmation, setConfirmation] = useState<Confirmation>(null);
  const initializedForOpen = useRef(false);

  useEffect(() => {
    if (!open) {
      initializedForOpen.current = false;
      return;
    }
    // Runtime polling may replace the source view while the dialog is open.
    // Initialize only on the open edge so it cannot overwrite an unsaved draft.
    if (initializedForOpen.current) return;
    initializedForOpen.current = true;
    const nextMode = initialMode ?? source.source_type;
    setMode(nextMode);
    setAllowInsecure(
      source.source_type === 'remote_url' && Boolean(source.allow_insecure),
    );
    setRemoteURL(
      source.source_type === 'remote_url' ? (source.remote_url ?? '') : '',
    );
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
      autoUpdateEnabled:
        source.source_type === 'github_release' &&
        source.release_selector === 'latest'
          ? source.auto_update_enabled
          : false,
      checkIntervalMinutes:
        source.source_type === 'github_release' &&
        source.release_selector === 'latest'
          ? String(
              source.check_interval_minutes || DEFAULT_GITHUB_CHECK_INTERVAL,
            )
          : String(DEFAULT_GITHUB_CHECK_INTERVAL),
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

  const remotePayload = (): PagesSourceUpdatePayload => ({
    source_type: 'remote_url',
    remote_url: remoteURL.trim(),
    allow_insecure: allowInsecure,
  });

  const submitRemote = () => {
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
    setURLError('');
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
    const checkIntervalMinutes = Number(githubForm.checkIntervalMinutes);
    const nextCheckIntervalError =
      githubForm.releaseSelector === 'latest' &&
      (!Number.isInteger(checkIntervalMinutes) ||
        checkIntervalMinutes < 5 ||
        checkIntervalMinutes > 1440)
        ? '检查间隔须为 5–1440 分钟的整数'
        : '';

    setGitHubErrors({
      repository: nextRepositoryError,
      releaseTag: nextReleaseTagError,
      assetName: nextAssetNameError,
      checkInterval: nextCheckIntervalError,
    });
    if (
      nextRepositoryError ||
      nextReleaseTagError ||
      nextAssetNameError ||
      nextCheckIntervalError
    ) {
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
            auto_update_enabled: githubForm.autoUpdateEnabled,
            check_interval_minutes: checkIntervalMinutes,
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
              选择部署来源。
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
                      setURLError('');
                    } else if (source.source_type === 'remote_url') {
                      setRemoteURL(source.remote_url ?? '');
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
                  <Input
                    id='pages-remote-url'
                    type='url'
                    placeholder='https://artifacts.example.com/site.zip'
                    value={remoteURL}
                    aria-invalid={Boolean(urlError)}
                    autoComplete='off'
                    onChange={(event) => {
                      setRemoteURL(event.target.value);
                      setURLError('');
                    }}
                  />
                  <FieldDescription>
                    {urlError || '填写可直接下载的部署包 HTTP(S) 地址。'}
                  </FieldDescription>
                </Field>

                <div className='flex items-center justify-between rounded-lg border border-dashed px-4 py-3'>
                  <div className='space-y-1 pr-4'>
                    <p className='text-sm font-medium'>允许不安全的连接</p>
                    <p className='text-xs text-muted-foreground'>
                      默认允许公网与内网地址；开启后跳过 TLS
                      证书校验，适用于自签名或私有 CA。
                    </p>
                  </div>
                  <Switch
                    checked={allowInsecure}
                    onCheckedChange={setAllowInsecure}
                    aria-label='允许不安全的连接'
                  />
                </div>
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
              onClick={() => onOpenChange(false)}
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
            <AlertDialogTitle>切换回手动部署</AlertDialogTitle>
            <AlertDialogDescription>
              当前来源配置将被删除，但已有部署与当前生产版本会保留。
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
