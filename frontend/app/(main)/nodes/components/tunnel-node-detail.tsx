'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  Activity,
  FileText,
  Fingerprint,
  KeyRound,
  Package,
  RefreshCw,
  RotateCcw,
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
import { formatDateTime } from '@/lib/utils';
import type {
  NodeAgentReleaseInfo,
  NodeItem,
  ReleaseChannel,
} from '@/lib/services/openflare';
import { NodeService } from '@/lib/services/openflare';

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
  getFlaredStatusLabel,
  getFlaredStatusTone,
  getNodeStatusLabel,
  getNodeStatusTone,
  isMeaningfulTime,
  isWSConnectedLastSeen,
} from './node-utils';

const nodesQueryKey = ['openflare', 'nodes'];

export function TunnelNodeDetail({ node }: { node: NodeItem }) {
  const t = useTranslations('nodes');
  const tc = useTranslations('common');
  const router = useRouter();
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const saveMutation = useMutation({
    mutationFn: (payload: Parameters<typeof NodeService.updateNode>[1]) =>
      NodeService.updateNode(node.id, payload),
    onSuccess: async () => {
      toast.success(t('tunnel.toastUpdated'));
      setEditorOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const forceSyncMutation = useMutation({
    mutationFn: () => NodeService.requestForceSync(node.id),
    onSuccess: async (updated) => {
      toast.success(t('tunnel.toastForceSync', { name: updated.name }));
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
        t('tunnel.toastUpgrade', {
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
      toast.success(t('tunnel.toastDeleted'));
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
        {node.update_requested ? t('viewUpgrade') : t('tunnel.upgrade')}
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
          title={t('tunnel.runStatus')}
          description={t('tunnel.runStatusDesc')}
        >
          <div className='divide-y'>
            <NodeInfoRow label={t('edge.runStatus')}>
              <NodeStatusBadge
                label={getNodeStatusLabel(node.status, t)}
                tone={getNodeStatusTone(node.status)}
              />
            </NodeInfoRow>
            <NodeInfoRow label={t('tunnel.flaredStatus')}>
              <NodeStatusBadge
                label={getFlaredStatusLabel(node, t)}
                tone={getFlaredStatusTone(node)}
              />
            </NodeInfoRow>
            <NodeInfoRow label={t('kpiLastSeen')}>
              {isWSConnectedLastSeen(node.last_seen_at)
                ? t('wsEstablished')
                : isMeaningfulTime(node.last_seen_at)
                  ? `${formatRelativeTime(node.last_seen_at, t)} · ${formatDateTime(node.last_seen_at)}`
                  : t('na')}
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.ip')}>
              {node.ip || '—'}
              {node.ip_manual_override ? t('lockedSuffix') : ''}
            </NodeInfoRow>
            <NodeInfoRow label={t('tunnel.autoUpdate')}>
              {node.auto_update_enabled ? t('enabled') : t('manual')}
            </NodeInfoRow>
          </div>
        </NodeSectionCard>

        <NodeSectionCard
          title={t('tunnel.syncTitle')}
          description={t('tunnel.syncDesc')}
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
            <NodeInfoRow label={t('tunnel.currentVersion')}>
              {node.current_version || t('notApplied')}
            </NodeInfoRow>
            <NodeInfoRow label={t('tunnel.latestApply')}>
              <NodeStatusBadge
                label={getApplyLabel(node.latest_apply_result, t)}
                tone={getApplyTone(node.latest_apply_result)}
              />
            </NodeInfoRow>
            <NodeInfoRow label={t('tunnel.latestApplyAt')}>
              {isMeaningfulTime(node.latest_apply_at)
                ? `${formatRelativeTime(node.latest_apply_at, t)} · ${formatDateTime(node.latest_apply_at)}`
                : t('na')}
            </NodeInfoRow>
            {node.latest_apply_checksum ? (
              <NodeInfoRow label={t('tunnel.fileCount')}>
                {t('tunnel.fileCountValue', {
                  count: node.latest_support_file_count,
                })}
              </NodeInfoRow>
            ) : null}
            <NodeInfoRow label={t('tunnel.upgradeStatus')}>
              <NodeStatusBadge
                label={
                  node.update_requested
                    ? node.update_channel === 'preview'
                      ? t('update.waitingPreview')
                      : t('update.waitingUpdate')
                    : t('update.notRequested')
                }
                tone={node.update_requested ? 'warning' : 'info'}
              />
            </NodeInfoRow>
          </div>
        </NodeSectionCard>
      </div>

      <InstallCommand node={node} variant='tunnel' />

      <NodeSectionCard
        title={t('tunnel.credentials')}
        description={t('tunnel.credentialsDesc')}
      >
        <div className='divide-y'>
          <NodeInfoRow label={t('tunnel.tunnelToken')}>
            <span className='font-mono text-xs break-all'>
              {node.access_token || t('na')}
            </span>
          </NodeInfoRow>
          <NodeInfoRow label={t('kpiNodeId')}>
            <span className='font-mono text-xs break-all'>{node.node_id}</span>
          </NodeInfoRow>
          <NodeInfoRow label={t('tunnel.createdAt')}>
            {formatDateTime(node.created_at)}
          </NodeInfoRow>
          <NodeInfoRow label={t('tunnel.updatedAt')}>
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
        typeLabel='Tunnel'
        typeTone='info'
        statusBadges={[
          {
            label: getNodeStatusLabel(node.status, t),
            tone: getNodeStatusTone(node.status),
          },
          {
            label: getFlaredStatusLabel(node, t),
            tone: getFlaredStatusTone(node),
          },
        ]}
        actions={headerActions}
        kpis={[
          { label: t('kpiNodeId'), value: node.node_id, icon: Fingerprint },
          {
            label: t('tunnel.kpiOpenflared'),
            value: node.version || 'unknown',
            icon: Package,
          },
          {
            label: t('kpiCurrentConfig'),
            value: node.current_version || t('notApplied'),
            icon: KeyRound,
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
            connectionHint={t('tunnel.connectionHint')}
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
            <AlertDialogTitle>{t('tunnel.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('tunnel.deleteDesc', { name: node.name })}
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
