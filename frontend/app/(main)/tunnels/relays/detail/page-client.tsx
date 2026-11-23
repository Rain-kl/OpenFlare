'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  Download,
  Pencil,
  RotateCcw,
  Server,
  ShieldAlert,
  ShieldCheck,
  Terminal,
  Trash2,
} from 'lucide-react';
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
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import type {
  TunnelRelayDeployView,
  TunnelRelayInput,
  TunnelRelayView,
} from '@/lib/services/openflare';
import { TunnelService, tunnelQueryKey } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { DeployCommandDialog } from '../../components/deploy-command-dialog';
import { RelayEditorDialog } from '../../components/relay-editor-dialog';
import { TunnelStatusBadge } from '../../components/tunnel-status-badge';
import {
  getErrorMessage,
  getTunnelStatusTone,
} from '../../components/tunnel-utils';

export function RelayDetailPageClient() {
  const t = useTranslations('tunnels');
  const tc = useTranslations('common');
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const relayId = Number(searchParams.get('id')?.trim() ?? '');
  const validId = Number.isSafeInteger(relayId) && relayId > 0;

  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deployDialog, setDeployDialog] = useState<{
    accessToken: string;
    bootstrapToken: string;
    serverUrl: string;
    deployCommand: string;
  } | null>(null);

  const relayQuery = useQuery({
    queryKey: [...tunnelQueryKey, 'relay-detail', relayId],
    queryFn: () => TunnelService.getRelay(relayId),
    enabled: validId,
    refetchInterval: (q) => (q.state.status === 'error' ? false : 15000),
    refetchIntervalInBackground: false,
  });

  const poolsQuery = useQuery({
    queryKey: [...tunnelQueryKey, 'pools'],
    queryFn: () => TunnelService.listPools(),
    enabled: validId,
  });

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: tunnelQueryKey });
  };

  const updateMutation = useMutation({
    mutationFn: (payload: TunnelRelayInput) =>
      TunnelService.updateRelay(relayId, payload),
    onSuccess: async () => {
      setEditOpen(false);
      toast.success(t('relays.updated'));
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const drainMutation = useMutation({
    mutationFn: (draining: boolean) =>
      TunnelService.setRelayDraining(relayId, { draining }),
    onSuccess: async () => {
      toast.success(t('relays.drainUpdated'));
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const rotateMutation = useMutation({
    mutationFn: () => TunnelService.rotateRelayToken(relayId),
    onSuccess: async (view) => {
      toast.success(t('detail.tokenRotated'));
      setDeployDialog({
        accessToken: view.access_token,
        bootstrapToken: view.bootstrap_token,
        serverUrl: view.server_url,
        deployCommand: view.deploy_command,
      });
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const quarantineMutation = useMutation<
    TunnelRelayDeployView | TunnelRelayView,
    Error,
    boolean
  >({
    mutationFn: (quarantined: boolean) =>
      quarantined
        ? TunnelService.unquarantineRelay(relayId)
        : TunnelService.quarantineRelay(relayId, ''),
    onSuccess: async (res) => {
      toast.success(t('detail.quarantineUpdated'));
      if ('deploy_command' in res) {
        setDeployDialog({
          accessToken: res.access_token,
          bootstrapToken: res.bootstrap_token,
          serverUrl: res.server_url,
          deployCommand: res.deploy_command,
        });
      }
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const updateBinaryMutation = useMutation({
    mutationFn: () =>
      TunnelService.requestRelayBinaryUpdate(relayId, { channel: 'stable' }),
    onSuccess: async () => {
      toast.success(t('detail.updateRequested'));
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const deleteMutation = useMutation({
    mutationFn: () => TunnelService.deleteRelay(relayId),
    onSuccess: async () => {
      toast.success(t('common.deleted'));
      await invalidate();
      router.push('/tunnels');
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  if (!validId) {
    return (
      <div className='py-6 px-1'>
        <EmptyStateWithBorder
          icon={Server}
          description={t('detail.missingId')}
        />
      </div>
    );
  }
  if (relayQuery.isLoading) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder
          icon={Server}
          description={t('detail.loading')}
        />
      </div>
    );
  }
  if (relayQuery.isError || !relayQuery.data) {
    return (
      <div className='py-6 px-1 space-y-4'>
        <Button variant='ghost' size='sm' className='h-8 px-2' asChild>
          <Link href='/tunnels'>
            <ArrowLeft className='size-4 mr-1' />
            {t('detail.back')}
          </Link>
        </Button>
        <div className='p-8 border border-dashed rounded-lg'>
          <ErrorInline
            message={getErrorMessage(relayQuery.error, t('detail.notFound'))}
            onRetry={() => void relayQuery.refetch()}
            className='justify-center'
          />
        </div>
      </div>
    );
  }

  const relay = relayQuery.data;
  const isQuarantined = Boolean(relay.quarantined_at);

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='space-y-1'>
          <Button variant='ghost' size='sm' className='h-7 px-2 -ml-2' asChild>
            <Link href='/tunnels'>
              <ArrowLeft className='size-3.5 mr-1' />
              {t('detail.back')}
            </Link>
          </Button>
          <div className='flex flex-wrap items-center gap-2'>
            <Server className='size-5 text-primary' />
            <h1 className='text-2xl font-semibold tracking-tight'>
              {relay.name}
            </h1>
            <TunnelStatusBadge
              label={t(`status.${relay.effective_status}`)}
              tone={getTunnelStatusTone(relay.effective_status)}
            />
          </div>
          <p className='text-xs font-mono text-muted-foreground'>
            {relay.relay_id} · {relay.pool_key}
          </p>
        </div>

        <div className='flex flex-wrap gap-2'>
          <Button
            variant='secondary'
            size='sm'
            className='h-7 text-xs'
            onClick={() =>
              void TunnelService.getRelayDeploy(relay.id).then((view) =>
                setDeployDialog({
                  accessToken: view.access_token,
                  bootstrapToken: view.bootstrap_token,
                  serverUrl: view.server_url,
                  deployCommand: view.deploy_command,
                }),
              )
            }
          >
            <Terminal className='size-3.5 mr-1' />
            {t('common.deploy')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            onClick={() => drainMutation.mutate(!relay.draining)}
          >
            {relay.draining ? t('relays.resume') : t('relays.drain')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            onClick={() => rotateMutation.mutate()}
          >
            <RotateCcw className='size-3.5 mr-1' />
            {t('detail.rotateToken')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            onClick={() => updateBinaryMutation.mutate()}
          >
            <Download className='size-3.5 mr-1' />
            {t('detail.updateBinary')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            onClick={() => quarantineMutation.mutate(isQuarantined)}
          >
            {isQuarantined ? (
              <ShieldCheck className='size-3.5 mr-1' />
            ) : (
              <ShieldAlert className='size-3.5 mr-1' />
            )}
            {isQuarantined ? t('detail.unquarantine') : t('detail.quarantine')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            onClick={() => setEditOpen(true)}
          >
            <Pencil className='size-3.5 mr-1' />
            {t('common.edit')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs text-destructive hover:text-destructive'
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className='size-3.5 mr-1' />
            {t('common.delete')}
          </Button>
        </div>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('relays.detailTitle')}
          </CardTitle>
        </CardHeader>
        <CardContent className='grid gap-4 text-sm sm:grid-cols-2'>
          <div>
            <p className='text-xs text-muted-foreground'>
              {t('relays.publicAddr')}
            </p>
            <p className='font-mono text-xs'>{relay.public_addr}</p>
          </div>
          <div>
            <p className='text-xs text-muted-foreground'>
              {t('relays.endpoint')}
            </p>
            <p className='font-mono text-xs'>
              T:{relay.transport_port} · WS:{relay.ws_port} · GW:
              {relay.gateway_port}
            </p>
          </div>
          <div>
            <p className='text-xs text-muted-foreground'>
              {t('relays.supportedTransports')}
            </p>
            <p className='text-xs'>
              {(relay.supported_transports ?? []).join(', ')}
            </p>
          </div>
          <div>
            <p className='text-xs text-muted-foreground'>
              {t('relays.zoneTag')}
            </p>
            <p className='text-xs'>
              {relay.zone_tag || '—'} · weight {relay.weight}
            </p>
          </div>
          <div>
            <p className='text-xs text-muted-foreground'>
              {t('relays.noisePublicKey')}
            </p>
            <p className='font-mono text-xs break-all'>
              {relay.noise_public_key || '—'}
            </p>
          </div>
          <div>
            <p className='text-xs text-muted-foreground'>
              {t('relays.lastSeen')}
            </p>
            <p className='text-xs'>
              {relay.last_seen_at
                ? formatDateTime(relay.last_seen_at)
                : t('common.never')}{' '}
              · {relay.version || 'dev'}
            </p>
          </div>
        </CardContent>
      </Card>

      <RelayEditorDialog
        open={editOpen}
        relay={relay}
        pools={poolsQuery.data ?? []}
        submitting={updateMutation.isPending}
        onClose={() => setEditOpen(false)}
        onSubmit={async (payload) => {
          await updateMutation.mutateAsync(payload);
        }}
      />

      {deployDialog ? (
        <DeployCommandDialog
          open={Boolean(deployDialog)}
          mode='relay'
          title={t('deploy.relayTitle', { name: relay.name })}
          accessToken={deployDialog.accessToken}
          bootstrapToken={deployDialog.bootstrapToken}
          serverUrl={deployDialog.serverUrl}
          deployCommand={deployDialog.deployCommand}
          onClose={() => setDeployDialog(null)}
        />
      ) : null}

      <AlertDialog
        open={deleteOpen}
        onOpenChange={(next) => !next && setDeleteOpen(false)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('common.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('common.deleteDesc', { name: relay.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tc('cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              onClick={() => deleteMutation.mutate()}
            >
              {t('common.confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
