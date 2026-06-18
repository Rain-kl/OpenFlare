'use client';

import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {useState} from 'react';
import {
  Activity,
  ExternalLink,
  FileText,
  Fingerprint,
  Network,
  RefreshCw,
  RotateCcw,
  Server,
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
import {Label} from '@/components/ui/label';
import {Switch} from '@/components/ui/switch';
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
  getNodeStatusLabel,
  getNodeStatusTone,
  getRelayStatusLabel,
  getRelayStatusTone,
  isMeaningfulTime,
  isWSConnectedLastSeen,
} from './node-utils';

const nodesQueryKey = ['openflare', 'nodes'];

export function RelayNodeDetail({ node }: { node: NodeItem }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const saveMutation = useMutation({
    mutationFn: (payload: Parameters<typeof NodeService.updateNode>[1]) =>
      NodeService.updateNode(node.id, payload),
    onSuccess: async () => {
      toast.success('中继节点已更新');
      setEditorOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
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
      toast.success('FRPS WebUI 设置已更新');
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const forceSyncMutation = useMutation({
    mutationFn: () => NodeService.requestForceSync(node.id),
    onSuccess: async (updated) => {
      toast.success(`已向中继节点 ${updated.name} 下发强制同步指令`);
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
        `已向中继节点 ${updated.name} 下发${updated.update_channel === 'preview' ? '预览版' : '正式版'}升级指令`,
      );
      setUpgradeOpen(false);
      await queryClient.invalidateQueries({ queryKey: nodesQueryKey });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: () => NodeService.deleteNode(node.id),
    onSuccess: async () => {
      toast.success('中继节点已删除');
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

  const webUiUrl =
    node.relay_web_server_enabled && node.relay_bind_port
      ? `http://${node.ip || '127.0.0.1'}:${node.relay_bind_port + 500}`
      : null;

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
        {node.update_requested ? '查看升级' : '升级 Relay'}
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
        <NodeSectionCard title="中继运行状态" description="frps 在线情况与负载摘要">
          <div className="divide-y">
            <NodeInfoRow label="运行状态">
              <NodeStatusBadge
                label={getNodeStatusLabel(node.status)}
                tone={getNodeStatusTone(node.status)}
              />
            </NodeInfoRow>
            <NodeInfoRow label="中继健康">
              <NodeStatusBadge
                label={getRelayStatusLabel(node.relay_status)}
                tone={getRelayStatusTone(node.relay_status)}
              />
            </NodeInfoRow>
            <NodeInfoRow label="最近心跳">
              {isWSConnectedLastSeen(node.last_seen_at)
                ? 'WS 已连接'
                : isMeaningfulTime(node.last_seen_at)
                  ? `${formatRelativeTime(node.last_seen_at)} · ${formatDateTime(node.last_seen_at)}`
                  : '暂无'}
            </NodeInfoRow>
            <NodeInfoRow label="活动连接 / 代理数">
              {node.relay_frps_connections ?? '—'} / {node.relay_frps_proxy_count ?? '—'}
            </NodeInfoRow>
            <NodeInfoRow label="IP 地址">
              {node.ip || '—'}
              {node.ip_manual_override ? '（已锁定）' : ''}
            </NodeInfoRow>
          </div>
        </NodeSectionCard>

        <NodeSectionCard
          title="网络端口"
          description="frps 控制面与 HTTP 虚拟主机配置"
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
            <NodeInfoRow label="绑定控制端口">{node.relay_bind_port || '—'}</NodeInfoRow>
            <NodeInfoRow label="VHost HTTP 端口">{node.relay_vhost_http_port || '—'}</NodeInfoRow>
            <NodeInfoRow label="Agent 接入地址">
              <span className="break-all">{node.relay_agent_access_addr || '—'}</span>
            </NodeInfoRow>
            <NodeInfoRow label="最近应用">
              <NodeStatusBadge
                label={getApplyLabel(node.latest_apply_result)}
                tone={getApplyTone(node.latest_apply_result)}
              />
            </NodeInfoRow>
            <NodeInfoRow label="Relay 版本">{node.version || 'unknown'}</NodeInfoRow>
            <NodeInfoRow label="frps 核心版本">{node.ext_version || 'unknown'}</NodeInfoRow>
          </div>
        </NodeSectionCard>
      </div>
    </div>
  );

  const manageTab = (
    <div className="space-y-6">
      <NodeSectionCard title="FRPS WebUI" description="控制 frps 内置 Web 管理界面是否启用">
        <div className="space-y-4">
          <div className="flex items-center justify-between rounded-xl border px-4 py-4">
            <div className="space-y-1">
              <Label>启用 Web 管理界面</Label>
              <p className="text-xs text-muted-foreground">
                默认监听绑定端口 + 500，例如 {node.relay_bind_port || 7000} →{' '}
                {(node.relay_bind_port || 7000) + 500}
              </p>
            </div>
            <Switch
              checked={node.relay_web_server_enabled}
              disabled={webServerMutation.isPending}
              onCheckedChange={(checked) => webServerMutation.mutate(checked)}
            />
          </div>

          {webUiUrl ? (
            <a
              href={webUiUrl}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center text-sm font-medium text-primary hover:underline"
            >
              打开 FRPS WebUI
              <ExternalLink className="size-3.5 ml-1.5" />
            </a>
          ) : (
            <p className="text-sm text-muted-foreground">WebUI 已禁用或未配置绑定端口。</p>
          )}
        </div>
      </NodeSectionCard>

      <InstallCommand node={node} variant="relay" />

      <NodeSectionCard title="节点元数据">
        <div className="divide-y">
          <NodeInfoRow label="节点 ID">
            <span className="font-mono text-xs break-all">{node.node_id}</span>
          </NodeInfoRow>
          <NodeInfoRow label="创建时间">{formatDateTime(node.created_at)}</NodeInfoRow>
          <NodeInfoRow label="更新时间">{formatDateTime(node.updated_at)}</NodeInfoRow>
        </div>
      </NodeSectionCard>
    </div>
  );

  return (
    <>
      <NodeDetailShell
        title={node.name}
        typeLabel="Relay"
        typeTone="warning"
        statusBadges={[
          { label: getNodeStatusLabel(node.status), tone: getNodeStatusTone(node.status) },
          {
            label: getRelayStatusLabel(node.relay_status),
            tone: getRelayStatusTone(node.relay_status),
          },
        ]}
        actions={headerActions}
        kpis={[
          { label: '节点 ID', value: node.node_id, icon: Fingerprint },
          { label: 'Relay 版本', value: node.version || 'unknown', icon: Server },
          { label: 'frps 核心', value: node.ext_version || 'unknown', icon: Network },
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
            connectionHint="中继承载活动连接数"
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
            <AlertDialogTitle>确认删除中继节点</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除中继节点「{node.name}」吗？删除后该节点需要重新创建并重新接入。
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