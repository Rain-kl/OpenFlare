'use client';

import Link from 'next/link';
import {Pencil, Trash2} from 'lucide-react';

import {Button} from '@/components/ui/button';
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from '@/components/ui/table';
import {formatDateTime} from '@/lib/utils';
import type {NodeItem} from '@/lib/services/openflare';

import {NodeStatusBadge} from './node-status-badge';
import {
  formatRelativeTime,
  getApplyLabel,
  getApplyTone,
  getNodeStatusLabel,
  getNodeStatusTone,
  getNodeTypeLabel,
  getOpenrestyStatusLabel,
  getOpenrestyStatusTone,
  getRelayStatusLabel,
  getRelayStatusTone,
  isMeaningfulTime,
  isWSConnectedLastSeen,
} from './node-utils';

export function NodesTable({
  nodes,
  deletingId,
  onEdit,
  onDelete,
}: {
  nodes: NodeItem[];
  deletingId: number | null;
  onEdit: (node: NodeItem) => void;
  onDelete: (node: NodeItem) => void;
}) {
  return (
    <div className="border border-dashed rounded-lg overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow className="border-dashed hover:bg-transparent">
            <TableHead className="py-2 h-8">节点</TableHead>
            <TableHead className="py-2 h-8">状态</TableHead>
            <TableHead className="py-2 h-8">Version</TableHead>
            <TableHead className="py-2 h-8">运行健康</TableHead>
            <TableHead className="py-2 h-8">当前版本</TableHead>
            <TableHead className="py-2 h-8">最近应用</TableHead>
            <TableHead className="py-2 h-8">最近心跳</TableHead>
            <TableHead className="py-2 h-8 text-right">操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {nodes.map((node) => (
            <TableRow key={node.id} className="border-dashed align-top">
              <TableCell className="py-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <p className="font-medium">{node.name}</p>
                    <NodeStatusBadge label={getNodeTypeLabel(node.node_type)} tone="info" />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    IP：{node.ip || '—'}
                    {node.ip_manual_override ? '（已锁定）' : ''}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    位置：{node.geo_name || '未配置地图点位'}
                  </p>
                </div>
              </TableCell>
              <TableCell className="py-3">
                <NodeStatusBadge
                  label={getNodeStatusLabel(node.status)}
                  tone={getNodeStatusTone(node.status)}
                />
              </TableCell>
              <TableCell className="py-3 text-muted-foreground">
                {node.version || 'unknown'}
              </TableCell>
              <TableCell className="py-3">
                {node.node_type === 'tunnel_relay' ? (
                  <NodeStatusBadge
                    label={getRelayStatusLabel(node.relay_status)}
                    tone={getRelayStatusTone(node.relay_status)}
                  />
                ) : node.node_type === 'tunnel_client' ? (
                  <NodeStatusBadge
                    label={node.status === 'online' ? '运行中' : '未知'}
                    tone={node.status === 'online' ? 'success' : 'warning'}
                  />
                ) : (
                  <NodeStatusBadge
                    label={getOpenrestyStatusLabel(node.openresty_status)}
                    tone={getOpenrestyStatusTone(node.openresty_status)}
                  />
                )}
              </TableCell>
              <TableCell className="py-3 text-muted-foreground">
                {node.current_version ||
                  (node.node_type === 'tunnel_relay' ? '实时配置' : '未应用')}
              </TableCell>
              <TableCell className="py-3">
                {node.node_type === 'tunnel_relay' ? (
                  <span className="text-sm text-muted-foreground">—</span>
                ) : (
                  <NodeStatusBadge
                    label={getApplyLabel(node.latest_apply_result)}
                    tone={getApplyTone(node.latest_apply_result)}
                  />
                )}
              </TableCell>
              <TableCell className="py-3 text-muted-foreground">
                {isWSConnectedLastSeen(node.last_seen_at)
                  ? 'WS 已连接'
                  : isMeaningfulTime(node.last_seen_at)
                    ? `${formatRelativeTime(node.last_seen_at)} · ${formatDateTime(node.last_seen_at)}`
                    : '暂无'}
              </TableCell>
              <TableCell className="py-3">
                <div className="flex flex-wrap justify-end gap-1">
                  <Button variant="outline" size="sm" className="h-7 text-xs" asChild>
                    <Link href={`/nodes/detail?id=${node.id}`}>详情</Link>
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs"
                    onClick={() => onEdit(node)}
                  >
                    <Pencil className="size-3 mr-1" />
                    编辑
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs text-destructive hover:text-destructive"
                    disabled={deletingId === node.id}
                    onClick={() => onDelete(node)}
                  >
                    <Trash2 className="size-3 mr-1" />
                    删除
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
