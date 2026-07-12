'use client';

import Link from 'next/link';
import {useMemo, useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Globe, Plus, Search} from 'lucide-react';

import {Button} from '@/components/ui/button';
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from '@/components/ui/card';
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {ErrorInline} from '@/components/layout/error';
import {Input} from '@/components/ui/input';
import {LoadingStateWithBorder} from '@/components/layout/loading';
import {ZoneService, zoneQueryKey} from '@/lib/services/openflare';

import {ZoneEditorDialog} from './[zoneId]/components/zone-editor-dialog';
import {getErrorMessage} from './components/website-utils';

export default function WebsitesPage() {
  const [search, setSearch] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const zonesQuery = useQuery({
    queryKey: zoneQueryKey,
    queryFn: () => ZoneService.list(),
  });

  const zones = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    const list = zonesQuery.data ?? [];
    if (!keyword) {
      return list;
    }
    return list.filter((zone) => zone.domain.toLowerCase().includes(keyword));
  }, [search, zonesQuery.data]);

  return (
    <div className="space-y-6 py-6 px-1">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Globe className="size-5 text-primary" />
          <h1 className="text-2xl font-semibold tracking-tight">网站</h1>
        </div>
        <Button size="sm" className="h-7 text-xs" onClick={() => setEditorOpen(true)}>
          <Plus className="mr-1 size-3.5" />
          新增 Zone
        </Button>
      </div>

      <Card className="border-dashed shadow-none">
        <CardHeader className="gap-3 pb-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <CardTitle className="text-base font-semibold">Zone 列表</CardTitle>
            <CardDescription>
              以可注册根域组织网站，并在详情中维护明确 FQDN、证书与路由关联。
            </CardDescription>
          </div>
          <div className="relative w-full sm:w-64">
            <Search className="pointer-events-none absolute left-2.5 top-2.5 size-3.5 text-muted-foreground" />
            <Input
              aria-label="搜索 Zone 根域"
              placeholder="搜索 Zone 根域"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              className="h-8 pl-8 text-xs"
            />
          </div>
        </CardHeader>
        <CardContent>
          {zonesQuery.isLoading ? (
            <LoadingStateWithBorder icon={Globe} description="加载 Zone 列表中..." />
          ) : zonesQuery.isError ? (
            <div className="rounded-lg border border-dashed p-8">
              <ErrorInline
                className="justify-center"
                message={getErrorMessage(zonesQuery.error)}
                onRetry={() => void zonesQuery.refetch()}
              />
            </div>
          ) : zones.length === 0 ? (
            <EmptyStateWithBorder
              icon={Globe}
              description={
                search
                  ? '未找到匹配的 Zone。'
                  : '暂无 Zone，点击右上角「新增 Zone」开始录入。'
              }
            />
          ) : (
            <div className="grid gap-3 lg:grid-cols-2">
              {zones.map((zone) => (
                <div key={zone.id} className="rounded-lg border bg-card p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <h2 className="truncate text-sm font-semibold">{zone.domain}</h2>
                      <p className="mt-2 text-xs text-muted-foreground">
                        {zone.domain_count ?? 0} 个域名
                        {zone.remark ? ` · ${zone.remark}` : ''}
                      </p>
                    </div>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-7 shrink-0 text-xs"
                      asChild
                    >
                      <Link href={`/websites/${zone.id}`}>管理</Link>
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <ZoneEditorDialog open={editorOpen} onOpenChange={setEditorOpen} />
    </div>
  );
}
