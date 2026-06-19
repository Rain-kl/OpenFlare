'use client';

import Link from 'next/link';

import {EmptyStateWithBorder} from '@/components/layout/empty';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardDescription, CardHeader, CardTitle,} from '@/components/ui/card';
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from '@/components/ui/table';
import type {DashboardNodeHealth} from '@/lib/services/openflare';

import {NodeStatusBadge} from '../../nodes/components/node-status-badge';
import {
  formatRelativeTime,
  getNodeStatusLabel,
  getNodeStatusTone,
  getOpenrestyStatusLabel,
  getOpenrestyStatusTone,
  isWSConnectedLastSeen,
} from '../../nodes/components/node-utils';
import {formatCompactNumber, formatPercent} from './dashboard-utils';

export function NodeHealthTable({ nodes }: { nodes: DashboardNodeHealth[] }) {
  const sortedNodes = [...nodes].sort((left, right) => {
    if (right.active_event_count !== left.active_event_count) {
      return right.active_event_count - left.active_event_count;
    }
    const leftPressure = Math.max(
      left.cpu_usage_percent,
      left.memory_usage_percent,
      left.storage_usage_percent,
    );
    const rightPressure = Math.max(
      right.cpu_usage_percent,
      right.memory_usage_percent,
      right.storage_usage_percent,
    );
    return rightPressure - leftPressure;
  });

  return (
    <Card className="border-dashed shadow-none">
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div>
          <CardTitle className="text-sm font-semibold">节点健康列表</CardTitle>
          <CardDescription className="text-xs">
            按异常数量和资源压力排序。
          </CardDescription>
        </div>
        <Button variant="outline" size="sm" className="h-8 text-xs" asChild>
          <Link href="/nodes">进入节点页</Link>
        </Button>
      </CardHeader>
      <CardContent>
        {sortedNodes.length === 0 ? (
          <EmptyStateWithBorder
            title="暂无节点"
            description="节点接入后，这里会展示系统健康与容量摘要。"
          />
        ) : (
          <div className="border border-dashed rounded-lg overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow className="border-dashed hover:bg-transparent">
                  <TableHead className="py-2 h-8">节点</TableHead>
                  <TableHead className="py-2 h-8">状态</TableHead>
                  <TableHead className="py-2 h-8">CPU / 内存</TableHead>
                  <TableHead className="py-2 h-8">请求 / 错误</TableHead>
                  <TableHead className="py-2 h-8">活动异常</TableHead>
                  <TableHead className="py-2 h-8">最近心跳</TableHead>
                  <TableHead className="py-2 h-8 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedNodes.slice(0, 12).map((node) => (
                  <TableRow key={node.node_id} className="border-dashed align-top">
                    <TableCell className="py-3">
                      <div className="space-y-1">
                        <p className="font-medium">{node.name}</p>
                        <p className="text-xs text-muted-foreground">{node.node_id}</p>
                        <p className="text-xs text-muted-foreground">
                          {node.geo_name || '未配置地图点位'}
                        </p>
                      </div>
                    </TableCell>
                    <TableCell className="py-3">
                      <div className="flex flex-wrap gap-1.5">
                        <NodeStatusBadge
                          label={getNodeStatusLabel(node.status)}
                          tone={getNodeStatusTone(node.status)}
                        />
                        <NodeStatusBadge
                          label={getOpenrestyStatusLabel(node.openresty_status)}
                          tone={getOpenrestyStatusTone(node.openresty_status)}
                        />
                      </div>
                    </TableCell>
                    <TableCell className="py-3 text-sm text-muted-foreground">
                      {formatPercent(node.cpu_usage_percent)} /{' '}
                      {formatPercent(node.memory_usage_percent)}
                    </TableCell>
                    <TableCell className="py-3 text-sm text-muted-foreground">
                      {formatCompactNumber(node.request_count)} /{' '}
                      {formatCompactNumber(node.error_count)}
                    </TableCell>
                    <TableCell className="py-3">
                      <NodeStatusBadge
                        label={String(node.active_event_count)}
                        tone={node.active_event_count > 0 ? 'danger' : 'success'}
                      />
                    </TableCell>
                    <TableCell className="py-3 text-xs text-muted-foreground">
                      {isWSConnectedLastSeen(node.last_seen_at)
                        ? 'WS 已连接'
                        : node.last_seen_at
                          ? formatRelativeTime(node.last_seen_at)
                          : '暂无'}
                    </TableCell>
                    <TableCell className="py-3 text-right">
                      <Button variant="ghost" size="sm" className="h-8 text-xs" asChild>
                        <Link href={`/nodes/detail?id=${node.id}`}>详情</Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
