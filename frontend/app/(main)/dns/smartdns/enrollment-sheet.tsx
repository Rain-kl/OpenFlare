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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Spinner } from '@/components/ui/spinner';
import { SmartDNSService, smartdnsQueryKey } from '@/lib/services/openflare';
import { getErrorMessage } from '../../websites/components/website-utils';

import { SmartDNSDeploymentCommands } from './deployment-commands';

const tokenKey = [...smartdnsQueryKey, 'discovery-token'] as const;

export function SmartDNSEnrollmentSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations('smartdns.instances');
  const common = useTranslations('common');
  const queryClient = useQueryClient();
  const [confirmRotate, setConfirmRotate] = useState(false);
  const tokenQuery = useQuery({
    queryKey: tokenKey,
    queryFn: () => SmartDNSService.getDiscoveryToken(),
    enabled: open,
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
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className='w-full overflow-y-auto sm:max-w-xl'>
          <SheetHeader>
            <SheetTitle>{t('discoveryTitle')}</SheetTitle>
            <SheetDescription>{t('discoveryDesc')}</SheetDescription>
          </SheetHeader>
          <div className='flex flex-col gap-6 px-4 pb-6'>
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
        </SheetContent>
      </Sheet>
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
