'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { KeyRound } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useState } from 'react';
import { toast } from 'sonner';

import { ErrorInline } from '@/components/layout/error';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import { SmartDNSService, smartdnsQueryKey } from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';

import { SmartDNSDeploymentCommands } from './deployment-commands';

const tokenKey = [...smartdnsQueryKey, 'discovery-token'] as const;

export function SmartDNSDiscoveryEnrollment() {
  const t = useTranslations('smartdns.instances');
  const common = useTranslations('common');
  const queryClient = useQueryClient();
  const [confirmRotate, setConfirmRotate] = useState(false);
  const tokenQuery = useQuery({
    queryKey: tokenKey,
    queryFn: () => SmartDNSService.getDiscoveryToken(),
  });
  const rotateMutation = useMutation({
    mutationFn: () => SmartDNSService.rotateDiscoveryToken(),
    onSuccess: (data) => {
      queryClient.setQueryData(tokenKey, data);
      setConfirmRotate(false);
      toast.success(t('discoveryRotated'));
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });
  const token = tokenQuery.data?.discovery_token;
  return (
    <>
      <div className='flex flex-col gap-4'>
        <p className='text-sm text-muted-foreground'>{t('discoveryDesc')}</p>
        {tokenQuery.isPending ? (
          <p
            role='status'
            className='flex items-center gap-2 text-sm text-muted-foreground'
          >
            <Spinner />
            {t('loadingToken')}
          </p>
        ) : tokenQuery.isError ? (
          <ErrorInline
            message={getErrorMessage(tokenQuery.error)}
            onRetry={() => void tokenQuery.refetch()}
          />
        ) : (
          token && (
            <>
              <SmartDNSDeploymentCommands
                key={token}
                tokenLabel={t('discoveryTokenLabel')}
                token={token}
                tokenType='discovery'
              />
              <Button
                className='self-start'
                variant='ghost'
                size='sm'
                onClick={() => setConfirmRotate(true)}
              >
                <KeyRound data-icon='inline-start' />
                {t('rotateDiscovery')}
              </Button>
            </>
          )
        )}
      </div>
      <AlertDialog
        open={confirmRotate}
        onOpenChange={(next) =>
          !rotateMutation.isPending && setConfirmRotate(next)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('rotateDiscovery')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('rotateDiscoveryDescription')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={rotateMutation.isPending}>
              {common('cancel')}
            </AlertDialogCancel>
            <Button
              variant='destructive'
              disabled={rotateMutation.isPending}
              onClick={() => rotateMutation.mutate()}
            >
              {rotateMutation.isPending && <Spinner />}
              {t('rotateDiscovery')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
