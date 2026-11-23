'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  Download,
  Network,
  Pencil,
  Plus,
  RotateCcw,
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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type {
  TunnelDeployView,
  TunnelInput,
  TunnelServiceInput,
  TunnelServiceView,
  TunnelView,
} from '@/lib/services/openflare';
import { TunnelService, tunnelQueryKey } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { DeployCommandDialog } from '../components/deploy-command-dialog';
import { ServiceEditorDialog } from '../components/service-editor-dialog';
import { TunnelEditorDialog } from '../components/tunnel-editor-dialog';
import { TunnelStatusBadge } from '../components/tunnel-status-badge';
import {
  getErrorMessage,
  getTunnelStatusTone,
} from '../components/tunnel-utils';

export function TunnelDetailPageClient() {
  const t = useTranslations('tunnels');
  const tc = useTranslations('common');
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const rawId = searchParams.get('id')?.trim() ?? '';
  const tunnelId = Number(rawId);
  const validId = Number.isSafeInteger(tunnelId) && tunnelId > 0;

  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [svcOpen, setSvcOpen] = useState(false);
  const [editingSvc, setEditingSvc] = useState<TunnelServiceView | null>(null);
  const [deployDialog, setDeployDialog] = useState<{
    accessToken: string;
    bootstrapToken: string;
    serverUrl: string;
    deployCommand: string;
  } | null>(null);

  const detailQuery = useQuery({
    queryKey: [...tunnelQueryKey, 'detail', tunnelId],
    queryFn: () => TunnelService.getTunnel(tunnelId),
    enabled: validId,
    refetchInterval: (q) => (q.state.status === 'error' ? false : 15000),
    refetchIntervalInBackground: false,
  });

  const poolsQuery = useQuery({
    queryKey: [...tunnelQueryKey, 'pools'],
    queryFn: () => TunnelService.listPools(),
    enabled: validId,
  });

  const relaysQuery = useQuery({
    queryKey: [...tunnelQueryKey, 'relays'],
    queryFn: () => TunnelService.listRelays(),
    enabled: validId,
  });

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: tunnelQueryKey });
  };

  const updateMutation = useMutation({
    mutationFn: (payload: TunnelInput) =>
      TunnelService.updateTunnel(tunnelId, payload),
    onSuccess: async () => {
      setEditOpen(false);
      toast.success(t('tunnels.updated'));
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const rotateTokenMutation = useMutation({
    mutationFn: () => TunnelService.rotateTunnelToken(tunnelId),
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
    TunnelDeployView | TunnelView,
    Error,
    boolean
  >({
    mutationFn: (quarantined: boolean) =>
      quarantined
        ? TunnelService.unquarantineTunnel(tunnelId)
        : TunnelService.quarantineTunnel(tunnelId, ''),
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

  const binaryUpdateMutation = useMutation({
    mutationFn: () =>
      TunnelService.requestTunnelBinaryUpdate(tunnelId, { channel: 'stable' }),
    onSuccess: async () => {
      toast.success(t('detail.updateRequested'));
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const deleteMutation = useMutation({
    mutationFn: () => TunnelService.deleteTunnel(tunnelId),
    onSuccess: async () => {
      toast.success(t('common.deleted'));
      await invalidate();
      router.push('/tunnels');
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const revokeConnectorMutation = useMutation({
    mutationFn: (connectorId: string) =>
      TunnelService.revokeConnector(tunnelId, connectorId),
    onSuccess: async () => {
      toast.success(t('detail.connectorRevoked'));
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const saveSvcMutation = useMutation({
    mutationFn: (payload: TunnelServiceInput) =>
      editingSvc
        ? TunnelService.updateService(tunnelId, editingSvc.id, payload)
        : TunnelService.createService(tunnelId, payload),
    onSuccess: async () => {
      setSvcOpen(false);
      setEditingSvc(null);
      toast.success(t('services.saved'));
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  const deleteSvcMutation = useMutation({
    mutationFn: (serviceId: number) =>
      TunnelService.deleteService(tunnelId, serviceId),
    onSuccess: async () => {
      toast.success(t('common.deleted'));
      await invalidate();
    },
    onError: (err) =>
      toast.error(getErrorMessage(err, t('common.requestFailed'))),
  });

  if (!validId) {
    return (
      <div className='py-6 px-1'>
        <EmptyStateWithBorder
          icon={Network}
          description={t('detail.missingId')}
        />
      </div>
    );
  }
  if (detailQuery.isLoading) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder
          icon={Network}
          description={t('detail.loading')}
        />
      </div>
    );
  }
  if (detailQuery.isError || !detailQuery.data) {
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
            message={getErrorMessage(detailQuery.error, t('detail.notFound'))}
            onRetry={() => void detailQuery.refetch()}
            className='justify-center'
          />
        </div>
      </div>
    );
  }

  const { tunnel, connectors, attachments, services, bound_routes } =
    detailQuery.data;
  const isQuarantined = Boolean(tunnel.quarantined_at);

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
            <Network className='size-5 text-primary' />
            <h1 className='text-2xl font-semibold tracking-tight'>
              {tunnel.name}
            </h1>
            <TunnelStatusBadge
              label={t(`status.${tunnel.status}`)}
              tone={getTunnelStatusTone(tunnel.status)}
            />
          </div>
          <p className='text-xs font-mono text-muted-foreground'>
            {tunnel.tunnel_id} · rev {tunnel.config_revision}
          </p>
        </div>

        <div className='flex flex-wrap gap-2'>
          <Button
            variant='secondary'
            size='sm'
            className='h-7 text-xs'
            onClick={() =>
              void TunnelService.getTunnelDeploy(tunnel.id).then((view) =>
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
            onClick={() => rotateTokenMutation.mutate()}
          >
            <RotateCcw className='size-3.5 mr-1' />
            {t('detail.rotateToken')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            className='h-7 text-xs'
            onClick={() => binaryUpdateMutation.mutate()}
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

      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
        <Card className='border-dashed shadow-none'>
          <CardHeader className='pb-2'>
            <CardDescription>{t('detail.kpiRevision')}</CardDescription>
            <CardTitle className='text-2xl tabular-nums'>
              #{tunnel.config_revision}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card className='border-dashed shadow-none'>
          <CardHeader className='pb-2'>
            <CardDescription>{t('detail.kpiConnectors')}</CardDescription>
            <CardTitle className='text-2xl tabular-nums'>
              {tunnel.online_connectors} / {tunnel.max_connectors || '∞'}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card className='border-dashed shadow-none'>
          <CardHeader className='pb-2'>
            <CardDescription>{t('detail.kpiAttachments')}</CardDescription>
            <CardTitle className='text-2xl tabular-nums'>
              {tunnel.active_attachments} (K={tunnel.ha_relay_count})
            </CardTitle>
          </CardHeader>
        </Card>
        <Card className='border-dashed shadow-none'>
          <CardHeader className='pb-2'>
            <CardDescription>{t('detail.kpiRoutes')}</CardDescription>
            <CardTitle className='text-2xl tabular-nums'>
              {tunnel.bound_routes_count}
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('detail.policyTitle')}
          </CardTitle>
        </CardHeader>
        <CardContent className='grid gap-3 text-sm sm:grid-cols-2'>
          <div>
            <p className='text-xs text-muted-foreground'>{t('tunnels.pool')}</p>
            <p className='font-medium'>{tunnel.pool_key}</p>
          </div>
          <div>
            <p className='text-xs text-muted-foreground'>
              {t('tunnels.preferredTransport')}
            </p>
            <p className='font-medium'>
              {tunnel.preferred_transport} (
              {(tunnel.fallback_transports ?? []).join(', ')})
            </p>
          </div>
          <div>
            <p className='text-xs text-muted-foreground'>
              {t('tunnels.allowedTargetCidrs')}
            </p>
            <p className='font-mono text-xs'>
              {(tunnel.allowed_target_cidrs ?? []).join(', ') || '*'}
            </p>
          </div>
          <div>
            <p className='text-xs text-muted-foreground'>
              {t('tunnels.blockedTargetCidrs')}
            </p>
            <p className='font-mono text-xs'>
              {(tunnel.blocked_target_cidrs ?? []).join(', ') || '—'}
            </p>
          </div>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('detail.connectorsTitle')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='border border-dashed rounded-lg overflow-hidden'>
            <Table>
              <TableHeader>
                <TableRow className='border-dashed hover:bg-transparent'>
                  <TableHead className='h-8 py-2'>
                    {t('topology.connector')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('tunnels.status')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>IP / OS</TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('relays.lastSeen')}
                  </TableHead>
                  <TableHead className='h-8 py-2 text-right'>
                    {t('common.actions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {connectors.map((c) => (
                  <TableRow key={c.id} className='border-dashed'>
                    <TableCell className='py-2.5 font-mono text-xs'>
                      <div>{c.connector_id}</div>
                      <div className='text-muted-foreground'>
                        {c.hostname || '—'} · {c.version || 'dev'}
                      </div>
                    </TableCell>
                    <TableCell className='py-2.5'>
                      <TunnelStatusBadge
                        label={t(`status.${c.effective_status}`)}
                        tone={getTunnelStatusTone(c.effective_status)}
                      />
                    </TableCell>
                    <TableCell className='py-2.5 text-xs text-muted-foreground'>
                      {c.public_ip || '—'} · {c.os_arch || '—'}
                    </TableCell>
                    <TableCell className='py-2.5 text-xs text-muted-foreground'>
                      {c.last_seen_at
                        ? formatDateTime(c.last_seen_at)
                        : t('common.never')}
                    </TableCell>
                    <TableCell className='py-2.5 text-right'>
                      <Button
                        variant='outline'
                        size='sm'
                        className='h-7 text-xs text-destructive hover:text-destructive'
                        disabled={Boolean(c.revoked_at)}
                        onClick={() =>
                          revokeConnectorMutation.mutate(c.connector_id)
                        }
                      >
                        {t('detail.revoke')}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('topology.attachmentsTitle')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='border border-dashed rounded-lg overflow-hidden'>
            <Table>
              <TableHeader>
                <TableRow className='border-dashed hover:bg-transparent'>
                  <TableHead className='h-8 py-2'>
                    {t('topology.relay')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('topology.connector')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('topology.transport')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('topology.state')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('topology.rttStreams')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {attachments.map((att) => (
                  <TableRow key={att.id} className='border-dashed'>
                    <TableCell className='py-2.5 text-sm'>
                      {att.relay_name || att.relay_public_id}
                    </TableCell>
                    <TableCell className='py-2.5 font-mono text-xs'>
                      {att.connector_id}
                    </TableCell>
                    <TableCell className='py-2.5 text-xs uppercase'>
                      {att.transport}
                    </TableCell>
                    <TableCell className='py-2.5'>
                      <TunnelStatusBadge
                        label={t(`status.${att.state}`)}
                        tone={getTunnelStatusTone(att.state)}
                      />
                    </TableCell>
                    <TableCell className='py-2.5 text-xs text-muted-foreground tabular-nums'>
                      {att.rtt_ms} ms · {att.active_streams}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <div className='flex items-center justify-between gap-2'>
            <CardTitle className='text-base font-semibold'>
              {t('services.listTitle')}
            </CardTitle>
            <Button
              size='sm'
              variant='secondary'
              className='h-7 text-xs'
              onClick={() => {
                setEditingSvc(null);
                setSvcOpen(true);
              }}
            >
              <Plus className='size-3.5 mr-1' />
              {t('services.create')}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className='border border-dashed rounded-lg overflow-hidden'>
            <Table>
              <TableHeader>
                <TableRow className='border-dashed hover:bg-transparent'>
                  <TableHead className='h-8 py-2'>
                    {t('services.name')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('services.protocol')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('services.listenPort')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('services.targetAddr')}
                  </TableHead>
                  <TableHead className='h-8 py-2 text-right'>
                    {t('common.actions')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {services.map((svc) => (
                  <TableRow key={svc.id} className='border-dashed'>
                    <TableCell className='py-2.5 font-medium'>
                      {svc.name}
                    </TableCell>
                    <TableCell className='py-2.5 text-xs uppercase'>
                      {svc.service_type}
                    </TableCell>
                    <TableCell className='py-2.5 font-mono text-xs'>
                      {svc.listen_port}
                    </TableCell>
                    <TableCell className='py-2.5 font-mono text-xs'>
                      {svc.target_addr}
                    </TableCell>
                    <TableCell className='py-2.5'>
                      <div className='flex justify-end gap-1'>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs'
                          onClick={() => {
                            setEditingSvc(svc);
                            setSvcOpen(true);
                          }}
                        >
                          {t('common.edit')}
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs text-destructive hover:text-destructive'
                          onClick={() => deleteSvcMutation.mutate(svc.id)}
                        >
                          {t('common.delete')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='pb-3'>
          <CardTitle className='text-base font-semibold'>
            {t('detail.boundRoutesTitle')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='border border-dashed rounded-lg overflow-hidden'>
            <Table>
              <TableHeader>
                <TableRow className='border-dashed hover:bg-transparent'>
                  <TableHead className='h-8 py-2'>
                    {t('detail.routeSite')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('detail.routeDomains')}
                  </TableHead>
                  <TableHead className='h-8 py-2'>
                    {t('detail.routeTarget')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {bound_routes.map((route) => (
                  <TableRow key={route.route_id} className='border-dashed'>
                    <TableCell className='py-2.5'>
                      <Link
                        href={`/proxy-routes/detail?id=${route.route_id}`}
                        className='font-medium hover:underline'
                      >
                        {route.site_name}
                      </Link>
                    </TableCell>
                    <TableCell className='py-2.5 text-xs'>
                      {(route.domains ?? []).join(', ') || '—'}
                    </TableCell>
                    <TableCell className='py-2.5 font-mono text-xs'>
                      {route.tunnel_target_protocol || 'http'}://
                      {route.tunnel_target_addr}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <TunnelEditorDialog
        open={editOpen}
        tunnel={tunnel}
        pools={poolsQuery.data ?? []}
        submitting={updateMutation.isPending}
        onClose={() => setEditOpen(false)}
        onSubmit={async (payload) => {
          await updateMutation.mutateAsync(payload);
        }}
      />

      <ServiceEditorDialog
        open={svcOpen}
        service={editingSvc}
        relays={relaysQuery.data ?? []}
        submitting={saveSvcMutation.isPending}
        onClose={() => {
          setSvcOpen(false);
          setEditingSvc(null);
        }}
        onSubmit={async (payload) => {
          await saveSvcMutation.mutateAsync(payload);
        }}
      />

      {deployDialog ? (
        <DeployCommandDialog
          open={Boolean(deployDialog)}
          mode='tunnel'
          title={t('deploy.tunnelTitle', { name: tunnel.name })}
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
              {t('common.deleteDesc', { name: tunnel.name })}
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
