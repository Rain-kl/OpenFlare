'use client';

import { useEffect, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
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
const DEFAULT_GITHUB_CHECK_INTERVAL = 1440;
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
  const t = useTranslations('pages.source');
  const tCommon = useTranslations('common');
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
      toast.success(t('updated'));
      if (result.warning) toast.warning(result.warning);
      setConfirmation(null);
      onOpenChange(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('updateFailed'));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => PagesService.deleteSource(projectId),
    onSuccess: async (manualSource) => {
      queryClient.setQueryData(sourceQueryKey(projectId), manualSource);
      await invalidateSourceState();
      toast.success(t('switchedManual'));
      setConfirmation(null);
      onOpenChange(false);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('switchFailed'));
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
      setURLError(t('urlRequired'));
      return;
    }
    try {
      const parsed = new URL(value);
      if (!['http:', 'https:'].includes(parsed.protocol)) throw new Error();
    } catch {
      setURLError(t('urlInvalid'));
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
      : t('githubUrlInvalid');
    const nextReleaseTagError =
      githubForm.releaseSelector === 'tag' &&
      !validGitHubReleaseTag(githubForm.releaseTag)
        ? t('tagInvalid')
        : '';
    const nextAssetNameError = validGitHubAssetName(githubForm.assetName)
      ? ''
      : t('assetInvalid');
    const checkIntervalMinutes = Number(githubForm.checkIntervalMinutes);
    const nextCheckIntervalError =
      githubForm.releaseSelector === 'latest' &&
      (!Number.isInteger(checkIntervalMinutes) ||
        checkIntervalMinutes < 5 ||
        checkIntervalMinutes > 1440)
        ? t('intervalInvalid')
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
      ? t('useManual')
      : mode === 'remote_url'
        ? t('saveRemote')
        : t('saveGithub');

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
            <DialogTitle>{t('settingsTitle')}</DialogTitle>
            <DialogDescription>{t('settingsDesc')}</DialogDescription>
          </DialogHeader>

          <FieldGroup>
            <Field>
              <FieldTitle id='pages-source-mode'>{t('mode')}</FieldTitle>
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
                  {t('manual')}
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
                <FieldLabel>{t('manual')}</FieldLabel>
                <div className='rounded-lg border bg-muted/20 p-4 text-sm text-muted-foreground'>
                  {t('manualHint')}
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
                    {urlError || t('urlHint')}
                  </FieldDescription>
                </Field>

                <div className='flex items-center justify-between rounded-lg border border-dashed px-4 py-3'>
                  <div className='space-y-1 pr-4'>
                    <p className='text-sm font-medium'>{t('allowInsecure')}</p>
                    <p className='text-xs text-muted-foreground'>
                      {t('allowInsecureDesc')}
                    </p>
                  </div>
                  <Switch
                    checked={allowInsecure}
                    onCheckedChange={setAllowInsecure}
                    aria-label={t('allowInsecure')}
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
              {tCommon('cancel')}
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
            <AlertDialogTitle>{t('switchManualTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('switchManualDesc')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isPending}>
              {tCommon('cancel')}
            </AlertDialogCancel>
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
              {tCommon('confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
