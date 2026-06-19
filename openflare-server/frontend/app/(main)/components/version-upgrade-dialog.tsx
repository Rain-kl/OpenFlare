'use client';

import {useEffect} from 'react';
import {ExternalLink, Loader2} from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import {Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle} from '@/components/ui/dialog';
import {useOpenFlareServerUpgrade} from '@/lib/hooks/use-openflare-server-upgrade';
import type {AppUpdateStatus} from '@/lib/services/admin/types';
import {formatDateTime} from '@/lib/utils';
import {formatRelativeTime} from '@/app/(main)/nodes/components/node-utils';

function getUpgradeBadge(update: AppUpdateStatus | null | undefined) {
  if (!update) {
    return { label: '未检查', variant: 'outline' as const };
  }
  if (update.update_available) {
    return { label: '可升级', variant: 'secondary' as const };
  }
  return { label: '最新', variant: 'default' as const };
}

export function VersionUpgradeDialog({
  open,
  onOpenChange,
  canUpgrade = true,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  canUpgrade?: boolean;
}) {
  const {
    currentVersion,
    update,
    releaseErrorMessage,
    isInitialLoading,
    isChecking,
    isUpgrading,
    handleOpen,
    handleCheckRelease,
    handleUpgrade,
  } = useOpenFlareServerUpgrade({ open, canUpgrade });

  useEffect(() => {
    if (open) {
      handleOpen();
    }
  }, [open, handleOpen]);

  const upgradeBadge = getUpgradeBadge(update);
  const isBusy = isChecking || isUpgrading;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>服务端版本</DialogTitle>
          <DialogDescription>
            检查上游 GitHub Release 并升级当前服务。升级开始后服务会短暂重启。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card className="border-dashed shadow-none py-4 gap-3">
              <CardHeader className="px-4 pb-0">
                <CardTitle className="text-sm">当前版本</CardTitle>
              </CardHeader>
              <CardContent className="px-4">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="text-sm font-medium">{currentVersion}</p>
                  <Badge variant={upgradeBadge.variant}>{upgradeBadge.label}</Badge>
                </div>
              </CardContent>
            </Card>

            <Card className="border-dashed shadow-none py-4 gap-3">
              <CardHeader className="px-4 pb-0">
                <CardTitle className="text-sm">最新版本</CardTitle>
              </CardHeader>
              <CardContent className="px-4 space-y-3">
                <p className="text-sm font-medium">{update?.latest_version || '未检查'}</p>
                {canUpgrade ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={isBusy}
                    onClick={handleCheckRelease}
                  >
                    {isChecking ? '检查中...' : '检查更新'}
                  </Button>
                ) : null}
              </CardContent>
            </Card>
          </div>

          {isInitialLoading ? (
            <div className="flex items-center justify-center py-8 text-sm text-muted-foreground">
              <Loader2 className="size-4 mr-2 animate-spin" />
              加载版本信息...
            </div>
          ) : null}

          {!isInitialLoading && releaseErrorMessage ? (
            <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
              {releaseErrorMessage}
            </div>
          ) : null}

          {!isInitialLoading && !releaseErrorMessage && !update ? (
            <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
              尚未检查更新，点击「检查更新」后展示 GitHub Release 信息。
            </div>
          ) : null}

          {update ? (
            <Card className="border-dashed shadow-none py-4 gap-3">
              <CardHeader className="px-4 pb-0">
                <CardTitle className="text-sm">GitHub Release · {update.latest_version}</CardTitle>
                <CardDescription>
                  {update.published_at
                    ? `发布时间：${formatRelativeTime(update.published_at)} · ${formatDateTime(update.published_at)}`
                    : '未提供发布时间'}
                </CardDescription>
              </CardHeader>
              <CardContent className="px-4 space-y-4">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={update.update_available ? 'secondary' : 'default'}>
                    {update.update_available ? '发现新版本' : '已经是最新版本'}
                  </Badge>
                  {update.prerelease ? (
                    <Badge variant="secondary">Preview 发布</Badge>
                  ) : (
                    <Badge variant="outline">正式发布</Badge>
                  )}
                  {!update.can_upgrade ? (
                    <Badge variant="destructive">当前平台不支持自动升级</Badge>
                  ) : null}
                </div>

                <div className="prose prose-sm dark:prose-invert max-w-none text-sm">
                  <ReactMarkdown remarkPlugins={[remarkGfm]}>
                    {update.release_notes || '暂无更新说明'}
                  </ReactMarkdown>
                </div>

                {update.release_url ? (
                  <a
                    href={update.release_url}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center text-sm text-primary hover:underline"
                  >
                    查看发布详情
                    <ExternalLink className="size-3 ml-1" />
                  </a>
                ) : null}

                {canUpgrade ? (
                  <div className="flex justify-end">
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          type="button"
                          disabled={
                            !update.update_available ||
                            isUpgrading ||
                            !update.can_upgrade ||
                            isBusy
                          }
                        >
                          {isUpgrading ? '升级中...' : '立即升级'}
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>升级到 {update.latest_version}？</AlertDialogTitle>
                          <AlertDialogDescription>
                            服务将下载并校验 {update.asset_name}，随后替换当前二进制并重启。请确保安装目录可写，且服务允许原地重启。
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>取消</AlertDialogCancel>
                          <AlertDialogAction onClick={handleUpgrade}>确认升级</AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                ) : null}

                {!update.can_upgrade ? (
                  <p className="text-sm text-muted-foreground">
                    {update.current_version === 'dev'
                      ? '开发构建没有可比较的 Release 版本，不能执行自动升级。'
                      : update.update_available
                        ? '当前平台暂不支持自动替换二进制，请从 Release 页面手动升级。'
                        : '当前版本无需升级。'}
                  </p>
                ) : null}
              </CardContent>
            </Card>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}