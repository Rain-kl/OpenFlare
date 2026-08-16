'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import {
  Activity,
  Copy,
  ExternalLink,
  FileText,
  Fingerprint,
  Loader2,
  Network,
  RefreshCw,
  RotateCcw,
  Server,
  Trash2,
  Upload,
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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { formatDateTime } from '@/lib/utils';
import type {
  NodeAgentReleaseInfo,
  NodeItem,
  ReleaseChannel,
} from '@/lib/services/openflare';
import { NodeService } from '@/lib/services/openflare';
import services from '@/lib/services';
import type { SystemConfig } from '@/lib/services/admin';

import { AgentUpdateDialog } from './agent-update-dialog';
import { InstallCommand } from './install-command';
import { NodeDetailShell } from './node-detail-shell';
import {
  NodeErrorBanner,
  NodeInfoRow,
  NodeSectionCard,
} from './node-detail-primitives';
import { NodeEditorDialog } from './node-editor-dialog';
import { NodeObservability } from './node-observability';
import { NodeStatusBadge } from './node-status-badge';
import {
  formatRelativeTime,
  getApplyLabel,
  getApplyTone,
  getErrorMessage,
  getNodeStatusLabel,
  getNodeStatusTone,
  getRelayStatusLabel,
  getRelayStatusTone,
  isMeaningfulTime,
  isWSConnectedLastSeen,
} from './node-utils';

const nodesQueryKey = ['openflare', 'nodes'];

function systemConfigMap(configs: SystemConfig[]) {
  return configs.reduce<Record<string, SystemConfig>>((accumulator, config) => {
    accumulator[config.key] = config;
    return accumulator;
  }, {});
}

