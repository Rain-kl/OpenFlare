'use client';

import {useCallback, useEffect, useState} from 'react';
import {useRouter, useSearchParams} from 'next/navigation';

import {EmptyStateWithBorder} from '@/components/layout/empty';
import {ErrorInline} from '@/components/layout/error';
import {Skeleton} from '@/components/ui/skeleton';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import type {ProxyRouteConfigSection, ProxyRouteItem} from '@/lib/services/openflare';
import {ProxyRouteService} from '@/lib/services/openflare';

import {getProxyRouteConfigSection, proxyRouteConfigSections} from '../components/helpers';
import {AuthSection} from './components/auth-section';
import {CacheSection} from './components/cache-section';
import {DomainSection} from './components/domain-section';
import {LimitsSection} from './components/limits-section';
import {ProxySection} from './components/proxy-section';
import {RouteHeader} from './components/route-header';
import {WafSection} from './components/waf-section';

export function ProxyRouteDetailPageClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const routeId = Number(searchParams.get('id'));
  const activeSection = getProxyRouteConfigSection(searchParams.get('section'));
  const hasValidId = Number.isFinite(routeId) && routeId > 0;

  const [route, setRoute] = useState<ProxyRouteItem | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [sectionSaving, setSectionSaving] = useState(false);

  const handleSectionChange = useCallback(
    (section: ProxyRouteConfigSection) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set('section', section);
      router.replace(`/proxy-routes/detail?${params.toString()}`);
    },
    [router, searchParams],
  );

  const handleRouteUpdate = useCallback((updatedRoute: ProxyRouteItem) => {
    setRoute(updatedRoute);
  }, []);

  const fetchRoute = useCallback(async () => {
    if (!hasValidId) {
      setLoading(false);
      setRoute(null);
      setLoadError(null);
      return;
    }

    setLoading(true);
    setLoadError(null);

    try {
      const data = await ProxyRouteService.getById(routeId);
      setRoute(data);
    } catch (error) {
      setRoute(null);
      setLoadError(error instanceof Error ? error.message : '未知错误');
    } finally {
      setLoading(false);
    }
  }, [hasValidId, routeId]);

  useEffect(() => {
    void fetchRoute();
  }, [fetchRoute]);

  if (!hasValidId) {
    return (
      <div className="py-6 px-1">
        <ErrorInline message="缺少有效的规则 ID。" />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="py-6 px-1 space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-10 w-full max-w-xl" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (loadError) {
    return (
      <div className="py-6 px-1">
        <ErrorInline
          message={loadError}
          onRetry={() => void fetchRoute()}
        />
      </div>
    );
  }

  if (!route) {
    return (
      <div className="py-6 px-1">
        <EmptyStateWithBorder
          title="未找到对应规则"
          description="请返回列表重试。"
        />
      </div>
    );
  }

  const sectionProps = {
    route,
    onRouteUpdate: handleRouteUpdate,
    onSavingChange: setSectionSaving,
  };

  return (
    <div className="py-6 px-1 space-y-6">
      <RouteHeader route={route} activeSection={activeSection} saving={sectionSaving} />

      <Tabs
        value={activeSection}
        onValueChange={(value) => handleSectionChange(value as ProxyRouteConfigSection)}
        className="w-full"
      >
        <TabsList variant="line" className="w-fit inline-flex gap-8 mb-6">
          {proxyRouteConfigSections.map((section) => (
            <TabsTrigger
              key={section.key}
              value={section.key}
              className="px-0 pb-2 text-xs font-semibold"
            >
              {section.label}
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value="domains" className="focus-visible:outline-none">
          <DomainSection {...sectionProps} />
        </TabsContent>
        <TabsContent value="limits" className="focus-visible:outline-none">
          <LimitsSection {...sectionProps} />
        </TabsContent>
        <TabsContent value="proxy" className="focus-visible:outline-none">
          <ProxySection {...sectionProps} />
        </TabsContent>
        <TabsContent value="cache" className="focus-visible:outline-none">
          <CacheSection {...sectionProps} />
        </TabsContent>
        <TabsContent value="waf" className="focus-visible:outline-none">
          <WafSection route={route} onSavingChange={setSectionSaving} />
        </TabsContent>
        <TabsContent value="auth" className="focus-visible:outline-none">
          <AuthSection {...sectionProps} />
        </TabsContent>
      </Tabs>
    </div>
  );
}