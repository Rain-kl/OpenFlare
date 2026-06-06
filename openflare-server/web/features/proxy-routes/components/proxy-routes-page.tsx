'use client';

import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useMemo, useState} from 'react';

import {EmptyState} from '@/components/feedback/empty-state';
import {ErrorState} from '@/components/feedback/error-state';
import {InlineMessage} from '@/components/feedback/inline-message';
import {LoadingState} from '@/components/feedback/loading-state';
import {PageHeader} from '@/components/layout/page-header';
import {AppCard} from '@/components/ui/app-card';
import {StatusBadge} from '@/components/ui/status-badge';
import {getConfigVersionDiff, publishConfigVersion,} from '@/features/config-versions/api/config-versions';
import {ProxyRouteCreateDrawer} from '@/features/proxy-routes/components/proxy-route-create-drawer';
import {getErrorMessage, getUpstreamSummary, getWebsiteStatusBadges,} from '@/features/proxy-routes/helpers';
import {deleteProxyRoute, getProxyRoutes,} from '@/features/proxy-routes/api/proxy-routes';
import type {ProxyRouteItem} from '@/features/proxy-routes/types';
import {
    DangerButton,
    PrimaryButton,
    ResourceInput,
    SecondaryButton,
} from '@/features/shared/components/resource-primitives';

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

function hasConfigChanges(diff: {
  active_version?: string;
  added_sites: string[];
  removed_sites: string[];
  modified_sites: string[];
  added_domains: string[];
  removed_domains: string[];
  modified_domains: string[];
  main_config_changed: boolean;
  waf_config_changed?: boolean;
  changed_option_keys: string[];
}) {
  return (
    diff.added_sites.length > 0 ||
    diff.removed_sites.length > 0 ||
    diff.modified_sites.length > 0 ||
    diff.added_domains.length > 0 ||
    diff.removed_domains.length > 0 ||
    diff.modified_domains.length > 0 ||
    diff.main_config_changed ||
    Boolean(diff.waf_config_changed) ||
    diff.changed_option_keys.length > 0 ||
    !diff.active_version
  );
}

