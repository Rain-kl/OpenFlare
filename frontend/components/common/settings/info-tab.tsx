'use client';

import { useMutation, useQuery } from '@tanstack/react-query';
import { ExternalLink, RefreshCw, Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { useTranslations } from 'next-intl';

import services from '@/lib/services';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Spinner } from '@/components/ui/spinner';

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='flex items-center justify-between gap-4 border-b border-dashed py-2 last:border-b-0'>
      <span className='text-xs text-muted-foreground'>{label}</span>
      <span className='text-right text-xs font-medium text-foreground break-all'>
        {value || '-'}
      </span>
    </div>
  );
}

export function InfoTab() {
  const t = useTranslations('settings.info');
  const updateQuery = useQuery({
    queryKey: ['admin', 'update'],
    queryFn: () => services.adminStatus.getUpdateStatus(),
    refetchInterval: 30 * 60 * 1000,
    staleTime: 5 * 60 * 1000,
  });

  const applyUpdateMutation = useMutation({
    mutationFn: () => services.adminStatus.applyUpdate(),
    onSuccess: () => {
      toast.success(t('upgradeVerified'));
    },
    onError: (error: Error) => {
      toast.error(error.message || t('applyUpgradeFailed'));
    },
  });

  const update = updateQuery.data;

  return (
    <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
      <Card className='border border-dashed shadow-sm'>
        <CardHeader className='border-b border-dashed pb-4'>
          <div className='flex items-start justify-between gap-4'>
            <div className='flex items-center gap-2'>
              <div className='rounded-lg bg-muted p-1.5 text-muted-foreground'>
                <Sparkles className='size-4' />
              </div>
              <div>
                <CardTitle className='text-base font-semibold'>
                  {t('appUpdate')}
                </CardTitle>
                <CardDescription className='text-xs'>
                  {t('appUpdateDesc')}
                </CardDescription>
              </div>
            </div>
            {update?.update_available ? (
              <Badge>{t('newVersionAvailable')}</Badge>
            ) : update ? (
              <Badge variant='secondary'>{t('alreadyLatest')}</Badge>
            ) : null}
          </div>
        </CardHeader>
        <CardContent className='flex flex-col gap-4 pt-4'>
          {updateQuery.isLoading ? (
            <div className='flex min-h-32 items-center justify-center'>
              <Spinner />
            </div>
          ) : updateQuery.isError ? (
            <div className='flex min-h-32 flex-col items-center justify-center gap-3 text-center'>
              <p className='text-xs text-muted-foreground'>
                {updateQuery.error.message || t('cannotGetVersion')}
              </p>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => updateQuery.refetch()}
              >
                <RefreshCw data-icon='inline-start' />
                {t('recheck')}
              </Button>
            </div>
          ) : update ? (
            <>
              <div>
                <InfoRow
                  label={t('currentVersion')}
                  value={update.current_version}
                />
                <InfoRow
                  label={t('latestVersion')}
                  value={update.latest_version}
                />
                {update.build_time && (
                  <InfoRow label={t('buildTime')} value={update.build_time} />
                )}
                <InfoRow label={t('platform')} value={update.platform} />
                <InfoRow
                  label={t('upstreamRepo')}
                  value={update.upstream_repository}
                />
                <InfoRow label={t('releaseAsset')} value={update.asset_name} />
              </div>

              {update.release_notes && (
                <div className='flex flex-col gap-2 rounded-md border border-dashed p-3'>
                  <p className='text-xs font-medium'>{t('releaseNotes')}</p>
                  <p className='max-h-40 overflow-y-auto whitespace-pre-wrap text-xs leading-relaxed text-muted-foreground'>
                    {update.release_notes}
                  </p>
                </div>
              )}

              <div className='flex flex-wrap justify-end gap-2'>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={() => updateQuery.refetch()}
                  disabled={updateQuery.isFetching}
                >
                  {updateQuery.isFetching ? (
                    <Spinner data-icon='inline-start' />
                  ) : (
                    <RefreshCw data-icon='inline-start' />
                  )}
                  {t('checkUpdate')}
                </Button>
                {update.release_url && (
                  <Button asChild variant='outline' size='sm'>
                    <a
                      href={update.release_url}
                      target='_blank'
                      rel='noreferrer'
                    >
                      <ExternalLink data-icon='inline-start' />
                      {t('viewRelease')}
                    </a>
                  </Button>
                )}
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button
                      type='button'
                      size='sm'
                      disabled={
                        !update.can_upgrade || applyUpdateMutation.isPending
                      }
                    >
                      {applyUpdateMutation.isPending && (
                        <Spinner data-icon='inline-start' />
                      )}
                      {t('upgradeNow')}
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>
                        {t('upgradeTo', { version: update.latest_version })}
                      </AlertDialogTitle>
                      <AlertDialogDescription>
                        {t('upgradeDesc', { asset: update.asset_name })}
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>{t('cancel')}</AlertDialogCancel>
                      <AlertDialogAction
                        onClick={() => applyUpdateMutation.mutate()}
                      >
                        {t('confirmUpgrade')}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>

              {!update.can_upgrade && (
                <p className='text-xs text-muted-foreground'>
                  {update.current_version === 'dev'
                    ? t('devBuildNoUpgrade')
                    : update.update_available
                      ? t('platformNotSupported')
                      : t('noUpgradeNeeded')}
                </p>
              )}
            </>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
