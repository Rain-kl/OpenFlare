'use client';

import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {useState} from 'react';
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
import {toast} from 'sonner';

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
import {Button} from '@/components/ui/button';
import {formatDateTime} from '@/lib/utils';
import type {NodeAgentReleaseInfo, NodeItem, ReleaseChannel} from '@/lib/services/openflare';
import {NodeService} from '@/lib/services/openflare';

import {AgentUpdateDialog} from './agent-update-dialog';
import {InstallCommand} from './install-command';
import {NodeDetailShell} from './node-detail-shell';
import {
  NodeErrorBanner,
  NodeInfoRow,
  NodeSectionCard,
} from './node-detail-primitives';
import {NodeEditorDialog} from './node-editor-dialog';
import {NodeObservability} from './node-observability';
import {NodeStatusBadge} from './node-status-badge';
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
  const router = useRouter();
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const saveMutation = useMutation({
    mutationFn: (payload: Parameters<typeof NodeService.updateNode>[1]) =>
      NodeService.updateNode(node.id, payload),
    onSuccess: async () => {
      toast.success('隧道节点已更新');
      setEditorOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const forceSyncMutation = useMutation({
    mutationFn: () => NodeService.requestForceSync(node.id),
    onSuccess: async (updated) => {
      toast.success(`已向隧道节点 ${updated.name} 下发强制同步指令`);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
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
        tag_name: release?.channel === 'preview' ? release.tag_name || undefined : undefined,
      }),
    onSuccess: async (updated) => {
      toast.success(
        `已向隧道节点 ${updated.name} 下发${updated.update_channel === 'preview' ? '预览版' : '正式版'}升级指令`,
      );
      setUpgradeOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: () => NodeService.deleteNode(node.id),
    onSuccess: async () => {
      toast.success('隧道节点已删除');
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
      router.push('/nodes');
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const handleRefresh = () => {
    void Promise.all([
      queryClient.invalidateQueries({ queryKey: nodesQueryKey }),
      queryClient.invalidateQueries({ queryKey: ['openflare', 'node-observability', node.id] }),
    ]);
  };

  const headerActions = (
    <>
      <Button variant="outline" size="sm" className="h-8" onClick={() => setEditorOpen(true)}>
        编辑
      </Button>
      <Button variant="outline" size="sm" className="h-8" onClick={handleRefresh}>
        <RefreshCw className="size-3.5 mr-1.5" />
        刷新
      </Button>
      <Button
        variant="outline"
        size="sm"
        className="h-8"
        disabled={forceSyncMutation.isPending}
        onClick={() => forceSyncMutation.mutate()}
      >
        <RotateCcw className="size-3.5 mr-1.5" />
        {forceSyncMutation.isPending ? '同步中...' : '强制同步'}
      </Button>
      <Button variant="secondary" size="sm" className="h-8" onClick={() => setUpgradeOpen(true)}>
        <Upload className="size-3.5 mr-1.5" />
        {node.update_requested ? '查看升级' : '升级 openflared'}
      </Button>
      <Button
        variant="outline"
        size="sm"
        className="h-8 text-destructive hover:text-destructive"
        onClick={() => setDeleteOpen(true)}
      >
        <Trash2 className="size-3.5 mr-1.5" />
        删除
      </Button>
    </>
  );

  const overviewTab = (
    <div className="space-y-6">
      {node.last_error ? <NodeErrorBanner message={node.last_error} /> : null}

      <div className="grid gap-6 xl:grid-cols-2">
        <NodeSectionCard title="隧道运行状态" description="openflared 连接与在线情况">
          <div className="divide-y">
            <NodeInfoRow label="运行状态">
              <NodeStatusBadge
                label={getNodeStatusLabel(node.status)}
                tone={getNodeStatusTone(node.status)}
              />
            </NodeInfoRow>
            <NodeInfoRow label="flared 状态">
              <NodeStatusBadge
                label={getFlaredStatusLabel(node)}
                tone={getFlaredStatusTone(node)}
              />
            </NodeInfoRow>
            <NodeInfoRow label="最近心跳">
              {isWSConnectedLastSeen(node.last_seen_at)
                ? 'WebSocket 长连接已建立'
                : isMeaningfulTime(node.last_seen_at)
                  ? `${formatRelativeTime(node.last_seen_at)} · ${formatDateTime(node.last_seen_at)}`
                  : '暂无'}
            </NodeInfoRow>
            <NodeInfoRow label="IP 地址">
              {node.ip || '—'}
              {node.ip_manual_override ? '（已锁定）' : ''}
            </NodeInfoRow>
            <NodeInfoRow label="自动更新">{node.auto_update_enabled ? '已启用' : '手动'}</NodeInfoRow>
          </div>
        </NodeSectionCard>

        <NodeSectionCard
          title="配置同步"
          description="隧道客户端配置追平状态"
          action={
            <Button variant="outline" size="sm" className="h-8" asChild>
              <Link href={`/apply-logs?node_id=${encodeURIComponent(node.node_id)}`}>
                <FileText className="size-3.5 mr-1.5" />
                应用记录
              </Link>
            </Button>
          }
        >
          <div className="divide-y">
            <NodeInfoRow label="当前配置版本">{node.current_version || '未应用'}</NodeInfoRow>
            <NodeInfoRow label="最近应用">
              <NodeStatusBadge
                label={getApplyLabel(node.latest_apply_result)}
                tone={getApplyTone(node.latest_apply_result)}
              />
            </NodeInfoRow>
            <NodeInfoRow label="最近应用时间">
              {isMeaningfulTime(node.latest_apply_at)
                ? `${formatRelativeTime(node.latest_apply_at)} · ${formatDateTime(node.latest_apply_at)}`
                : '暂无'}
            </NodeInfoRow>
            {node.latest_apply_checksum ? (
              <NodeInfoRow label="同步文件数">{node.latest_support_file_count} 个</NodeInfoRow>
            ) : null}
            <NodeInfoRow label="升级状态">
              <NodeStatusBadge
                label={
                  node.update_requested
                    ? node.update_channel === 'preview'
                      ? '等待预览更新'
                      : '等待更新'
                    : '未下发'
                }
                tone={node.update_requested ? 'warning' : 'info'}
              />
            </NodeInfoRow>
          </div>
        </NodeSectionCard>
      </div>
    </div>
  );

  const manageTab = (
    <div className="space-y-6">
      <NodeSectionCard title="接入凭证" description="隧道客户端接入所需 Token">
        <div className="divide-y">
          <NodeInfoRow label="Tunnel Token">
            <span className="font-mono text-xs break-all">{node.access_token || '暂无'}</span>
          </NodeInfoRow>
          <NodeInfoRow label="节点 ID">
            <span className="font-mono text-xs break-all">{node.node_id}</span>
          </NodeInfoRow>
          <NodeInfoRow label="创建时间">{formatDateTime(node.created_at)}</NodeInfoRow>
          <NodeInfoRow label="更新时间">{formatDateTime(node.updated_at)}</NodeInfoRow>
        </div>
      </NodeSectionCard>

      <InstallCommand node={node} variant="tunnel" />
    </div>
  );

  return (
    <>
      <NodeDetailShell
        title={node.name}
        typeLabel="Tunnel"
        typeTone="info"
        statusBadges={[
          { label: getNodeStatusLabel(node.status), tone: getNodeStatusTone(node.status) },
          { label: getFlaredStatusLabel(node), tone: getFlaredStatusTone(node) },
        ]}
        actions={headerActions}
        kpis={[
          { label: '节点 ID', value: node.node_id, icon: Fingerprint },
          { label: 'openflared', value: node.version || 'unknown', icon: Package },
          { label: '当前配置', value: node.current_version || '未应用', icon: KeyRound },
          {
            label: '最近心跳',
            value: isWSConnectedLastSeen(node.last_seen_at)
              ? 'WS 已连接'
              : isMeaningfulTime(node.last_seen_at)
                ? formatRelativeTime(node.last_seen_at)
                : '暂无',
            icon: Activity,
          },
        ]}
        overview={overviewTab}
        dashboard={
          <NodeObservability
            nodeId={node.id}
            variant="compact"
            connectionHint="隧道并发连接数"
          />
        }
        manage={manageTab}
        defaultTab="overview"
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
            <AlertDialogTitle>确认删除隧道节点</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除隧道节点「{node.name}」吗？删除后该节点需要重新创建并重新接入。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-white hover:bg-destructive/90"
              disabled={deleteMutation.isPending}
              onClick={() => deleteMutation.mutate()}
            >
              {deleteMutation.isPending ? '删除中...' : '确认删除'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}