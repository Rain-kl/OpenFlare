'use client';

import Link from 'next/link';
import {useCallback, useState} from 'react';
import {ArrowLeft, Loader2, Route, Save, Upload} from 'lucide-react';
import {toast} from 'sonner';

import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {DiffDialog} from '@/app/(main)/config-versions/components/diff-dialog';
import type {ConfigDiffResult, ProxyRouteConfigSection, ProxyRouteItem} from '@/lib/services/openflare';
import {ConfigVersionService} from '@/lib/services/openflare';

import {getErrorMessage} from '../../components/helpers';
import {submitProxyRouteSectionForm} from '../helpers';

function hasConfigDiff(diff: ConfigDiffResult) {
  return (
    diff.added_domains.length > 0 ||
    diff.removed_domains.length > 0 ||
    diff.modified_domains.length > 0 ||
    diff.main_config_changed ||
    diff.waf_config_changed ||
    diff.changed_option_keys.length > 0 ||
    !diff.active_version
  );
}

interface RouteHeaderProps {
  route: ProxyRouteItem;
  activeSection: ProxyRouteConfigSection;
  saving?: boolean;
}

export function RouteHeader({ route, activeSection, saving = false }: RouteHeaderProps) {
  const [diffOpen, setDiffOpen] = useState(false);
  const [publishConfirmOpen, setPublishConfirmOpen] = useState(false);
  const [diff, setDiff] = useState<ConfigDiffResult | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [diffError, setDiffError] = useState<string | null>(null);
  const [publishing, setPublishing] = useState(false);

  const handleSave = useCallback(() => {
    const submitted = submitProxyRouteSectionForm(activeSection);
    if (!submitted) {
      toast.error('当前分区无法保存');
    }
  }, [activeSection]);

  const loadDiff = useCallback(async () => {
    setDiffLoading(true);
    setDiffError(null);
    try {
      const diffData = await ConfigVersionService.diff();
      setDiff(diffData);
      return diffData;
    } catch (error) {
      const message = getErrorMessage(error);
      setDiffError(message);
      return null;
    } finally {
      setDiffLoading(false);
    }
  }, []);

  const handlePublishClick = useCallback(async () => {
    setDiffOpen(true);
    const diffData = diff ?? (await loadDiff());
    if (!diffData) {
      return;
    }
    if (!hasConfigDiff(diffData)) {
      toast.info('当前配置与激活版本一致，无需发布');
      return;
    }
    setPublishConfirmOpen(true);
  }, [diff, loadDiff]);

  const handlePublish = useCallback(async () => {
    setPublishing(true);
    try {
      const version = await ConfigVersionService.publish();
      toast.success('发布成功', { description: `版本 ${version.version}` });
      setPublishConfirmOpen(false);
      setDiffOpen(false);
      setDiff(null);
    } catch (error) {
      toast.error('发布失败', { description: getErrorMessage(error) });
    } finally {
      setPublishing(false);
    }
  }, []);

  return (
    <>
      <div className="space-y-4">
        <Button variant="ghost" size="sm" className="h-8 gap-1.5 px-0 text-xs" asChild>
          <Link href="/proxy-routes">
            <ArrowLeft className="size-3.5" />
            返回规则列表
          </Link>
        </Button>

        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-2">
            <Route className="size-5 text-primary" />
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">{route.site_name}</h1>
              <Badge variant={route.enabled ? 'default' : 'secondary'}>
                {route.enabled ? '已启用' : '已停用'}
              </Badge>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="secondary"
              className="h-8 gap-1.5 text-xs"
              disabled={saving}
              onClick={handleSave}
            >
              {saving ? <Loader2 className="size-3.5 animate-spin" /> : <Save className="size-3.5" />}
              {saving ? '保存中...' : '保存当前分区'}
            </Button>
            <Button
              size="sm"
              className="h-8 gap-1.5 text-xs"
              disabled={publishing || diffLoading}
              onClick={() => void handlePublishClick()}
            >
              {publishing || diffLoading ? (
                <Loader2 className="size-3.5 animate-spin" />
              ) : (
                <Upload className="size-3.5" />
              )}
              发布配置
            </Button>
          </div>
        </div>
      </div>

      <DiffDialog
        open={diffOpen}
        onOpenChange={(open) => {
          setDiffOpen(open);
          if (!open) {
            setPublishConfirmOpen(false);
          }
        }}
        diff={diff}
        loading={diffLoading}
        error={diffError}
      />

      <AlertDialog open={publishConfirmOpen} onOpenChange={setPublishConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认发布配置</AlertDialogTitle>
            <AlertDialogDescription>
              将把当前待发布配置生成新版本并设为激活版本，节点将随后拉取更新。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={publishing}>取消</AlertDialogCancel>
            <Button onClick={() => void handlePublish()} disabled={publishing}>
              {publishing ? <Loader2 className="size-4 animate-spin" /> : '确认发布'}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}