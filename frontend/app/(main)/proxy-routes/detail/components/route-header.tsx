'use client';

import { useRouter } from 'next/navigation';
import { useCallback, useState } from 'react';
import { ArrowLeft, Loader2, Route, Upload } from 'lucide-react';
import { toast } from 'sonner';

import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type {
  ConfigDiffResult,
  ProxyRouteItem,
} from '@/lib/services/openflare';
import { ConfigVersionService } from '@/lib/services/openflare';

import { getErrorMessage } from '../../components/helpers';

function hasConfigDiff(diff: ConfigDiffResult) {
  return (
    diff.added_domains.length > 0 ||
    diff.removed_domains.length > 0 ||
    diff.modified_domains.length > 0 ||
    diff.added_sites.length > 0 ||
    diff.removed_sites.length > 0 ||
    diff.modified_sites.length > 0 ||
    diff.main_config_changed ||
    diff.waf_config_changed ||
    diff.changed_option_keys.length > 0 ||
    !diff.active_version
  );
}

interface RouteHeaderProps {
  route: ProxyRouteItem;
}

export function RouteHeader({ route }: RouteHeaderProps) {
  const router = useRouter();
  const [publishConfirmOpen, setPublishConfirmOpen] = useState(false);
  const [diff, setDiff] = useState<ConfigDiffResult | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [publishing, setPublishing] = useState(false);

  const loadDiff = useCallback(async () => {
    setDiffLoading(true);
    try {
      const diffData = await ConfigVersionService.diff();
      setDiff(diffData);
      return diffData;
    } catch (error) {
      const message = getErrorMessage(error);
      toast.error('获取配置差异失败', { description: message });
      return null;
    } finally {
      setDiffLoading(false);
    }
  }, []);

  const handlePublishClick = useCallback(async () => {
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
      setDiff(null);
    } catch (error) {
      toast.error('发布失败', { description: getErrorMessage(error) });
    } finally {
      setPublishing(false);
    }
  }, []);

  return (
    <>
      <div className='space-y-4'>
        <Button
          variant='ghost'
          size='sm'
          className='h-8 gap-1.5 px-0 text-xs'
          onClick={() => router.back()}
        >
          <ArrowLeft className='size-3.5' />
          返回
        </Button>

        <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
          <div className='flex items-center gap-2'>
            <Route className='size-5 text-primary' />
            <div className='flex flex-wrap items-center gap-2'>
              <h1 className='text-2xl font-semibold tracking-tight'>
                {route.site_name}
              </h1>
              <Badge variant={route.enabled ? 'default' : 'secondary'}>
                {route.enabled ? '已启用' : '已停用'}
              </Badge>
            </div>
          </div>

          <div className='flex flex-wrap gap-2'>
            <Button
              size='sm'
              className='h-8 gap-1.5 text-xs'
              disabled={publishing || diffLoading}
              onClick={() => void handlePublishClick()}
            >
              {publishing || diffLoading ? (
                <Loader2 className='size-3.5 animate-spin' />
              ) : (
                <Upload className='size-3.5' />
              )}
              发布配置
            </Button>
          </div>
        </div>
      </div>

      <AlertDialog
        open={publishConfirmOpen}
        onOpenChange={setPublishConfirmOpen}
      >
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
              {publishing ? (
                <Loader2 className='size-4 animate-spin' />
              ) : (
                '确认发布'
              )}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
