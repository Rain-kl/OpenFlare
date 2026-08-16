'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  Activity,
  Cpu,
  FileText,
  Fingerprint,
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
  getNodeStatusLabel,
  getNodeStatusTone,
  getOpenrestyStatusLabel,
  getOpenrestyStatusTone,
  isMeaningfulTime,
} from './node-utils';

const nodesQueryKey = ['openflare', 'nodes'];

export function EdgeNodeDetail({ node }: { node: NodeItem }) {
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
      toast.success(t('edge.toastUpdated'));
      setEditorOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const forceSyncMutation = useMutation({
    mutationFn: () => NodeService.requestForceSync(node.id),
    onSuccess: async (updated) => {
      toast.success(t('edge.toastForceSync', { name: updated.name }));
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const restartMutation = useMutation({
    mutationFn: () => NodeService.requestOpenrestyRestart(node.id),
    onSuccess: async (updated) => {
      toast.success(t('edge.toastRestart', { name: updated.name }));
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
        t('edge.toastUpgrade', {
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
      toast.success(t('edge.toastDeleted'));
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
        variant='outline'
        size='sm'
        className='h-8'
        disabled={restartMutation.isPending}
        onClick={() => restartMutation.mutate()}
      >
        {restartMutation.isPending
          ? t('dispatching')
          : t('edge.restartOpenresty')}
      </Button>
      <Button
        variant='secondary'
        size='sm'
        className='h-8'
        onClick={() => setUpgradeOpen(true)}
      >
        <Upload className='size-3.5 mr-1.5' />
        {node.update_requested ? t('viewUpgrade') : t('edge.upgrade')}
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
          title={t('edge.runStatus')}
          description={t('edge.runStatusDesc')}
        >
          <div className='divide-y'>
            <NodeInfoRow label={t('edge.runStatus')}>
              <NodeStatusBadge
                label={getNodeStatusLabel(node.status, t)}
                tone={getNodeStatusTone(node.status)}
              />
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.openrestyHealth')}>
              <NodeStatusBadge
                label={getOpenrestyStatusLabel(node.openresty_status, t)}
                tone={getOpenrestyStatusTone(node.openresty_status)}
              />
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.lastSeen')}>
              {isMeaningfulTime(node.last_seen_at)
                ? `${formatRelativeTime(node.last_seen_at, t)} · ${formatDateTime(node.last_seen_at)}`
                : t('na')}
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.ip')}>
              {node.ip || '—'}
              {node.ip_manual_override ? t('lockedSuffix') : ''}
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.geo')}>
              {node.geo_name || t('notConfigured')}
            </NodeInfoRow>
          </div>
        </NodeSectionCard>

        <NodeSectionCard
          title={t('edge.syncTitle')}
          description={t('edge.syncDesc')}
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
            <NodeInfoRow label={t('edge.currentVersion')}>
              {node.current_version || t('notApplied')}
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.latestApply')}>
              <NodeStatusBadge
                label={getApplyLabel(node.latest_apply_result, t)}
                tone={getApplyTone(node.latest_apply_result)}
              />
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.latestApplyAt')}>
              {isMeaningfulTime(node.latest_apply_at)
                ? `${formatRelativeTime(node.latest_apply_at, t)} · ${formatDateTime(node.latest_apply_at)}`
                : t('na')}
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.agentVersion')}>
              {node.version || 'unknown'}
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.nginxVersion')}>
              {node.ext_version || 'unknown'}
            </NodeInfoRow>
            <NodeInfoRow label={t('edge.autoUpdate')}>
              {node.auto_update_enabled ? t('enabled') : t('manual')}
            </NodeInfoRow>
          </div>
        </NodeSectionCard>
      </div>

      <InstallCommand node={node} variant='edge' />

      <NodeSectionCard
        title={t('edge.runtimeMessages')}
        description={t('edge.runtimeMessagesDesc')}
      >
        <div className='space-y-4 text-sm'>
          <div>
            <p className='mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground'>
              {t('edge.openrestyMessage')}
            </p>
            <p className='rounded-lg border bg-muted/30 px-3 py-3 leading-6 break-words whitespace-pre-wrap'>
              {node.openresty_message || t('edge.noExtraError')}
            </p>
          </div>
          {node.restart_openresty_requested ? (
            <div className='flex items-center gap-2 text-sm text-amber-700 dark:text-amber-300'>
              <Activity className='size-4' />
              {t('edge.restartPending')}
            </div>
          ) : null}
        </div>
      </NodeSectionCard>
    </div>
  );

  return (
    <>
      <NodeDetailShell
        title={node.name}
        typeLabel='Edge'
        typeTone='info'
        statusBadges={[
          {
            label: getNodeStatusLabel(node.status, t),
            tone: getNodeStatusTone(node.status),
          },
          {
            label: getOpenrestyStatusLabel(node.openresty_status, t),
            tone: getOpenrestyStatusTone(node.openresty_status),
          },
        ]}
        actions={headerActions}
        kpis={[
          { label: t('kpiNodeId'), value: node.node_id, icon: Fingerprint },
          {
            label: t('edge.kpiAgent'),
            value: node.version || 'unknown',
            icon: Cpu,
          },
          {
            label: t('kpiCurrentConfig'),
            value: node.current_version || t('notApplied'),
            icon: Package,
          },
          {
            label: t('kpiLastSeen'),
            value: isMeaningfulTime(node.last_seen_at)
              ? formatRelativeTime(node.last_seen_at, t)
              : t('na'),
            icon: Activity,
          },
        ]}
        overview={
          <NodeObservability
            nodeId={node.id}
            node={node}
            variant='edge'
            connectionHint={t('edge.connectionHint')}
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
            <AlertDialogTitle>{t('deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteDesc', { name: node.name })}
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
