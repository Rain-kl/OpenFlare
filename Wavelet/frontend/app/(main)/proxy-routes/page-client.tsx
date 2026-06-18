'use client';

import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {useCallback, useEffect, useMemo, useState} from 'react';
import {Plus, RefreshCw, Route, Trash2} from 'lucide-react';
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
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Input} from '@/components/ui/input';
import {Skeleton} from '@/components/ui/skeleton';
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from '@/components/ui/table';
import type {ProxyRouteItem} from '@/lib/services/openflare';
import {ProxyRouteService} from '@/lib/services/openflare';

import {getUpstreamSummary} from './components/helpers';
import {ProxyRouteCreateSheet} from './components/proxy-route-create-sheet';

export function ProxyRoutesPageClient() {
  const router = useRouter();
  const [routes, setRoutes] = useState<ProxyRouteItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [keyword, setKeyword] = useState('');
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ProxyRouteItem | null>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchRoutes = useCallback(async () => {
    setLoading(true);
    try {
      const data = await ProxyRouteService.list();
      setRoutes(data);
    } catch (error) {
      toast.error('规则列表加载失败', {
        description: error instanceof Error ? error.message : '未知错误',
      });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchRoutes();
  }, [fetchRoutes]);

  const filteredRoutes = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    if (!normalizedKeyword) {
      return routes;
    }

    return routes.filter((route) => {
      const haystack = [
        route.site_name,
        route.primary_domain,
        ...route.domains,
        route.origin_url,
        route.remark,
      ]
        .join(' ')
        .toLowerCase();

      return haystack.includes(normalizedKeyword);
    });
  }, [keyword, routes]);

  const handleDelete = async () => {
    if (!deleteTarget) {
      return;
    }

    setDeleting(true);
    try {
      await ProxyRouteService.deleteById(deleteTarget.id);
      toast.success('网站已删除');
      setDeleteTarget(null);
      await fetchRoutes();
    } catch (error) {
      toast.error('删除失败', {
        description: error instanceof Error ? error.message : '未知错误',
      });
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="py-6 px-1 space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <Route className="size-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">规则配置</h1>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="secondary"
            className="h-8 gap-1.5 text-xs"
            onClick={() => void fetchRoutes()}
            disabled={loading}
          >
            <RefreshCw className={`size-3 ${loading ? 'animate-spin' : ''}`} />
            刷新
          </Button>
          <Button
            size="sm"
            className="h-8 gap-1.5 text-xs"
            onClick={() => setIsCreateOpen(true)}
          >
            <Plus className="size-3.5" />
            新建规则
          </Button>
        </div>
      </div>

      <Card className="border-border/40 shadow-sm">
        <CardHeader className="pb-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle className="text-sm font-semibold">规则列表</CardTitle>
            <Input
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              placeholder="搜索站点、域名或上游..."
              className="h-8 max-w-sm text-xs"
            />
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, index) => (
                <Skeleton key={index} className="h-10 w-full" />
              ))}
            </div>
          ) : filteredRoutes.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 py-12 text-center">
              <p className="text-sm font-medium">
                {routes.length === 0 ? '暂无网站' : '没有匹配结果'}
              </p>
              <p className="text-xs text-muted-foreground">
                {routes.length === 0
                  ? '先创建一个网站，再进入配置子页面继续补齐 HTTPS、缓存和限流。'
                  : '试试调整搜索词，或者清空筛选条件。'}
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>站点名称</TableHead>
                  <TableHead>域名</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>上游</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredRoutes.map((route) => (
                  <TableRow key={route.id}>
                    <TableCell className="font-medium">{route.site_name}</TableCell>
                    <TableCell className="max-w-[220px] truncate" title={route.domains.join(', ')}>
                      {route.primary_domain || route.domain}
                    </TableCell>
                    <TableCell>
                      <Badge variant={route.enabled ? 'default' : 'secondary'}>
                        {route.enabled ? '已启用' : '已停用'}
                      </Badge>
                    </TableCell>
                    <TableCell className="max-w-[280px] truncate" title={getUpstreamSummary(route)}>
                      {getUpstreamSummary(route)}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button variant="outline" size="sm" className="h-7 text-xs" asChild>
                          <Link href={`/proxy-routes/detail?id=${route.id}`}>
                            配置
                          </Link>
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 text-xs text-destructive hover:text-destructive"
                          onClick={() => setDeleteTarget(route)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <ProxyRouteCreateSheet
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
        onCreated={(route) => {
          toast.success('网站已创建');
          void fetchRoutes();
          router.push(`/proxy-routes/detail?id=${route.id}&section=domains`);
        }}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除网站</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget
                ? `确认删除网站 ${deleteTarget.site_name} 吗？这会删除该站点下的全部域名与配置。`
                : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-white hover:bg-destructive/90"
              disabled={deleting}
              onClick={(event) => {
                event.preventDefault();
                void handleDelete();
              }}
            >
              {deleting ? '删除中...' : '删除'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
