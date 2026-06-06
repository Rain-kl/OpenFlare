'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Globe2, Network, Plus, Save, ShieldCheck, Trash2 } from 'lucide-react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { getProxyRoutes } from '@/features/proxy-routes/api/proxy-routes';
import {
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceTextarea,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
import {
  createWAFRuleGroup,
  deleteWAFRuleGroup,
  getWAFIPGroups,
  getWAFRuleGroups,
  replaceWAFRuleGroupSites,
  updateWAFRuleGroup,
} from '@/features/waf/api/waf';
import type {
  WAFIPGroup,
  WAFRuleGroup,
  WAFRuleGroupPayload,
} from '@/features/waf/types';
import { cn } from '@/lib/utils/cn';

import { RuleEntryModal } from './rule-entry-modal';
import { SiteApplyDrawer } from './site-apply-drawer';
import { PowTabPanel } from './pow-tab-panel';
import { RuleListSection } from './rule-list-section';
import { TabButton } from './tab-button';
import {
  buildCountryOptions,
  buildDraft,
  countRuleEntries,
  defaultRuleModalState,
  emptyDraft,
  formatCountryItem,
  getErrorMessage,
  getListFieldKey,
  normalizeItems,
  tabItems,
  textToList,
  updateDraftList,
} from './helpers';
import type {
  FeedbackState,
  ListFieldKey,
  RuleModalState,
  WAFTab,
} from './types';

export function WAFPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [selectedID, setSelectedID] = useState<number | null>(null);
  const [activeTab, setActiveTab] = useState<WAFTab>('basic');
  const [draft, setDraft] = useState<WAFRuleGroupPayload>(emptyDraft);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [applyGroup, setApplyGroup] = useState<WAFRuleGroup | null>(null);
  const [ruleModal, setRuleModal] = useState<RuleModalState>(
    defaultRuleModalState,
  );

  const groupsQuery = useQuery({
    queryKey: ['waf', 'rule-groups'],
    queryFn: getWAFRuleGroups,
  });
  const ipGroupsQuery = useQuery({
    queryKey: ['waf', 'ip-groups'],
    queryFn: getWAFIPGroups,
  });
  const routesQuery = useQuery({
    queryKey: ['proxy-routes'],
    queryFn: getProxyRoutes,
  });

  const groups = useMemo(() => groupsQuery.data ?? [], [groupsQuery.data]);
  const ipGroups = useMemo(
    () => ipGroupsQuery.data ?? [],
    [ipGroupsQuery.data],
  );
  const routes = useMemo(() => routesQuery.data ?? [], [routesQuery.data]);
  const countryOptions = useMemo(() => buildCountryOptions(), []);
  const countryLabelMap = useMemo(
    () => new Map(countryOptions.map((option) => [option.code, option.label])),
    [countryOptions],
  );

  const selectedGroup = useMemo(
    () =>
      selectedID === 0
        ? null
        : (groups.find((group) => group.id === selectedID) ??
          groups[0] ??
          null),
    [groups, selectedID],
  );

  useEffect(() => {
    if (selectedGroup) {
      setSelectedID(selectedGroup.id);
      setDraft(buildDraft(selectedGroup));
    }
  }, [selectedGroup]);

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['waf', 'rule-groups'] }),
      queryClient.invalidateQueries({ queryKey: ['waf', 'ip-groups'] }),
      queryClient.invalidateQueries({ queryKey: ['config-versions', 'diff'] }),
    ]);
  };

  const saveMutation = useMutation({
    mutationFn: (payload: WAFRuleGroupPayload) => {
      if (selectedGroup) {
        return updateWAFRuleGroup(selectedGroup.id, payload);
      }
      return createWAFRuleGroup(payload);
    },
    onSuccess: async (group) => {
      setSelectedID(group.id);
      setFeedback({ tone: 'success', message: 'WAF 规则组已保存。' });
      await invalidate();
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteWAFRuleGroup,
    onSuccess: async () => {
      setSelectedID(null);
      setFeedback({ tone: 'success', message: 'WAF 规则组已删除。' });
      await invalidate();
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const applyMutation = useMutation({
    mutationFn: ({ id, ids }: { id: number; ids: number[] }) =>
      replaceWAFRuleGroupSites(id, ids),
    onSuccess: async () => {
      setApplyGroup(null);
      setFeedback({ tone: 'success', message: '规则组应用范围已更新。' });
      await invalidate();
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  if (
    groupsQuery.isLoading ||
    routesQuery.isLoading ||
    ipGroupsQuery.isLoading
  ) {
    return <LoadingState />;
  }
  if (groupsQuery.isError) {
    return (
      <ErrorState
        title="WAF 加载失败"
        description={getErrorMessage(groupsQuery.error)}
      />
    );
  }
  if (routesQuery.isError) {
    return (
      <ErrorState
        title="网站列表加载失败"
        description={getErrorMessage(routesQuery.error)}
      />
    );
  }
  if (ipGroupsQuery.isError) {
    return (
      <ErrorState
        title="IP 组加载失败"
        description={getErrorMessage(ipGroupsQuery.error)}
      />
    );
  }
  if (!selectedGroup && groups.length === 0) {
    return (
      <EmptyState
        title="WAF 尚未初始化"
        description="刷新页面后系统会自动创建全局规则组。"
      />
    );
  }

  const currentRuleCount = countRuleEntries(draft);
  const appliedSiteNames = selectedGroup?.is_global
    ? ['全部网站']
    : (selectedGroup?.applied_site_ids ?? [])
        .map(
          (id) =>
            routes.find((route) => route.id === id)?.site_name ?? `网站 #${id}`,
        )
        .sort((left, right) => left.localeCompare(right));
  const ipGroupByID = new Map(ipGroups.map((group) => [group.id, group]));
  const whitelistGroupItems = draft.ip_whitelist_group_ids
    .map((id) => ipGroupByID.get(id))
    .filter((group): group is WAFIPGroup => Boolean(group));
  const blacklistGroupItems = draft.ip_blacklist_group_ids
    .map((id) => ipGroupByID.get(id))
    .filter((group): group is WAFIPGroup => Boolean(group));

  const openRuleModal = () => {
    setRuleModal({ ...defaultRuleModalState, open: true });
  };

  const closeRuleModal = () => {
    setRuleModal(defaultRuleModalState);
  };

  const applyRuleModal = () => {
    const values =
      ruleModal.dimension === 'ip'
        ? textToList(ruleModal.ipValue)
        : ruleModal.dimension === 'ip_group'
          ? ruleModal.ipGroupIDs.map(String)
          : normalizeItems(ruleModal.countryValues);

    if (values.length === 0) {
      setFeedback({
        tone: 'danger',
        message:
          ruleModal.dimension === 'ip'
            ? '请先输入 IP 或 IP 段。'
            : ruleModal.dimension === 'ip_group'
              ? '请先选择 IP 组。'
              : '请先选择地域。',
      });
      return;
    }

    const listKey = getListFieldKey(ruleModal.listType, ruleModal.dimension);
    setDraft((current) =>
      updateDraftList(current, listKey, (items) =>
        normalizeItems([...items, ...values]),
      ),
    );
    setFeedback({
      tone: 'info',
      message: '名单项已添加到当前草稿，保存后生效。',
    });
    closeRuleModal();
    setActiveTab('lists');
  };

  const removeRuleItem = (key: ListFieldKey, value: string) => {
    setDraft((current) =>
      updateDraftList(current, key, (items) =>
        items.filter((item) => item !== value),
      ),
    );
  };
  const removeRuleGroup = (key: ListFieldKey, id: number) => {
    setDraft((current) =>
      updateDraftList(current, key, (items) =>
        items.filter((item) => item !== String(id)),
      ),
    );
  };

  const overviewItems: Array<{ label: string; value: ReactNode }> = [
    {
      label: '当前规则组',
      value: selectedGroup ? selectedGroup.name : '新建规则组',
    },
    { label: '启用状态', value: draft.enabled ? '启用中' : '已停用' },
    { label: '当前规则数', value: `${currentRuleCount} 条` },
    {
      label: '生效范围',
      value: selectedGroup?.is_global
        ? '全部网站'
        : `${selectedGroup?.applied_site_count ?? 0} 个网站`,
    },
    {
      label: '拦截返回',
      value: draft.block_response_body.trim()
        ? `${draft.block_status_code} + 自定义页面`
        : `${draft.block_status_code} 状态码`,
    },
    {
      label: '最后更新',
      value: selectedGroup?.updated_at
        ? new Date(selectedGroup.updated_at).toLocaleString('zh-CN')
        : '未保存',
    },
  ];

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="WAF"
          description="按规则组维护 WAF 与 PoW 防护规则，全局规则组始终应用到所有网站。"
          action={
            <div className="flex flex-wrap gap-3">
              <SecondaryButton
                type="button"
                onClick={() => router.push('/waf/ip-groups')}
              >
                <Network className="mr-2 h-4 w-4" />
                管理 IP 组
              </SecondaryButton>
              <PrimaryButton
                type="button"
                onClick={() => {
                  setSelectedID(0);
                  setActiveTab('basic');
                  setDraft({ ...emptyDraft, name: '自定义规则组' });
                }}
              >
                <Plus className="mr-2 h-4 w-4" />
                新建规则组
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

        <div className="grid gap-5 xl:grid-cols-[360px_minmax(0,1fr)]">
          <AppCard title="规则组">
            <div className="space-y-2">
              {groups.map((group) => (
                <button
                  key={group.id}
                  type="button"
                  onClick={() => {
                    setSelectedID(group.id);
                    setActiveTab('basic');
                  }}
                  className={cn(
                    'w-full rounded-2xl border px-4 py-3 text-left transition',
                    selectedGroup?.id === group.id
                      ? 'border-[var(--border-strong)] bg-[var(--accent-soft)]'
                      : 'border-[var(--border-default)] bg-[var(--surface-elevated)] hover:bg-[var(--surface-muted)]',
                  )}
                >
                  <span className="flex items-center justify-between gap-3">
                    <span className="flex min-w-0 items-center gap-2">
                      {group.is_global ? (
                        <Globe2 className="h-4 w-4" />
                      ) : (
                        <ShieldCheck className="h-4 w-4" />
                      )}
                      <span className="truncate text-sm font-semibold text-[var(--foreground-primary)]">
                        {group.name}
                      </span>
                    </span>
                    <span className="text-xs text-[var(--foreground-secondary)]">
                      {group.enabled ? '启用' : '停用'}
                    </span>
                  </span>
                  <span className="mt-2 block text-xs text-[var(--foreground-secondary)]">
                    {group.is_global
                      ? '应用全部网站'
                      : `已应用 ${group.applied_site_count} 个网站`}{' '}
                    · {countRuleEntries(group)} 条规则
                  </span>
                </button>
              ))}
            </div>
          </AppCard>

          <AppCard
            title={selectedGroup ? selectedGroup.name : '新建规则组'}
            description="简介：白名单命中后直接放行；未命中白名单时继续判断黑名单。"
            action={
              <div className="flex flex-wrap gap-3">
                {selectedGroup && !selectedGroup.is_global ? (
                  <SecondaryButton
                    type="button"
                    onClick={() => setApplyGroup(selectedGroup)}
                  >
                    一键应用
                  </SecondaryButton>
                ) : null}
                <PrimaryButton
                  type="button"
                  disabled={saveMutation.isPending}
                  onClick={() => saveMutation.mutate(draft)}
                >
                  <Save className="mr-2 h-4 w-4" />
                  {saveMutation.isPending ? '保存中...' : '保存规则组'}
                </PrimaryButton>
              </div>
            }
          >
            <div className="space-y-6">
              <div className="grid gap-3 md:grid-cols-4">
                {tabItems.map((tab) => (
                  <TabButton
                    key={tab.id}
                    label={tab.label}
                    active={activeTab === tab.id}
                    onClick={() => setActiveTab(tab.id)}
                  />
                ))}
              </div>

              {activeTab === 'basic' ? (
                <div className="space-y-5">
                  <div className="grid gap-5 xl:grid-cols-2">
                    <ResourceField label="规则组名称">
                      <ResourceInput
                        value={draft.name}
                        disabled={selectedGroup?.is_global}
                        onChange={(event) =>
                          setDraft((current) => ({
                            ...current,
                            name: event.target.value,
                          }))
                        }
                      />
                    </ResourceField>
                    <ToggleField
                      label="启用规则组"
                      description="关闭后保留配置，但不会参与匹配。"
                      checked={draft.enabled}
                      onChange={(checked) =>
                        setDraft((current) => ({
                          ...current,
                          enabled: checked,
                        }))
                      }
                    />
                    <ResourceField
                      label="备注"
                      className="xl:col-span-2"
                      hint="用于记录规则组用途、业务说明或变更备注。"
                    >
                      <ResourceInput
                        value={draft.remark}
                        onChange={(event) =>
                          setDraft((current) => ({
                            ...current,
                            remark: event.target.value,
                          }))
                        }
                      />
                    </ResourceField>
                  </div>
                  <div className="grid gap-5">
                    <div className="space-y-5">
                      <div className="rounded-[26px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5">
                        <div className="flex items-start justify-between gap-4">
                          <div>
                            <h3 className="text-sm font-semibold text-[var(--foreground-primary)]">
                              配置总览
                            </h3>
                          </div>
                          <span className="rounded-full border border-[var(--border-default)] px-2.5 py-1 text-xs font-medium text-[var(--foreground-secondary)]">
                            {selectedGroup?.is_global ? '全局' : '自定义'}
                          </span>
                        </div>

                        <dl className="mt-5 grid gap-4 sm:grid-cols-2">
                          {overviewItems.map((item) => (
                            <div
                              key={item.label}
                              className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-panel)] px-4 py-4"
                            >
                              <dt className="text-xs font-medium tracking-[0.18em] text-[var(--foreground-muted)] uppercase">
                                {item.label}
                              </dt>
                              <dd className="mt-2 text-sm font-medium text-[var(--foreground-primary)]">
                                {item.value}
                              </dd>
                            </div>
                          ))}
                        </dl>

                        <div className="mt-5">
                          <p className="text-xs font-medium tracking-[0.18em] text-[var(--foreground-muted)] uppercase">
                            当前应用网站
                          </p>
                          <div className="mt-3 flex flex-wrap gap-2">
                            {appliedSiteNames.length > 0 ? (
                              appliedSiteNames.map((name) => (
                                <span
                                  key={name}
                                  className="rounded-full border border-[var(--border-default)] bg-[var(--surface-panel)] px-3 py-2 text-sm text-[var(--foreground-secondary)]"
                                >
                                  {name}
                                </span>
                              ))
                            ) : (
                              <span className="rounded-2xl border border-dashed border-[var(--border-default)] px-4 py-3 text-sm text-[var(--foreground-secondary)]">
                                尚未绑定网站，可点击右上角「一键应用」进行配置。
                              </span>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              ) : null}

              {activeTab === 'lists' ? (
                <div className="space-y-5">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <h3 className="text-sm font-semibold text-[var(--foreground-primary)]">
                        黑白名单规则
                      </h3>
                    </div>
                    <PrimaryButton type="button" onClick={openRuleModal}>
                      <Plus className="mr-2 h-4 w-4" />
                      添加
                    </PrimaryButton>
                  </div>

                  <div className="grid gap-4 xl:grid-cols-2">
                    <RuleListSection
                      title="IP 白名单"
                      description="命中后直接放行，不再继续判断黑名单。"
                      items={draft.ip_whitelist}
                      groupItems={whitelistGroupItems}
                      tone="whitelist"
                      emptyText="暂无 IP 白名单规则。"
                      onRemove={(item) => removeRuleItem('ip_whitelist', item)}
                      onRemoveGroup={(id) =>
                        removeRuleGroup('ip_whitelist_group_ids', id)
                      }
                    />
                    <RuleListSection
                      title="IP 黑名单"
                      description="未命中白名单时，命中这些 IP / IP 段将被拦截。"
                      items={draft.ip_blacklist}
                      groupItems={blacklistGroupItems}
                      tone="blacklist"
                      emptyText="暂无 IP 黑名单规则。"
                      onRemove={(item) => removeRuleItem('ip_blacklist', item)}
                      onRemoveGroup={(id) =>
                        removeRuleGroup('ip_blacklist_group_ids', id)
                      }
                    />
                    <RuleListSection
                      title="地域白名单"
                      description="显示格式为国家代码与中文名，命中后直接放行。"
                      items={draft.country_whitelist.map((code) =>
                        formatCountryItem(code, countryLabelMap),
                      )}
                      tone="whitelist"
                      emptyText="暂无地域白名单规则。"
                      onRemove={(item) => {
                        const code = item.split(' ')[0] ?? item;
                        removeRuleItem('country_whitelist', code);
                      }}
                    />
                    <RuleListSection
                      title="地域黑名单"
                      description="当请求未命中白名单时，命中这些地域将被拦截。"
                      items={draft.country_blacklist.map((code) =>
                        formatCountryItem(code, countryLabelMap),
                      )}
                      tone="blacklist"
                      emptyText="暂无地域黑名单规则。"
                      onRemove={(item) => {
                        const code = item.split(' ')[0] ?? item;
                        removeRuleItem('country_blacklist', code);
                      }}
                    />
                  </div>
                </div>
              ) : null}

              {activeTab === 'pow' ? (
                <PowTabPanel
                  key={selectedID}
                  enabled={draft.pow_enabled}
                  config={draft.pow_config}
                  onChange={(enabled, config) =>
                    setDraft((current) => ({
                      ...current,
                      pow_enabled: enabled,
                      pow_config: config,
                    }))
                  }
                />
              ) : null}

              {activeTab === 'block' ? (
                <div className="grid gap-5 xl:grid-cols-[360px_minmax(0,1fr)]">
                  <div className="space-y-5">
                    <div className="rounded-[26px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5">
                      <h3 className="text-sm font-semibold text-[var(--foreground-primary)]">
                        拦截返回状态码
                      </h3>
                      <p className="mt-1 text-xs leading-5 text-[var(--foreground-secondary)]">
                        建议使用 403、418、451 等明确表达策略拦截含义的状态码。
                      </p>
                      <div className="mt-4 space-y-4">
                        <ResourceField label="状态码">
                          <ResourceInput
                            type="number"
                            min={400}
                            max={599}
                            value={draft.block_status_code}
                            onChange={(event) =>
                              setDraft((current) => ({
                                ...current,
                                block_status_code: Number(event.target.value),
                              }))
                            }
                          />
                        </ResourceField>
                        <div className="flex flex-wrap gap-2">
                          {[403, 418, 451, 503].map((code) => (
                            <button
                              key={code}
                              type="button"
                              onClick={() =>
                                setDraft((current) => ({
                                  ...current,
                                  block_status_code: code,
                                }))
                              }
                              className={cn(
                                'rounded-full border px-3 py-2 text-sm transition',
                                draft.block_status_code === code
                                  ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                                  : 'border-[var(--border-default)] bg-[var(--surface-panel)] text-[var(--foreground-secondary)] hover:bg-[var(--surface-muted)]',
                              )}
                            >
                              {code}
                            </button>
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-[26px] border border-[var(--border-default)] bg-[var(--surface-elevated)] p-5">
                    <ResourceField
                      label="拦截页面"
                      hint="支持直接输入 HTML 或纯文本。留空时只返回状态码。"
                    >
                      <ResourceTextarea
                        value={draft.block_response_body}
                        className="min-h-72"
                        placeholder="<html><body><h1>Request blocked</h1></body></html>"
                        onChange={(event) =>
                          setDraft((current) => ({
                            ...current,
                            block_response_body: event.target.value,
                          }))
                        }
                      />
                    </ResourceField>
                  </div>
                </div>
              ) : null}

              <div className="flex flex-wrap justify-between gap-3 border-t border-[var(--border-default)] pt-6">
                <div>
                  {selectedGroup && !selectedGroup.is_global ? (
                    <DangerButton
                      type="button"
                      disabled={deleteMutation.isPending}
                      onClick={() => {
                        if (
                          window.confirm(
                            `确认删除 WAF 规则组 ${selectedGroup.name} 吗？`,
                          )
                        ) {
                          deleteMutation.mutate(selectedGroup.id);
                        }
                      }}
                    >
                      <Trash2 className="mr-2 h-4 w-4" />
                      删除
                    </DangerButton>
                  ) : null}
                </div>
              </div>
            </div>
          </AppCard>
        </div>
      </div>

      <RuleEntryModal
        state={ruleModal}
        countryOptions={countryOptions}
        ipGroups={ipGroups}
        pending={saveMutation.isPending}
        onClose={closeRuleModal}
        onChange={(patch) =>
          setRuleModal((current) => ({ ...current, ...patch, open: true }))
        }
        onSubmit={applyRuleModal}
      />

      <SiteApplyDrawer
        group={applyGroup}
        routes={routes}
        open={Boolean(applyGroup)}
        pending={applyMutation.isPending}
        onOpenChange={(open) => {
          if (!open) {
            setApplyGroup(null);
          }
        }}
        onSave={(ids) => {
          if (applyGroup) {
            applyMutation.mutate({ id: applyGroup.id, ids });
          }
        }}
      />
    </>
  );
}
