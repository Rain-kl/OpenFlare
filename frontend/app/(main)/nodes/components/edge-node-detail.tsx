'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import {
  Activity,
  BarChart3,
  Cpu,
  FileText,
  Fingerprint,
  Gauge,
  Globe,
  Package,
  RefreshCw,
  RotateCcw,
  ShieldAlert,
  ShieldCheck,
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
import { getFormattingLocale } from '@/i18n/format';
import { formatDateTime } from '@/lib/utils';
import type { NodeItem } from '@/lib/services/openflare';
import { NodeService } from '@/lib/services/openflare';

import { AgentUpdateDialog } from './agent-update-dialog';
import { InstallCommand } from './install-command';
import {
  NodeQuarantineDialog,
  NodeSecuritySection,
} from './node-security-section';
import { type NodeDetailKpi, NodeDetailShell } from './node-detail-shell';
import {
  NodeErrorBanner,
  NodeInfoRow,
  NodeKpiCard,
  NodeSectionCard,
} from './node-detail-primitives';
import { NodeEditorDialog } from './node-editor-dialog';
import {
  NodeObservability,
  useNodeObservabilityQuery,
} from './node-observability';
import { NodeStatusBadge } from './node-status-badge';
import {
  formatMetricCount,
  formatPercent,
  formatRelativeTime,
  formatUsageRatio,
  getApplyLabel,
  getApplyTone,
  getErrorMessage,
  getHealthEventLabel,
  getNodeStatusLabel,
  getNodeStatusTone,
  getOpenrestyStatusLabel,
  getOpenrestyStatusTone,
  isMeaningfulTime,
} from './node-utils';

const nodesQueryKey = ['openflare', 'nodes'];

