'use client';

import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {ExternalLink, Loader2} from 'lucide-react';

import {Button} from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {formatDateTime} from '@/lib/utils';
import type {NodeAgentReleaseInfo, NodeItem, ReleaseChannel} from '@/lib/services/openflare';
import {NodeService} from '@/lib/services/openflare';

import {NodeStatusBadge} from './node-status-badge';
import {formatRelativeTime, getErrorMessage} from './node-utils';

export function AgentUpdateDialog({
  open,
  node,
  submitting,
  onClose,
  onConfirm,
}: {
  open: boolean;
  node: NodeItem;
  submitting: boolean;
  onClose: () => void;
  onConfirm: (release: NodeAgentReleaseInfo | null, channel: ReleaseChannel) => Promise<void>;
}) {
  const [channel, setChannel] = useState<ReleaseChannel>('stable');

  const releaseQuery = useQuery({
    queryKey: ['openflare', 'node-agent-release', node.id, channel],
    queryFn: () => NodeService.getAgentRelease(node.id, channel),
    enabled: open,
  });

  const release = releaseQuery.data ?? null;

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Agent 升级</DialogTitle>
          <DialogDescription>
            检查 GitHub 发布并向节点下发升级指令，节点将在下一次心跳后执行。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <div className="rounded-lg border p-3">
              <p className="text-xs text-muted-foreground">当前版本</p>
              <p className="mt-1 text-sm font-medium">{node.version || 'unknown'}</p>
            </div>
            <div className="rounded-lg border p-3">
              <p className="text-xs text-muted-foreground">检查通道</p>
              <p className="mt-1 text-sm font-medium">
                {channel === 'preview' ? '预览版' : '正式版'}
              </p>
            </div>
            <div className="rounded-lg border p-3">
              <p className="text-xs text-muted-foreground">更新状态</p>
              <div className="mt-1">
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
              </div>
            </div>
          </div>

          {releaseQuery.isFetching ? (
            <div className="flex items-center justify-center py-8 text-sm text-muted-foreground">
              <Loader2 className="size-4 mr-2 animate-spin" />
              检查发布版本中...
            </div>
          ) : releaseQuery.isError ? (
            <p className="text-sm text-destructive">{getErrorMessage(releaseQuery.error)}</p>
          ) : release ? (
            <div className="space-y-3 rounded-lg border p-4">
              <div className="flex flex-wrap items-center gap-2">
                <NodeStatusBadge
                  label={release.has_update ? '发现可升级版本' : '当前已是最新版本'}
                  tone={release.has_update ? 'warning' : 'success'}
                />
                {release.prerelease ? (
                  <NodeStatusBadge label="Preview 发布" tone="warning" />
                ) : (
                  <NodeStatusBadge label="正式发布" tone="info" />
                )}
              </div>
              <div className="grid gap-3 sm:grid-cols-2 text-sm">
                <div>
                  <p className="text-xs text-muted-foreground">目标版本</p>
                  <p className="mt-1 font-medium">{release.tag_name || '未找到'}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">发布时间</p>
                  <p className="mt-1">
                    {release.published_at
                      ? `${formatRelativeTime(release.published_at)} · ${formatDateTime(release.published_at)}`
                      : '—'}
                  </p>
                </div>
              </div>
              <p className="text-sm text-muted-foreground whitespace-pre-wrap">
                {release.body || '暂无更新说明'}
              </p>
              {release.html_url ? (
                <a
                  href={release.html_url}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center text-sm text-primary hover:underline"
                >
                  查看发布详情
                  <ExternalLink className="size-3 ml-1" />
                </a>
              ) : null}
            </div>
          ) : null}
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button
            type="button"
            variant="outline"
            disabled={submitting || releaseQuery.isFetching}
            onClick={() => setChannel('stable')}
          >
            检查正式版
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={submitting || releaseQuery.isFetching}
            onClick={() => setChannel('preview')}
          >
            检查预览版
          </Button>
          <Button
            type="button"
            disabled={
              submitting ||
              releaseQuery.isFetching ||
              !release?.has_update ||
              node.update_requested
            }
            onClick={() => void onConfirm(release, channel)}
          >
            {submitting ? '下发中...' : channel === 'preview' ? '升级到预览版' : '升级到正式版'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