export function RelayNodeDetail({ node }: { node: NodeItem }) {
  const t = useTranslations('nodes');
  const tc = useTranslations('common');
  const router = useRouter();
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const [webServerPort, setWebServerPort] = useState('17500');

  const systemConfigsQuery = useQuery({
    queryKey: ['admin', 'system-configs'],
    queryFn: () => services.adminSystemConfig.listSystemConfigs(),
  });

  const configs = useMemo(
    () => systemConfigMap(systemConfigsQuery.data ?? []),
    [systemConfigsQuery.data],
  );

  useEffect(() => {
    if (systemConfigsQuery.data) {
      setWebServerPort(configs['relay_frps_web_ui_port']?.value || '17500');
    }
  }, [systemConfigsQuery.data, configs]);

  const savePortMutation = useMutation({
    mutationFn: async (port: string) => {
      const portNum = parseInt(port, 10);
      if (isNaN(portNum) || portNum < 1 || portNum > 65535) {
        throw new Error(t('relay.invalidPort'));
      }
      const portCfg = configs['relay_frps_web_ui_port'];
      await services.adminSystemConfig.updateSystemConfig(
        'relay_frps_web_ui_port',
        {
          value: String(portNum),
          description: portCfg?.description || 'FRPS 内置 Web 管理界面端口',
        },
      );
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'system-configs'],
      });
      toast.success(t('relay.toastPortSaved'));
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('relay.portSaveFailed'),
      ),
  });

  const saveMutation = useMutation({
    mutationFn: (payload: Parameters<typeof NodeService.updateNode>[1]) =>
      NodeService.updateNode(node.id, payload),
    onSuccess: async () => {
      toast.success(t('relay.toastUpdated'));
      setEditorOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const webServerMutation = useMutation({
    mutationFn: (enabled: boolean) =>
      NodeService.updateNode(node.id, {
        node_type: node.node_type,
        name: node.name,
        ip: node.ip,
        ip_manual_override: node.ip_manual_override,
        auto_update_enabled: node.auto_update_enabled,
        geo_name: node.geo_name,
        geo_latitude: node.geo_latitude ?? null,
        geo_longitude: node.geo_longitude ?? null,
        geo_manual_override: node.geo_manual_override,
        relay_bind_port: node.relay_bind_port,
        relay_vhost_http_port: node.relay_vhost_http_port,
        relay_web_server_enabled: enabled,
      }),
    onSuccess: async () => {
      toast.success(t('relay.toastWebui'));
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const forceSyncMutation = useMutation({
    mutationFn: () => NodeService.requestForceSync(node.id),
    onSuccess: async (updated) => {
      toast.success(t('relay.toastForceSync', { name: updated.name }));
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const upgradeMutation = useMutation({
    mutationFn: ({
      release,
      channel,
    }: {
      release: NodeAgentReleaseInfo | null;
      channel: ReleaseChannel;
    }) =>
      NodeService.requestAgentUpdate(node.id, {
        channel: release?.channel ?? channel,
        tag_name:
          release?.channel === 'preview'
            ? release.tag_name || undefined
            : undefined,
      }),
    onSuccess: async (updated) => {
      toast.success(
        t('relay.toastUpgrade', {
          name: updated.name,
          channel:
            updated.update_channel === 'preview'
              ? t('channel.preview')
              : t('channel.stable'),
        }),
      );
      setUpgradeOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const deleteMutation = useMutation({
    mutationFn: () => NodeService.deleteNode(node.id),
    onSuccess: async () => {
      toast.success(t('relay.toastDeleted'));
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
      router.push('/nodes');
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const handleRefresh = () => {
    void Promise.all([
      queryClient.invalidateQueries({ queryKey: nodesQueryKey }),
      queryClient.invalidateQueries({
        queryKey: ['openflare', 'node-observability', node.id],
      }),
    ]);
  };

  const webServerPortNum = parseInt(
    configs['relay_frps_web_ui_port']?.value || '',
    10,
  );
  const resolvedPort =
    !isNaN(webServerPortNum) && webServerPortNum > 0
      ? webServerPortNum
      : (node.relay_bind_port || 7000) + 500;

  const webUiUrl =
    node.relay_web_server_enabled && node.relay_bind_port
      ? `http://${node.ip || '127.0.0.1'}:${resolvedPort}`
      : null;

  const headerActions = (
    <>
      <Button
        variant='outline'
        size='sm'
        className='h-8'
        onClick={() => setEditorOpen(true)}
      >
        {t('edit')}
      </Button>
      <Button
        variant='outline'
        size='sm'
        className='h-8'
        onClick={handleRefresh}
      >
        <RefreshCw className='size-3.5 mr-1.5' />
        {t('refresh')}
      </Button>
      <Button
        variant='outline'
        size='sm'
        className='h-8'
        disabled={forceSyncMutation.isPending}
        onClick={() => forceSyncMutation.mutate()}
      >
        <RotateCcw className='size-3.5 mr-1.5' />
        {forceSyncMutation.isPending ? t('syncing') : t('forceSync')}
      </Button>
      <Button
        variant='secondary'
        size='sm'
        className='h-8'
        onClick={() => setUpgradeOpen(true)}
      >
        <Upload className='size-3.5 mr-1.5' />
        {node.update_requested ? t('viewUpgrade') : t('relay.upgrade')}
      </Button>
      <Button
        variant='outline'
        size='sm'
        className='h-8 text-destructive hover:text-destructive'
        onClick={() => setDeleteOpen(true)}
      >
        <Trash2 className='size-3.5 mr-1.5' />
        {t('delete')}
      </Button>
    </>
  );

  const manageTab = (
    <div className='space-y-6'>
      {node.last_error ? <NodeErrorBanner message={node.last_error} /> : null}

      <div className='grid gap-6 xl:grid-cols-2'>
        <NodeSectionCard
          title={t('relay.runStatus')}
          description={t('relay.runStatusDesc')}
        >
          <div className='divide-y'>
            <NodeInfoRow label={t('edge.runStatus')}>
              <NodeStatusBadge
                label={getNodeStatusLabel(node.status, t)}
                tone={getNodeStatusTone(node.status)}
              />
            </NodeInfoRow>
            <NodeInfoRow label={t('relay.health')}>
              <NodeStatusBadge
                label={getRelayStatusLabel(node.relay_status, t)}
                tone={getRelayStatusTone(node.relay_status)}
              />
            </NodeInfoRow>
            <NodeInfoRow label={t('kpiLastSeen')}>
              {isWSConnectedLastSeen(node.last_seen_at)
                ? t('wsConnected')
                : isMeaningfulTime(node.last_seen_at)
                  ? `${formatRelativeTime(node.last_seen_at, t)} · ${formatDateTime(node.last_seen_at)}`
                  : t('na')}
            </NodeInfoRow>
            <NodeInfoRow label={t('relay.connections')}>
              {node.relay_frps_connections ?? '—'} /{' '}
              {node.relay_frps_proxy_count ?? '—'}
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.ip')}>
              {node.ip || '—'}
              {node.ip_manual_override ? t('lockedSuffix') : ''}
            </NodeInfoRow>
          </div>
        </NodeSectionCard>

        <NodeSectionCard
          title={t('relay.portsTitle')}
          description={t('relay.portsDesc')}
          action={
            <Button variant='outline' size='sm' className='h-8' asChild>
              <Link
                href={`/apply-logs?node_id=${encodeURIComponent(node.node_id)}`}
              >
                <FileText className='size-3.5 mr-1.5' />
                {t('applyLogs')}
              </Link>
            </Button>
          }
        >
          <div className='divide-y'>
            <NodeInfoRow label={t('relay.bindPort')}>
              {node.relay_bind_port || '—'}
            </NodeInfoRow>
            <NodeInfoRow label={t('relay.vhostPort')}>
              {node.relay_vhost_http_port || '—'}
            </NodeInfoRow>
            <NodeInfoRow label={t('relay.agentAddr')}>
              <span className='break-all'>
                {node.relay_agent_access_addr || '—'}
              </span>
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.latestApply')}>
              <NodeStatusBadge
                label={getApplyLabel(node.latest_apply_result, t)}
                tone={getApplyTone(node.latest_apply_result)}
              />
            </NodeInfoRow>
            <NodeInfoRow label={t('relay.relayVersion')}>
              {node.version || 'unknown'}
            </NodeInfoRow>
            <NodeInfoRow label={t('relay.frpsVersion')}>
              {node.ext_version || 'unknown'}
            </NodeInfoRow>
          </div>
        </NodeSectionCard>
      </div>

      <NodeSectionCard
        title={t('relay.webuiTitle')}
        description={t('relay.webuiDesc')}
      >
        <div className='space-y-4'>
          <div className='flex items-center justify-between rounded-xl border px-4 py-4'>
            <div className='space-y-1'>
              <Label>{t('relay.enableWebui')}</Label>
              <p className='text-xs text-muted-foreground'>
                {t('relay.enableWebuiHint')}
              </p>
            </div>
            <Switch
              checked={node.relay_web_server_enabled}
              disabled={webServerMutation.isPending}
              onCheckedChange={(checked) => webServerMutation.mutate(checked)}
            />
          </div>

          <div className='rounded-xl border p-4 space-y-3'>
            <Label htmlFor='web_server_port' className='text-xs font-semibold'>
              {t('relay.webuiPort')}
            </Label>
            <div className='flex max-w-sm items-center gap-2'>
              <Input
                id='web_server_port'
                type='number'
                min={1}
                max={65535}
                value={webServerPort}
                onChange={(e) => setWebServerPort(e.target.value)}
                placeholder={t('relay.webuiPortPlaceholder')}
                className='bg-card text-xs h-9'
                disabled={
                  savePortMutation.isPending || systemConfigsQuery.isLoading
                }
              />
              <Button
                size='sm'
                className='h-9'
                onClick={() => savePortMutation.mutate(webServerPort)}
                disabled={
                  savePortMutation.isPending || systemConfigsQuery.isLoading
                }
              >
                {savePortMutation.isPending ? (
                  <Loader2 className='size-3.5 animate-spin' />
                ) : (
                  tc('save')
                )}
              </Button>
            </div>
            <p className='text-[11px] text-muted-foreground'>
              {t('relay.actualPort', { port: resolvedPort })}
            </p>
          </div>

          {webUiUrl ? (
            <div className='flex flex-wrap items-center gap-3 text-sm'>
              <a
                href={webUiUrl}
                target='_blank'
                rel='noopener noreferrer'
                className='inline-flex items-center font-medium text-primary hover:underline'
              >
                {t('relay.openWebui')}
                <ExternalLink className='size-3.5 ml-1.5' />
              </a>
              <span className='text-xs text-muted-foreground font-mono bg-muted px-2 py-0.5 rounded border'>
                {webUiUrl}
              </span>
              <Button
                variant='ghost'
                size='icon'
                className='size-7 text-muted-foreground hover:text-foreground'
                onClick={() => {
                  void navigator.clipboard.writeText(webUiUrl);
                  toast.success(t('relay.copiedLink'));
                }}
                title={t('relay.copyLink')}
              >
                <Copy className='size-3.5' />
              </Button>
            </div>
          ) : (
            <p className='text-sm text-muted-foreground'>
              {t('relay.webuiDisabled')}
            </p>
          )}
        </div>
      </NodeSectionCard>

      <InstallCommand node={node} variant='relay' />

      <NodeSectionCard title={t('relay.metadata')}>
        <div className='divide-y'>
          <NodeInfoRow label={t('kpiNodeId')}>
            <span className='font-mono text-xs break-all'>{node.node_id}</span>
          </NodeInfoRow>
          <NodeInfoRow label={t('relay.createdAt')}>
            {formatDateTime(node.created_at)}
          </NodeInfoRow>
          <NodeInfoRow label={t('relay.updatedAt')}>
            {formatDateTime(node.updated_at)}
          </NodeInfoRow>
        </div>
      </NodeSectionCard>
    </div>
  );

  return (
    <>
      <NodeDetailShell
        title={node.name}
        typeLabel='Relay'
        typeTone='warning'
        statusBadges={[
          {
            label: getNodeStatusLabel(node.status, t),
            tone: getNodeStatusTone(node.status),
          },
          {
            label: getRelayStatusLabel(node.relay_status, t),
            tone: getRelayStatusTone(node.relay_status),
          },
        ]}
        actions={headerActions}
        kpis={[
          { label: t('kpiNodeId'), value: node.node_id, icon: Fingerprint },
          {
            label: t('relay.relayVersion'),
            value: node.version || 'unknown',
            icon: Server,
          },
          {
            label: t('relay.kpiFrps'),
            value: node.ext_version || 'unknown',
            icon: Network,
          },
          {
            label: t('kpiLastSeen'),
            value: isWSConnectedLastSeen(node.last_seen_at)
              ? t('wsConnected')
              : isMeaningfulTime(node.last_seen_at)
                ? formatRelativeTime(node.last_seen_at, t)
                : t('na'),
            icon: Activity,
          },
        ]}
        overview={
          <NodeObservability
            nodeId={node.id}
            variant='compact'
            connectionHint={t('relay.connectionHint')}
          />
        }
        manage={manageTab}
        defaultTab='overview'
      />

      <NodeEditorDialog
        open={editorOpen}
        node={node}
        submitting={saveMutation.isPending}
        onClose={() => setEditorOpen(false)}
        onSubmit={async (payload) => {
          await saveMutation.mutateAsync(payload);
        }}
      />

      <AgentUpdateDialog
        open={upgradeOpen}
        node={node}
        submitting={upgradeMutation.isPending}
        onClose={() => setUpgradeOpen(false)}
        onConfirm={async (release, channel) => {
          await upgradeMutation.mutateAsync({ release, channel });
        }}
      />

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('relay.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('relay.deleteDesc', { name: node.name })}
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
              {deleteMutation.isPending ? t('deleting') : t('confirmDelete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
