'use client';

import {Loader2, Trash2} from 'lucide-react';
import {useMemo, useState} from 'react';

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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from '@/components/ui/table';
import type {WAFIPGroup} from '@/lib/services/openflare';
import {formatDateTime} from '@/lib/utils';

import {getIPGroupViewEntries, ipGroupTypeLabels, type IPGroupViewEntry} from './helpers';

interface IPGroupViewDialogProps {
  open: boolean;
  group: WAFIPGroup | null;
  loading: boolean;
  removingIp: string | null;
  onOpenChange: (open: boolean) => void;
  onRemoveIp: (ip: string) => Promise<void>;
}

export function IPGroupViewDialog({
  open,
  group,
  loading,
  removingIp,
  onOpenChange,
  onRemoveIp,
}: IPGroupViewDialogProps) {
  const [deleteTarget, setDeleteTarget] = useState<IPGroupViewEntry | null>(null);

  const entries = useMemo(
    () => (group ? getIPGroupViewEntries(group) : []),
    [group],
  );

  const showAutomaticMeta = group?.type === 'automatic';

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          onOpenChange(nextOpen);
          if (!nextOpen) {
            setDeleteTarget(null);
          }
        }}
      >
        <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{group ? `查看 ${group.name}` : '查看 IP 组'}</DialogTitle>
            <DialogDescription>
              {group
                ? `${ipGroupTypeLabels[group.type]} · 共 ${entries.length} 条 IP`
                : '查看当前 IP 组中的 IP 列表，并可移除不需要的条目。'}
            </DialogDescription>
          </DialogHeader>

          {loading ? (
            <div className="flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" />
              加载 IP 列表...
            </div>
          ) : !group ? (
            <p className="py-8 text-center text-sm text-muted-foreground">未选择 IP 组。</p>
          ) : entries.length === 0 ? (
            <EmptyStateWithBorder
              description={
                group.type === 'automatic'
                  ? '该自动规则暂未抓取任何 IP，可点击「立即执行」手动触发抓取。'
                  : '当前 IP 组暂无 IP 条目。'
              }
            />
          ) : (
            <div className="space-y-3">
              {group.type === 'subscription' ? (
                <p className="text-xs text-muted-foreground">
                  订阅类型在下次同步时可能重新拉取已删除的 IP。
                </p>
              ) : null}
              <div className="rounded-lg border border-dashed">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>IP 地址</TableHead>
                      {showAutomaticMeta ? (
                        <>
                          <TableHead>抓取时间</TableHead>
                          <TableHead>封禁剩余</TableHead>
                        </>
                      ) : null}
                      <TableHead className="w-[80px] text-right">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {entries.map((entry) => (
                      <TableRow key={entry.ip}>
                        <TableCell className="font-mono text-sm">{entry.ip}</TableCell>
                        {showAutomaticMeta ? (
                          <>
                            <TableCell className="text-sm text-muted-foreground">
                              {entry.capturedAt ? formatDateTime(entry.capturedAt) : '—'}
                            </TableCell>
                            <TableCell className="text-sm">{entry.banRemaining ?? '—'}</TableCell>
                          </>
                        ) : null}
                        <TableCell className="text-right">
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="size-8 text-destructive hover:text-destructive"
                            disabled={removingIp === entry.ip}
                            onClick={() => setDeleteTarget(entry)}
                          >
                            {removingIp === entry.ip ? (
                              <Loader2 className="size-4 animate-spin" />
                            ) : (
                              <Trash2 className="size-4" />
                            )}
                            <span className="sr-only">删除 {entry.ip}</span>
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              关闭
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(nextOpen) => !nextOpen && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除 IP</AlertDialogTitle>
            <AlertDialogDescription>
              确认从 IP 组「{group?.name}」中移除 {deleteTarget?.ip} 吗？
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={Boolean(removingIp)}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-white hover:bg-destructive/90"
              disabled={Boolean(removingIp)}
              onClick={async () => {
                if (!deleteTarget) return;
                await onRemoveIp(deleteTarget.ip);
                setDeleteTarget(null);
              }}
            >
              {removingIp ? '删除中...' : '确认删除'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}