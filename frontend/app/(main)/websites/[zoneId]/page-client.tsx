'use client';

import {usePathname, useRouter, useSearchParams} from 'next/navigation';
import {useCallback, useMemo, useState} from 'react';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {ArrowLeft, Globe, Pencil, Trash2} from 'lucide-react';
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
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {LoadingStateWithBorder} from '@/components/layout/loading';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {
  ProxyRouteService,
  TlsCertificateService,
  ZoneService,
  zoneQueryKey,
} from '@/lib/services/openflare';

import {getErrorMessage} from '../components/website-utils';
import {ZoneCertificatesPanel} from './components/zone-certificates';
import {ZoneDomainsTable} from './components/zone-domains-table';
import {ZoneEditorDialog} from './components/zone-editor-dialog';
import {ZoneOverviewPanel} from './components/zone-overview';

const zoneTabs = ['overview', 'domains', 'certificates', 'settings'] as const;
export type ZonePageTab = (typeof zoneTabs)[number];

function getZonePageTab(value: string | null | undefined): ZonePageTab {
  return zoneTabs.includes(value as ZonePageTab) ? (value as ZonePageTab) : 'overview';
}

export function ZonePageClient({zoneId}: {zoneId: number}) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const activeTab = useMemo(
    () => getZonePageTab(searchParams.get('tab')),
    [searchParams],
  );
  const [editZone, setEditZone] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const setActiveTab = useCallback(
    (tab: string) => {
      const next = getZonePageTab(tab);
      const params = new URLSearchParams(searchParams.toString());
      if (next === 'overview') {
        params.delete('tab');
      } else {
        params.set('tab', next);
      }
      const query = params.toString();
      router.replace(query ? `${pathname}?${query}` : pathname, {scroll: false});
    },
    [pathname, router, searchParams],
  );

  const overviewQuery = useQuery({
    queryKey: [...zoneQueryKey, zoneId],
    queryFn: () => ZoneService.getOverview(zoneId),
    enabled: Number.isInteger(zoneId) && zoneId > 0,
  });

  const certificatesQuery = useQuery({
    queryKey: ['openflare', 'tls-certificates'],
    queryFn: () => TlsCertificateService.list(),
    enabled: overviewQuery.isSuccess,
  });

  const routesQuery = useQuery({
    queryKey: ['openflare', 'proxy-routes'],
    queryFn: () => ProxyRouteService.list(),
    enabled: overviewQuery.isSuccess,
  });

  const deleteZone = useMutation({
    mutationFn: () => ZoneService.deleteById(zoneId),
    onSuccess: async () => {
      toast.success('Zone 已删除');
      await queryClient.invalidateQueries({queryKey: zoneQueryKey});
      window.location.assign('/websites');
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  if (!Number.isInteger(zoneId) || zoneId <= 0) {
    return (
      <div className="py-6 px-1">
        <EmptyStateWithBorder
          icon={Globe}
          description="无效的网站 ID，请从网站列表进入详情页。"
        />
      </div>
    );
  }

  if (overviewQuery.isLoading) {
    return (
      <div className="py-6 px-1">
        <LoadingStateWithBorder icon={Globe} description="加载 Zone 详情中..." />
      </div>
    );
  }

  if (overviewQuery.isError || !overviewQuery.data) {
    return (
      <div className="space-y-4 py-6 px-1">
        <Button
          variant="ghost"
          size="sm"
          className="h-8 gap-1.5 px-0 text-xs"
          onClick={() => router.back()}
        >
          <ArrowLeft className="size-3.5" />
          返回
        </Button>
        <EmptyStateWithBorder
          icon={Globe}
          description="网站不存在，可能已被删除或 ID 无效。"
        />
      </div>
    );
  }

  const overview = overviewQuery.data;
  const certificates = certificatesQuery.data ?? [];
  const routes = routesQuery.data ?? [];
  const boundCertCount = new Set(
    overview.domains.map((domain) => domain.cert_id).filter((id): id is number => id != null),
  ).size;

  return (
    <div className="space-y-6 py-6 px-1">
      <div className="space-y-4">
        <Button
          variant="ghost"
          size="sm"
          className="h-8 gap-1.5 px-0 text-xs"
          onClick={() => router.back()}
        >
          <ArrowLeft className="size-3.5" />
          返回
        </Button>

        <div className="flex items-center gap-2">
          <Globe className="size-5 text-primary" />
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">{overview.zone.domain}</h1>
            <p className="text-sm text-muted-foreground">Zone 网站管理</p>
          </div>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList variant="line" className="mb-6 inline-flex w-fit gap-8">
          <TabsTrigger value="overview" className="px-0 pb-2 text-xs font-semibold">
            概览
          </TabsTrigger>
          <TabsTrigger value="domains" className="px-0 pb-2 text-xs font-semibold">
            域名 ({overview.domains.length})
          </TabsTrigger>
          <TabsTrigger value="certificates" className="px-0 pb-2 text-xs font-semibold">
            证书 ({boundCertCount})
          </TabsTrigger>
          <TabsTrigger value="settings" className="px-0 pb-2 text-xs font-semibold">
            设置
          </TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <ZoneOverviewPanel overview={overview} />
        </TabsContent>
        <TabsContent value="domains">
          <ZoneDomainsTable
            zoneId={zoneId}
            zoneRoot={overview.zone.domain}
            domains={overview.domains}
            certificates={certificates}
            routes={routes}
            routesLoading={routesQuery.isLoading}
            onChanged={() => overviewQuery.refetch()}
          />
        </TabsContent>
        <TabsContent value="certificates">
          <ZoneCertificatesPanel domains={overview.domains} certificates={certificates} />
        </TabsContent>
        <TabsContent value="settings">
          <div className="space-y-4">
            <div className="rounded-lg border p-4">
              <p className="text-sm font-semibold">基本信息</p>
              <p className="mt-1 text-sm text-muted-foreground">
                维护 Zone 的可注册根域。根域变更后，请确认其下域名仍归属该根域。
              </p>
              <div className="mt-3 grid gap-2 text-sm sm:grid-cols-2">
                <div>
                  <p className="text-xs text-muted-foreground">当前根域</p>
                  <p className="mt-1 font-mono text-[13px] font-medium">{overview.zone.domain}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">域名数量</p>
                  <p className="mt-1 font-mono text-[13px] font-medium">{overview.domains.length}</p>
                </div>
              </div>
              <Button
                variant="outline"
                size="sm"
                className="mt-4 h-7 text-xs"
                onClick={() => setEditZone(true)}
              >
                <Pencil className="mr-1 size-3.5" />
                编辑根域
              </Button>
            </div>

            <div className="rounded-lg border border-destructive/30 p-4">
              <p className="text-sm font-semibold text-destructive">危险操作</p>
              <p className="mt-1 text-sm text-muted-foreground">
                删除 Zone 前须清空其下全部域名；删除后不可恢复。
              </p>
              <Button
                variant="destructive"
                size="sm"
                className="mt-4 h-7 text-xs"
                onClick={() => setConfirmDelete(true)}
              >
                <Trash2 className="mr-1 size-3.5" />
                删除 Zone
              </Button>
            </div>
          </div>
        </TabsContent>
      </Tabs>

      <ZoneEditorDialog
        open={editZone}
        onOpenChange={setEditZone}
        zone={overview.zone}
      />

      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除 Zone</AlertDialogTitle>
            <AlertDialogDescription>
              确认删除 {overview.zone.domain} 吗？其下域名须先解绑路由后才能删除，此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteZone.isPending}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-white hover:bg-destructive/90"
              disabled={deleteZone.isPending}
              onClick={(event) => {
                event.preventDefault();
                deleteZone.mutate();
              }}
            >
              {deleteZone.isPending ? '删除中…' : '确认删除'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
