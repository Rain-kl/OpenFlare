'use client';

import Link from 'next/link';
import {usePathname, useRouter, useSearchParams} from 'next/navigation';
import {useCallback, useMemo, type ReactNode} from 'react';
import type {LucideIcon} from 'lucide-react';
import {ArrowLeft} from 'lucide-react';

import {Button} from '@/components/ui/button';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {cn} from '@/lib/utils';

import {NodeKpiCard} from './node-detail-primitives';
import {NodeStatusBadge} from './node-status-badge';
import type {StatusTone} from './node-utils';

export type NodeDetailTabId = 'overview' | 'dashboard' | 'manage';

export type NodeDetailTabConfig = {
  id: NodeDetailTabId;
  label: string;
  description: string;
  icon?: LucideIcon;
};

export type NodeDetailKpi = {
  label: string;
  value: ReactNode;
  icon?: LucideIcon;
};

const TAB_CONFIGS: NodeDetailTabConfig[] = [
  {
    id: 'overview',
    label: '概览',
    description: '快速查看节点身份、运行状态与关键指标。',
  },
  {
    id: 'dashboard',
    label: '数据看板',
    description: '系统画像、资源快照、流量趋势与健康事件时间线。',
  },
  {
    id: 'manage',
    label: '配置与部署',
    description: '版本同步、部署命令与节点运维配置。',
  },
];

function isValidTab(value: string | null): value is NodeDetailTabId {
  return value === 'overview' || value === 'dashboard' || value === 'manage';
}

export function NodeDetailShell({
  title,
  typeLabel,
  typeTone = 'info',
  statusBadges,
  actions,
  kpis,
  overview,
  dashboard,
  manage,
  defaultTab = 'overview',
}: {
  title: string;
  typeLabel: string;
  typeTone?: StatusTone;
  statusBadges: Array<{ label: string; tone: StatusTone }>;
  actions: ReactNode;
  kpis: NodeDetailKpi[];
  overview: ReactNode;
  dashboard: ReactNode;
  manage: ReactNode;
  defaultTab?: NodeDetailTabId;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const activeTab = useMemo(() => {
    const tab = searchParams.get('tab');
    return isValidTab(tab) ? tab : defaultTab;
  }, [defaultTab, searchParams]);

  const setActiveTab = useCallback(
    (tab: NodeDetailTabId) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set('tab', tab);
      router.replace(`${pathname}?${params.toString()}`, { scroll: false });
    },
    [pathname, router, searchParams],
  );

  const activeTabMeta = TAB_CONFIGS.find((tab) => tab.id === activeTab) ?? TAB_CONFIGS[0];

  return (
    <div className="py-6 px-1 space-y-6">
      <section className="overflow-hidden rounded-2xl border bg-gradient-to-br from-card via-card to-muted/30">
        <div className="flex flex-col gap-5 p-5 md:p-6">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="flex min-w-0 items-start gap-3">
              <Button variant="ghost" size="sm" className="mt-0.5 h-8 w-8 shrink-0 p-0" asChild>
                <Link href="/nodes" aria-label="返回节点列表">
                  <ArrowLeft className="size-4" />
                </Link>
              </Button>

              <div className="min-w-0 space-y-2">
                <div className="flex flex-wrap items-center gap-2">
                  <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
                  <NodeStatusBadge label={typeLabel} tone={typeTone} />
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {statusBadges.map((badge) => (
                    <NodeStatusBadge
                      key={badge.label}
                      label={badge.label}
                      tone={badge.tone}
                    />
                  ))}
                </div>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">{actions}</div>
          </div>

          {kpis.length > 0 ? (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              {kpis.map((kpi) => (
                <NodeKpiCard
                  key={kpi.label}
                  label={kpi.label}
                  value={kpi.value}
                  icon={kpi.icon}
                />
              ))}
            </div>
          ) : null}
        </div>
      </section>

      <Tabs
        value={activeTab}
        onValueChange={(value) => {
          if (isValidTab(value)) {
            setActiveTab(value);
          }
        }}
        className="w-full gap-0"
      >
        <div className="space-y-3 border-b pb-1">
          <TabsList variant="line" className="h-auto w-full justify-start gap-6 bg-transparent p-0">
            {TAB_CONFIGS.map((tab) => (
              <TabsTrigger
                key={tab.id}
                value={tab.id}
                className={cn(
                  'h-auto flex-none rounded-none px-0 pb-3 pt-1 text-sm font-semibold',
                  'data-[state=active]:text-foreground data-[state=inactive]:text-muted-foreground',
                )}
              >
                {tab.label}
              </TabsTrigger>
            ))}
          </TabsList>
          <p className="text-sm text-muted-foreground">{activeTabMeta.description}</p>
        </div>

        <TabsContent value="overview" className="mt-6 outline-none">
          {activeTab === 'overview' ? overview : null}
        </TabsContent>
        <TabsContent value="dashboard" className="mt-6 outline-none">
          {activeTab === 'dashboard' ? dashboard : null}
        </TabsContent>
        <TabsContent value="manage" className="mt-6 outline-none">
          {activeTab === 'manage' ? manage : null}
        </TabsContent>
      </Tabs>
    </div>
  );
}