export function ProxyRoutesPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [keyword, setKeyword] = useState('');
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const routesQuery = useQuery({
    queryKey: ['proxy-routes'],
    queryFn: getProxyRoutes,
  });
  const diffQuery = useQuery({
    queryKey: ['config-versions', 'diff'],
    queryFn: getConfigVersionDiff,
  });

  const deleteMutation = useMutation({
    mutationFn: deleteProxyRoute,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '网站已删除。' });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['proxy-routes'] }),
        queryClient.invalidateQueries({
          queryKey: ['config-versions', 'diff'],
        }),
      ]);
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const publishMutation = useMutation({
    mutationFn: publishConfigVersion,
    onSuccess: async (version) => {
      setFeedback({
        tone: 'success',
        message: `配置已发布，版本号 ${version.version}。`,
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['config-versions'] }),
        queryClient.invalidateQueries({
          queryKey: ['config-versions', 'diff'],
        }),
      ]);
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const routes = useMemo(() => routesQuery.data ?? [], [routesQuery.data]);
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
        ...route.upstream_list,
        route.remark,
      ]
        .join(' ')
        .toLowerCase();

      return haystack.includes(normalizedKeyword);
    });
  }, [keyword, routes]);

  const diff = diffQuery.data;
  const totalDomains = routes.reduce(
    (sum, route) => sum + route.domain_count,
    0,
  );
  const enabledCount = routes.filter((route) => route.enabled).length;

  const handleDelete = (route: ProxyRouteItem) => {
    if (
      !window.confirm(
        `确认删除网站 ${route.site_name} 吗？\n这会删除该站点下的全部域名与配置。`,
      )
    ) {
      return;
    }

    setFeedback(null);
    deleteMutation.mutate(route.id);
  };

  const handlePublish = () => {
    if (!diff || !hasConfigChanges(diff)) {
      setFeedback({ tone: 'info', message: '当前草稿没有可发布的变更。' });
      return;
    }

    const summary = [
      `新增网站 ${diff.added_sites.length} 个`,
      `删除网站 ${diff.removed_sites.length} 个`,
      `修改网站 ${diff.modified_sites.length} 个`,
      `域名变更 ${diff.added_domains.length + diff.removed_domains.length + diff.modified_domains.length} 项`,
    ].join('，');

    if (!window.confirm(`确认发布当前配置吗？\n${summary}`)) {
      return;
    }

    setFeedback(null);
    publishMutation.mutate(false);
  };

  if (routesQuery.isLoading) {
    return <LoadingState />;
  }

  if (routesQuery.isError) {
    return (
      <ErrorState
        title="规则列表加载失败"
        description={getErrorMessage(routesQuery.error)}
      />
    );
  }

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="规则配置"
          action={
            <div className="flex flex-wrap gap-3">
              <SecondaryButton
                type="button"
                onClick={handlePublish}
                disabled={publishMutation.isPending || diffQuery.isLoading}
              >
                {publishMutation.isPending ? '发布中...' : '发布配置'}
              </SecondaryButton>
              <PrimaryButton
                type="button"
                onClick={() => {
                  setFeedback(null);
                  setIsCreateOpen(true);
                }}
              >
                新建规则
              </PrimaryButton>
            </div>
          }
        />

        {feedback ? (
          <InlineMessage
            tone={feedback.tone}
            message={feedback.message}
            onClear={() => setFeedback(null)}
          />
        ) : null}

        <div className="grid gap-4 xl:grid-cols-4">
          <AppCard title="网站数">
            <div className="space-y-2">
              <p className="text-3xl font-semibold text-[var(--foreground-primary)]">
                {routes.length}
              </p>
              <p className="text-sm text-[var(--foreground-secondary)]">
                已启用 {enabledCount} 个
              </p>
            </div>
          </AppCard>

          <AppCard title="域名数">
            <div className="space-y-2">
              <p className="text-3xl font-semibold text-[var(--foreground-primary)]">
                {totalDomains}
              </p>
              <p className="text-sm text-[var(--foreground-secondary)]">
                每个网站第一行域名为主域名
              </p>
            </div>
          </AppCard>

          <AppCard title="草稿状态">
            <div className="space-y-3">
              <StatusBadge
                label={
                  diff && hasConfigChanges(diff)
                    ? '有待发布变更'
                    : '已与线上一致'
                }
                variant={diff && hasConfigChanges(diff) ? 'warning' : 'success'}
              />
              <p className="text-sm text-[var(--foreground-secondary)]">
                {diff
                  ? `网站变更 ${diff.added_sites.length + diff.removed_sites.length + diff.modified_sites.length} 项`
                  : '正在读取 diff...'}
              </p>
            </div>
          </AppCard>

          <AppCard title="发布摘要">
            <div className="space-y-2 text-sm text-[var(--foreground-secondary)]">
              <p>新增网站：{diff?.added_sites.length ?? 0}</p>
              <p>删除网站：{diff?.removed_sites.length ?? 0}</p>
              <p>修改网站：{diff?.modified_sites.length ?? 0}</p>
            </div>
          </AppCard>
        </div>

        <AppCard title="规则列表">
          <div className="space-y-4">
            <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div className="max-w-xl">
                <ResourceInput
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  placeholder="搜索站点"
                />
              </div>
              {diff && hasConfigChanges(diff) ? (
                <div className="flex flex-wrap gap-2">
                  {diff.modified_sites.length > 0 ? (
                    <StatusBadge
                      label={`修改网站 ${diff.modified_sites.length}`}
                      variant="warning"
                    />
                  ) : null}
                  {diff.added_sites.length > 0 ? (
                    <StatusBadge
                      label={`新增网站 ${diff.added_sites.length}`}
                      variant="success"
                    />
                  ) : null}
                  {diff.removed_sites.length > 0 ? (
                    <StatusBadge
                      label={`删除网站 ${diff.removed_sites.length}`}
                      variant="danger"
                    />
                  ) : null}
                </div>
              ) : null}
            </div>

            {filteredRoutes.length === 0 ? (
              <EmptyState
                title={routes.length === 0 ? '暂无网站' : '没有匹配结果'}
                description={
                  routes.length === 0
                    ? '先创建一个网站，再进入配置子页面继续补齐 HTTPS、缓存和限流。'
                    : '试试调整搜索词，或者清空筛选条件。'
                }
              />
            ) : (
              <div className="grid gap-4 xl:grid-cols-2">
                {filteredRoutes.map((route) => (
                  <article
                    key={route.id}
                    className="rounded-[28px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5"
                  >
                    <div className="flex flex-col gap-5">
                      <div className="flex items-start justify-between gap-4">
                        <div className="space-y-3">
                          <div className="space-y-2">
                            <div className="flex flex-wrap items-center gap-2">
                              <h2 className="text-lg font-semibold text-[var(--foreground-primary)]">
                                {route.site_name}
                              </h2>
                              {getWebsiteStatusBadges(route).map((badge) => (
                                <StatusBadge
                                  key={`${route.id}-${badge.label}`}
                                  label={badge.label}
                                  variant={badge.variant}
                                />
                              ))}
                            </div>
                          </div>
                        </div>

                        <div className="flex flex-wrap gap-2">
                          <Link
                            href={`/proxy-route/detail?id=${route.id}&section=domains`}
                            className="inline-flex items-center justify-center rounded-2xl border border-[var(--border-default)] bg-[var(--control-background)] px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] transition hover:bg-[var(--control-background-hover)]"
                          >
                            配置
                          </Link>
                          <DangerButton
                            type="button"
                            onClick={() => handleDelete(route)}
                            disabled={deleteMutation.isPending}
                          >
                            删除
                          </DangerButton>
                        </div>
                      </div>

                      <div className="grid gap-3 md:grid-cols-2">
                        <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] px-4 py-3">
                          <p className="text-xs tracking-[0.18em] text-[var(--foreground-muted)] uppercase">
                            域名列表
                          </p>
                          <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                            {route.domains.join(' / ')}
                          </p>
                        </div>

                        <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] px-4 py-3">
                          <p className="text-xs tracking-[0.18em] text-[var(--foreground-muted)] uppercase">
                            上游摘要
                          </p>
                          <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                            {getUpstreamSummary(route)}
                          </p>
                        </div>
                      </div>

                      <p className="text-sm text-[var(--foreground-secondary)]">
                        {route.remark || null}
                      </p>
                    </div>
                  </article>
                ))}
              </div>
            )}
          </div>
        </AppCard>
      </div>

      <ProxyRouteCreateDrawer
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
        domainSuggestionSources={routes.flatMap((route) => route.domains)}
        onCreated={async (route) => {
          await Promise.all([
            queryClient.invalidateQueries({ queryKey: ['proxy-routes'] }),
            queryClient.invalidateQueries({
              queryKey: ['config-versions', 'diff'],
            }),
          ]);
          router.push(`/proxy-route/detail?id=${route.id}&section=domains`);
        }}
      />
    </>
  );
}