export function EdgeNodeDetail({ node }: { node: NodeItem }) {
  const t = useTranslations('nodes');
  const rt = useTranslations('runtime');
  const tc = useTranslations('common');
  const router = useRouter();
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [quarantineOpen, setQuarantineOpen] = useState(false);
  const [unquarantineOpen, setUnquarantineOpen] = useState(false);
  const isQuarantined = node.status === 'quarantined';
  const observabilityQuery = useNodeObservabilityQuery(node.id);
  const observability = observabilityQuery.data ?? null;
  const activeHealthEvents =
    observability?.health_events.filter((event) => event.status === 'active') ??
    [];
  const latestHealthEvent = activeHealthEvents[0] ?? null;
  const trafficSummary = observability?.analytics?.traffic ?? null;
  const healthSummary = observability?.analytics?.health ?? null;
  const topSourceCountry =
    observability?.analytics?.distributions?.source_countries?.[0] ?? null;
  const latestMetric =
    observability?.metric_snapshots && observability.metric_snapshots.length > 0
      ? observability.metric_snapshots[
          observability.metric_snapshots.length - 1
        ]
      : null;
  const storageUsageRatio = formatUsageRatio(
    latestMetric?.storage_used_bytes,
    latestMetric?.storage_total_bytes,
  );
  const diagnosticPlaceholderHint = observabilityQuery.isLoading
    ? t('obs.loading')
    : t('requestFailed');
  const diagnosticReady =
    !observabilityQuery.isLoading && !observabilityQuery.isError;
  const diagnosticKpis: NodeDetailKpi[] = [
    {
      label: t('obs.diag'),
      icon: Activity,
      value: !diagnosticReady
        ? '—'
        : activeHealthEvents.length
          ? t('obs.activeIssues', { count: activeHealthEvents.length })
          : t('obs.stable'),
      hint: !diagnosticReady
        ? diagnosticPlaceholderHint
        : latestHealthEvent
          ? getHealthEventLabel(latestHealthEvent)
          : t('obs.noActiveEvents'),
    },
    {
      label: t('obs.windowRequests'),
      icon: BarChart3,
      value: !diagnosticReady
        ? '—'
        : formatMetricCount(trafficSummary?.request_count),
      hint: !diagnosticReady
        ? diagnosticPlaceholderHint
        : trafficSummary
          ? t('obs.windowVisitorsHint', {
              visitors: formatMetricCount(trafficSummary.unique_visitor_count),
              rate: trafficSummary.error_rate_percent.toFixed(1),
            })
          : t('obs.noWindowSummary'),
    },
    {
      label: t('obs.capacityPressure'),
      icon: Gauge,
      value: !diagnosticReady
        ? '—'
        : healthSummary?.has_capacity_risk
          ? t('obs.needsAttention')
          : t('obs.normalRange'),
      hint: !diagnosticReady
        ? diagnosticPlaceholderHint
        : latestMetric
          ? t('obs.cpuStorage', {
              cpu: formatPercent(latestMetric.cpu_usage_percent),
              storage: formatPercent(storageUsageRatio),
            })
          : t('obs.noResourceSnapshot'),
    },
    {
      label: t('obs.sourceSignal'),
      icon: Globe,
      value: !diagnosticReady ? '—' : (topSourceCountry?.key ?? '—'),
      hint: !diagnosticReady
        ? diagnosticPlaceholderHint
        : topSourceCountry
          ? t('obs.requestTimes', {
              count: topSourceCountry.value.toLocaleString(
                getFormattingLocale(),
              ),
            })
          : t('obs.noSourceDist'),
    },
  ];

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
    mutationFn: () => NodeService.requestEdgeRestart(node.id),
    onSuccess: async (updated) => {
      toast.success(t('edge.toastRestart', { name: updated.name }));
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const upgradeMutation = useMutation({
    mutationFn: (
      payload: Parameters<typeof NodeService.requestAgentUpdate>[1],
    ) => NodeService.requestAgentUpdate(node.id, payload),
    onSuccess: async (updated) => {
      toast.success(t('edge.toastUpgrade', { tag: updated.update_tag }));
      setUpgradeOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error, t('requestFailed'))),
  });

  const unquarantineMutation = useMutation({
    mutationFn: () => NodeService.unquarantine(node.id),
    onSuccess: async () => {
      toast.success(t('security.unquarantineSuccess'));
      setUnquarantineOpen(false);
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
      queryClient.invalidateQueries({
        queryKey: ['openflare', 'node-load-metrics', node.id],
      }),
      queryClient.invalidateQueries({
        queryKey: ['openflare', 'node-ping-records', node.id],
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
        disabled={forceSyncMutation.isPending || isQuarantined}
        onClick={() => forceSyncMutation.mutate()}
      >
        <RotateCcw className='size-3.5 mr-1.5' />
        {forceSyncMutation.isPending ? t('syncing') : t('forceSync')}
      </Button>
      <Button
        variant='outline'
        size='sm'
        className='h-8'
        disabled={isQuarantined}
        onClick={() => setUpgradeOpen(true)}
      >
        <Upload data-icon='inline-start' />
        {t('edge.upgrade')}
      </Button>
      <Button
        variant='outline'
        size='sm'
        className='h-8'
        disabled={restartMutation.isPending || isQuarantined}
        onClick={() => restartMutation.mutate()}
      >
        {restartMutation.isPending ? t('dispatching') : t('edge.restartEdge')}
      </Button>
      {isQuarantined ? (
        <Button
          variant='outline'
          size='sm'
          className='h-8'
          disabled={unquarantineMutation.isPending}
          onClick={() => setUnquarantineOpen(true)}
        >
          <ShieldCheck className='size-3.5 mr-1.5' />
          {t('security.unquarantineAction')}
        </Button>
      ) : (
        <Button
          variant='outline'
          size='sm'
          className='h-8 text-destructive hover:text-destructive'
          onClick={() => setQuarantineOpen(true)}
        >
          <ShieldAlert className='size-3.5 mr-1.5' />
          {t('security.quarantineAction')}
        </Button>
      )}
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

      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <NodeKpiCard
          label={t('kpiNodeId')}
          value={node.node_id}
          icon={Fingerprint}
        />
        <NodeKpiCard
          label={t('edge.kpiAgent')}
          value={node.version || 'unknown'}
          icon={Cpu}
        />
        <NodeKpiCard
          label={t('kpiCurrentConfig')}
          value={node.current_version || t('notApplied')}
          icon={Package}
        />
        <NodeKpiCard
          label={t('kpiLastSeen')}
          value={
            isMeaningfulTime(node.last_seen_at)
              ? formatRelativeTime(node.last_seen_at, t)
              : t('na')
          }
          icon={Activity}
        />
      </div>

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
            {node.update_requested && node.update_tag ? (
              <NodeInfoRow label={t('update.targetVersion')}>
                <NodeStatusBadge label={node.update_tag} tone='warning' />
              </NodeInfoRow>
            ) : null}
          </div>
        </NodeSectionCard>
      </div>

      {node.status === 'quarantined' ? (
        <div className='rounded-md border border-dashed p-6 text-sm text-muted-foreground'>
          {t('security.quarantinedNoInstall')}
        </div>
      ) : (
        <InstallCommand node={node} variant='edge' />
      )}
      <NodeSecuritySection
        node={node}
        onNodeUpdated={() =>
          void queryClient.invalidateQueries({ queryKey: nodesQueryKey })
        }
      />

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
        typeLabel={rt('node_type_edge')}
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
        kpis={diagnosticKpis}
        overview={
          <NodeObservability
            nodeId={node.id}
            node={node}
            variant='edge'
            connectionHint={t('edge.connectionHint')}
            showSummaryStats={false}
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
        onConfirm={(release, channel) => {
          if (release?.tag_name) {
            upgradeMutation.mutate({
              channel,
              tag_name: release.tag_name,
            });
          }
        }}
      />

      <NodeQuarantineDialog
        node={node}
        open={quarantineOpen}
        onOpenChange={setQuarantineOpen}
        onQuarantined={() =>
          void queryClient.invalidateQueries({ queryKey: nodesQueryKey })
        }
      />

      <AlertDialog open={unquarantineOpen} onOpenChange={setUnquarantineOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('security.unquarantineTitle')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t('security.unquarantineDesc', { name: node.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={unquarantineMutation.isPending}>
              {tc('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={unquarantineMutation.isPending}
              onClick={() => unquarantineMutation.mutate()}
            >
              {t('security.unquarantineConfirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

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
