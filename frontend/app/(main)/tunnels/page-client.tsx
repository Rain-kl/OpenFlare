'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, Network, Plus, RefreshCw } from 'lucide-react';
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type {
  TunnelDeployView,
  TunnelInput,
  TunnelPoolInput,
  TunnelPoolView,
  TunnelRelayDeployView,
  TunnelRelayInput,
  TunnelRelayView,
  TunnelView,
} from '@/lib/services/openflare';
import { TunnelService, tunnelQueryKey } from '@/lib/services/openflare';

import { DeployCommandDialog } from './components/deploy-command-dialog';
import { PoolEditorDialog } from './components/pool-editor-dialog';
import { PoolsTab } from './components/pools-tab';
import { RelayEditorDialog } from './components/relay-editor-dialog';
import { RelaysTab } from './components/relays-tab';
import { TopologyTab } from './components/topology-tab';
import { TunnelEditorDialog } from './components/tunnel-editor-dialog';
import { TunnelsTab } from './components/tunnels-tab';
import { getErrorMessage } from './components/tunnel-utils';

export function TunnelsPageClient() {
  const t = useTranslations('tunnels');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();

  const [activeTab, setActiveTab] = useState('tunnels');
  const [tunnelDialogOpen, setTunnelDialogOpen] = useState(false);
  const [editingTunnel, setEditingTunnel] = useState<TunnelView | null>(null);
  const [relayDialogOpen, setRelayDialogOpen] = useState(false);
  const [editingRelay, setEditingRelay] = useState<TunnelRelayView | null>(
    null,
  );
  const [poolDialogOpen, setPoolDialogOpen] = useState(false);
  const [editingPool, setEditingPool] = useState<TunnelPoolView | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<
    | { kind: 'tunnel'; id: number; name: string }
    | { kind: 'relay'; id: number; name: string }
    | { kind: 'pool'; id: number; name: string }
    | null
  >(null);
  const [deployDialog, setDeployDialog] = useState<{
    mode: 'relay' | 'tunnel';
    title: string;
    accessToken: string;
    bootstrapToken: string;
    serverUrl: string;
    deployCommand: string;
  } | null>(null);

  const topologyQuery = useQuery({
    queryKey: [...tunnelQueryKey, 'topology'],
    queryFn: () => TunnelService.getTopology(),
    refetchInterval: (q) => (q.state.status === 'error' ? false : 15000),
    refetchIntervalInBackground: false,
  });

  const bootstrapQuery = useQuery({
    queryKey: [...tunnelQueryKey, 'relay-bootstrap'],
    queryFn: () => TunnelService.getRelayBootstrapToken(),
  });

  const invalidateAll = async () => {
    await queryClient.invalidateQueries({ queryKey: tunnelQueryKey });
  };

  const saveTunnelMutation = useMutation<
    TunnelDeployView | TunnelView,
    Error,
    TunnelInput
  >({
    mutationFn: (payload: TunnelInput) =>
      editingTunnel
        ? TunnelService.updateTunnel(editingTunnel.id, payload)
        : TunnelService.createTunnel(payload),
    onSuccess: async (res) => {
      setTunnelDialogOpen(false);
      setEditingTunnel(null);
      await invalidateAll();
      if ('deploy_command' in res) {
        toast.success(t('tunnels.created'));
        setDeployDialog({
          mode: 'tunnel',
          title: t('deploy.tunnelTitle', { name: res.tunnel.name }),
          accessToken: res.access_token,
          bootstrapToken: res.bootstrap_token,
          serverUrl: res.server_url,
          deployCommand: res.deploy_command,
        });
      } else {
        toast.success(t('tunnels.updated'));
      }
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const saveRelayMutation = useMutation<
    TunnelRelayDeployView | TunnelRelayView,
    Error,
    TunnelRelayInput
  >({
    mutationFn: (payload: TunnelRelayInput) =>
      editingRelay
        ? TunnelService.updateRelay(editingRelay.id, payload)
        : TunnelService.createRelay(payload),
    onSuccess: async (res) => {
      setRelayDialogOpen(false);
      setEditingRelay(null);
      await invalidateAll();
      if ('deploy_command' in res) {
        toast.success(t('relays.created'));
        setDeployDialog({
          mode: 'relay',
          title: t('deploy.relayTitle', { name: res.relay.name }),
          accessToken: res.access_token,
          bootstrapToken: res.bootstrap_token,
          serverUrl: res.server_url,
          deployCommand: res.deploy_command,
        });
      } else {
        toast.success(t('relays.updated'));
      }
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const savePoolMutation = useMutation({
    mutationFn: (payload: TunnelPoolInput) =>
      editingPool
        ? TunnelService.updatePool(editingPool.id, payload)
        : TunnelService.createPool(payload),
    onSuccess: async () => {
      setPoolDialogOpen(false);
      setEditingPool(null);
      toast.success(t('pools.saved'));
      await invalidateAll();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!deleteTarget) return;
      if (deleteTarget.kind === 'tunnel') {
        await TunnelService.deleteTunnel(deleteTarget.id);
      } else if (deleteTarget.kind === 'relay') {
        await TunnelService.deleteRelay(deleteTarget.id);
      } else {
        await TunnelService.deletePool(deleteTarget.id);
      }
    },
    onSuccess: async () => {
      toast.success(t('common.deleted'));
      setDeleteTarget(null);
      await invalidateAll();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const rotateBootstrapMutation = useMutation({
    mutationFn: () => TunnelService.rotateRelayBootstrapToken(),
    onSuccess: async () => {
      toast.success(t('relays.discoveryRotated'));
      await queryClient.invalidateQueries({
        queryKey: [...tunnelQueryKey, 'relay-bootstrap'],
      });
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const toggleDrainMutation = useMutation({
    mutationFn: (relay: TunnelRelayView) =>
      TunnelService.setRelayDraining(relay.id, { draining: !relay.draining }),
    onSuccess: async () => {
      toast.success(t('relays.drainUpdated'));
      await invalidateAll();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const openTunnelDeploy = async (tunnel: TunnelView) => {
    try {
      const view = await TunnelService.getTunnelDeploy(tunnel.id);
      setDeployDialog({
        mode: 'tunnel',
        title: t('deploy.tunnelTitle', { name: tunnel.name }),
        accessToken: view.access_token,
        bootstrapToken: view.bootstrap_token,
        serverUrl: view.server_url,
        deployCommand: view.deploy_command,
      });
    } catch (err) {
      toast.error(getErrorMessage(err, t('common.requestFailed')));
    }
  };

  const openRelayDeploy = async (relay: TunnelRelayView) => {
    try {
      const view = await TunnelService.getRelayDeploy(relay.id);
      setDeployDialog({
        mode: 'relay',
        title: t('deploy.relayTitle', { name: relay.name }),
        accessToken: view.access_token,
        bootstrapToken: view.bootstrap_token,
        serverUrl: view.server_url,
        deployCommand: view.deploy_command,
      });
    } catch (err) {
      toast.error(getErrorMessage(err, t('common.requestFailed')));
    }
  };

  const topology = topologyQuery.data;
  const pools = topology?.pools ?? [];
  const relays = topology?.relays ?? [];
  const tunnels = topology?.tunnels ?? [];

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Network className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>

        <div className='flex flex-wrap items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            disabled={topologyQuery.isFetching}
            onClick={() => void invalidateAll()}
          >
            {topologyQuery.isFetching ? (
              <Loader2 className='size-3.5 mr-1 animate-spin' />
            ) : (
              <RefreshCw className='size-3.5 mr-1' />
            )}
            {t('common.refresh')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            onClick={() => {
              setEditingRelay(null);
              setRelayDialogOpen(true);
            }}
          >
            <Plus className='size-3.5 mr-1' />
            {t('relays.create')}
          </Button>
          <Button
            variant='secondary'
            size='sm'
            className='h-7 text-xs'
            onClick={() => {
              setEditingTunnel(null);
              setTunnelDialogOpen(true);
            }}
          >
            <Plus className='size-3.5 mr-1' />
            {t('tunnels.create')}
          </Button>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className='w-full'>
        <TabsList>
          <TabsTrigger value='tunnels'>{t('tabs.tunnels')}</TabsTrigger>
          <TabsTrigger value='relays'>{t('tabs.relays')}</TabsTrigger>
          <TabsTrigger value='pools'>{t('tabs.pools')}</TabsTrigger>
          <TabsTrigger value='topology'>{t('tabs.topology')}</TabsTrigger>
        </TabsList>

        <TabsContent value='tunnels' className='mt-4'>
          <TunnelsTab
            tunnels={tunnels}
            onDeploy={(item) => void openTunnelDeploy(item)}
            onEdit={(item) => {
              setEditingTunnel(item);
              setTunnelDialogOpen(true);
            }}
            onDelete={(item) =>
              setDeleteTarget({
                kind: 'tunnel',
                id: item.id,
                name: item.name,
              })
            }
          />
        </TabsContent>

        <TabsContent value='relays' className='mt-4'>
          <RelaysTab
            relays={relays}
            onDeploy={(item) => void openRelayDeploy(item)}
            onToggleDrain={(item) => toggleDrainMutation.mutate(item)}
            onEdit={(item) => {
              setEditingRelay(item);
              setRelayDialogOpen(true);
            }}
            onDelete={(item) =>
              setDeleteTarget({ kind: 'relay', id: item.id, name: item.name })
            }
          />
        </TabsContent>

        <TabsContent value='pools' className='mt-4'>
          <PoolsTab
            pools={pools}
            onCreate={() => {
              setEditingPool(null);
              setPoolDialogOpen(true);
            }}
            onEdit={(item) => {
              setEditingPool(item);
              setPoolDialogOpen(true);
            }}
            onDelete={(item) =>
              setDeleteTarget({ kind: 'pool', id: item.id, name: item.name })
            }
          />
        </TabsContent>

        <TabsContent value='topology' className='mt-4'>
          <TopologyTab topology={topology} />
        </TabsContent>
      </Tabs>

      <TunnelEditorDialog
        open={tunnelDialogOpen}
        tunnel={editingTunnel}
        pools={pools}
        submitting={saveTunnelMutation.isPending}
        onClose={() => {
          setTunnelDialogOpen(false);
          setEditingTunnel(null);
        }}
        onSubmit={async (payload) => {
          await saveTunnelMutation.mutateAsync(payload);
        }}
      />

      <RelayEditorDialog
        open={relayDialogOpen}
        relay={editingRelay}
        pools={pools}
        bootstrap={bootstrapQuery.data}
        rotatingBootstrap={rotateBootstrapMutation.isPending}
        onRotateBootstrap={() => rotateBootstrapMutation.mutate()}
        submitting={saveRelayMutation.isPending}
        onClose={() => {
          setRelayDialogOpen(false);
          setEditingRelay(null);
        }}
        onSubmit={async (payload) => {
          await saveRelayMutation.mutateAsync(payload);
        }}
      />

      <PoolEditorDialog
        open={poolDialogOpen}
        pool={editingPool}
        submitting={savePoolMutation.isPending}
        onClose={() => {
          setPoolDialogOpen(false);
          setEditingPool(null);
        }}
        onSubmit={async (payload) => {
          await savePoolMutation.mutateAsync(payload);
        }}
      />

      {deployDialog ? (
        <DeployCommandDialog
          open={Boolean(deployDialog)}
          mode={deployDialog.mode}
          title={deployDialog.title}
          accessToken={deployDialog.accessToken}
          bootstrapToken={deployDialog.bootstrapToken}
          serverUrl={deployDialog.serverUrl}
          deployCommand={deployDialog.deployCommand}
          onClose={() => setDeployDialog(null)}
        />
      ) : null}

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(next) => !next && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('common.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('common.deleteDesc', { name: deleteTarget?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-white hover:bg-destructive/90'
              disabled={deleteMutation.isPending}
              onClick={() => deleteMutation.mutate()}
            >
              {deleteMutation.isPending
                ? t('common.deleting')
                : t('common.confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
