'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Download, Play, Plus, Save, Trash2, Eye } from 'lucide-react';
import { useRouter } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import { AppModal } from '@/components/ui/app-modal';
import {
  createWAFIPGroup,
  deleteWAFIPGroup,
  getWAFIPGroups,
  syncWAFIPGroup,
  testWAFIPGroupAutoConfig,
  updateWAFIPGroup,
} from '@/features/waf/api/waf';
import type {
  WAFIPGroup,
  WAFIPGroupAutoTestResult,
  WAFIPGroupPayload,
  WAFIPGroupSubscriptionFormat,
  WAFIPGroupType,
} from '@/features/waf/types';
import {
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  ResourceTextarea,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';
import { cn } from '@/lib/utils/cn';

import { getErrorMessage, listToText, parseTextareaList } from './helpers';

type FeedbackState = {
  tone: 'success' | 'danger' | 'info';
  message: string;
};

type IPGroupDraft = WAFIPGroupPayload & {
  ip_list_text: string;
  auto_config_text: string;
};

const emptyIPGroupDraft: IPGroupDraft = {
  name: '',
  type: 'manual',
  enabled: true,
  ip_list: [],
  ip_list_text: '',
  auto_config: {},
  auto_config_text: '{}',
  subscription_url: '',
  subscription_format: 'text',
  subscription_mapping_rule: '',
  sync_interval_minutes: 1440,
  remark: '',
};

const typeLabels: Record<WAFIPGroupType, string> = {
  manual: '手动',
  automatic: '自动',
  subscription: '订阅',
};

const automaticPresetRules = [
  {
    name: '单 IP 404 高频扫描',
    expr: 'request_count > 100 && status_404_ratio >= 0.8',
  },
  {
    name: '单 IP 直连访问异常',
    expr: 'ip_host_count > 50 && ip_host_ratio > 0.5',
  },
];

function buildDraft(group: WAFIPGroup | null): IPGroupDraft {
  if (!group) {
    return { ...emptyIPGroupDraft };
  }
  return {
    name: group.name,
    type: group.type,
    enabled: group.enabled,
    ip_list: group.ip_list ?? [],
    ip_list_text: listToText(group.ip_list),
    auto_config: group.auto_config ?? {},
    auto_config_text: JSON.stringify(group.auto_config ?? {}, null, 2),
    subscription_url: group.subscription_url ?? '',
    subscription_format: group.subscription_format ?? 'text',
    subscription_mapping_rule: group.subscription_mapping_rule ?? '',
    sync_interval_minutes: group.sync_interval_minutes || 1440,
    remark: group.remark ?? '',
  };
}

function buildPayload(draft: IPGroupDraft): WAFIPGroupPayload {
  const autoConfig =
    draft.type === 'automatic'
      ? parseAutomaticConfig(draft.auto_config_text)
      : {};
  return {
    name: draft.name,
    type: draft.type,
    enabled: draft.enabled,
    ip_list: parseTextareaList(draft.ip_list_text),
    auto_config: autoConfig,
    subscription_url: draft.subscription_url,
    subscription_format: draft.subscription_format,
    subscription_mapping_rule: draft.subscription_mapping_rule,
    sync_interval_minutes: draft.sync_interval_minutes,
    remark: draft.remark,
  };
}

function parseAutomaticConfig(text: string): Record<string, unknown> {
  const parsed = JSON.parse(text || '{}') as unknown;
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error('自动配置必须是 JSON 对象。');
  }
  return parsed as Record<string, unknown>;
}

function appendAutomaticPresetRule(
  autoConfigText: string,
  rule: (typeof automaticPresetRules)[number],
) {
  const parsed = JSON.parse(autoConfigText || '{}') as unknown;
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    throw new Error('自动配置必须是 JSON 对象。');
  }
  const config = parsed as Record<string, unknown>;
  const rules = Array.isArray(config.rules) ? config.rules : [];
  const exists = rules.some(
    (item) =>
      item &&
      typeof item === 'object' &&
      'expr' in item &&
      (item as { expr?: unknown }).expr === rule.expr,
  );
  const nextRules = exists ? rules : [...rules, rule];
  return JSON.stringify(
    {
      lookback_minutes:
        typeof config.lookback_minutes === 'number'
          ? config.lookback_minutes
          : 60,
      ...config,
      rules: nextRules,
    },
    null,
    2,
  );
}

export function WAFIPGroupsPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [selectedID, setSelectedID] = useState<number | null>(null);
  const [draft, setDraft] = useState<IPGroupDraft>(emptyIPGroupDraft);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [autoTestResult, setAutoTestResult] =
    useState<WAFIPGroupAutoTestResult | null>(null);
  const [isCapturedIPsModalOpen, setIsCapturedIPsModalOpen] = useState(false);

  const groupsQuery = useQuery({
    queryKey: ['waf', 'ip-groups'],
    queryFn: getWAFIPGroups,
  });

  const groups = useMemo(() => groupsQuery.data ?? [], [groupsQuery.data]);
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
      queryClient.invalidateQueries({ queryKey: ['waf', 'ip-groups'] }),
      queryClient.invalidateQueries({ queryKey: ['waf', 'rule-groups'] }),
      queryClient.invalidateQueries({ queryKey: ['config-versions', 'diff'] }),
    ]);
  };

  const saveMutation = useMutation({
    mutationFn: (payload: WAFIPGroupPayload) => {
      if (selectedGroup) {
        return updateWAFIPGroup(selectedGroup.id, payload);
      }
      return createWAFIPGroup(payload);
    },
    onSuccess: async (group) => {
      setSelectedID(group.id);
      setFeedback({ tone: 'success', message: 'IP 组已保存。' });
      await invalidate();
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteWAFIPGroup,
    onSuccess: async () => {
      setSelectedID(null);
      setFeedback({ tone: 'success', message: 'IP 组已删除。' });
      await invalidate();
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const syncMutation = useMutation({
    mutationFn: syncWAFIPGroup,
    onSuccess: async (result) => {
      setSelectedID(result.group.id);
      setFeedback({ tone: 'success', message: result.message });
      await invalidate();
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const testMutation = useMutation({
    mutationFn: testWAFIPGroupAutoConfig,
    onSuccess: (result) => {
      setAutoTestResult(result);
      setFeedback({
        tone: result.matched_count > 0 ? 'success' : 'info',
        message:
          result.matched_count > 0
            ? `规则测试完成，命中 ${result.matched_count} 个 IP。`
            : '规则测试完成，当前未命中任何 IP。',
      });
    },
    onError: (error) => {
      setAutoTestResult(null);
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  useEffect(() => {
    setAutoTestResult(null);
  }, [draft.type, draft.auto_config_text, selectedID]);

  if (groupsQuery.isLoading) {
    return <LoadingState />;
  }
  if (groupsQuery.isError) {
    return (
      <ErrorState
        title="IP 组加载失败"
        description={getErrorMessage(groupsQuery.error)}
      />
    );
  }

  const saveDraft = () => {
    try {
      saveMutation.mutate(buildPayload(draft));
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    }
  };

  const testDraft = () => {
    try {
      testMutation.mutate({
        auto_config: parseAutomaticConfig(draft.auto_config_text),
      });
    } catch (error) {
      setAutoTestResult(null);
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="IP 组"
        description="维护可被 WAF IP 黑白名单引用的手动、自动与订阅 IP 集合。"
        action={
          <div className="flex flex-wrap gap-3">
            <SecondaryButton type="button" onClick={() => router.push('/waf')}>
              <ArrowLeft className="mr-2 h-4 w-4" />
              返回 WAF
            </SecondaryButton>
            <PrimaryButton
              type="button"
              onClick={() => {
                setSelectedID(0);
                setDraft({ ...emptyIPGroupDraft, name: '自定义 IP 组' });
              }}
            >
              <Plus className="mr-2 h-4 w-4" />
              新建 IP 组
            </PrimaryButton>
          </div>
        }
      />

      {feedback ? (
        <InlineMessage tone={feedback.tone} message={feedback.message} />
      ) : null}

      <div className="grid gap-5 xl:grid-cols-[360px_minmax(0,1fr)]">
        <AppCard title="IP 组列表">
          {groups.length === 0 && selectedID !== 0 ? (
            <EmptyState title="暂无 IP 组" />
          ) : (
            <div className="space-y-2">
              {groups.map((group) => (
                <button
                  key={group.id}
                  type="button"
                  onClick={() => setSelectedID(group.id)}
                  className={cn(
                    'w-full rounded-2xl border px-4 py-3 text-left transition',
                    selectedGroup?.id === group.id
                      ? 'border-[var(--border-strong)] bg-[var(--accent-soft)]'
                      : 'border-[var(--border-default)] bg-[var(--surface-elevated)] hover:bg-[var(--surface-muted)]',
                  )}
                >
                  <span className="flex items-center justify-between gap-3">
                    <span className="truncate text-sm font-semibold text-[var(--foreground-primary)]">
                      {group.name}
                    </span>
                    <span className="text-xs text-[var(--foreground-secondary)]">
                      {typeLabels[group.type]}
                    </span>
                  </span>
                  <span className="mt-2 block text-xs text-[var(--foreground-secondary)]">
                    {group.enabled ? '启用' : '停用'} · {group.ip_list.length}{' '}
                    条 · 被引用 {group.referenced_by_rule_count} 次
                  </span>
                </button>
              ))}
            </div>
          )}
        </AppCard>

        <AppCard
          title={selectedGroup ? selectedGroup.name : '新建 IP 组'}
          description={
            draft.type === 'automatic'
              ? '自动 IP 组会按 Expr 规则定时从请求日志中聚合命中 IP。'
              : '保存后可在 WAF 规则组黑白名单中引用。'
          }
          action={
            <div className="flex flex-wrap gap-3">
              {draft.type === 'automatic' ? (
                <SecondaryButton
                  type="button"
                  disabled={testMutation.isPending}
                  onClick={testDraft}
                >
                  <Play className="mr-2 h-4 w-4" />
                  {testMutation.isPending ? '测试中...' : '测试规则'}
                </SecondaryButton>
              ) : null}
              {selectedGroup?.type === 'subscription' ||
              selectedGroup?.type === 'automatic' ? (
                <SecondaryButton
                  type="button"
                  disabled={syncMutation.isPending}
                  onClick={() => syncMutation.mutate(selectedGroup.id)}
                >
                  <Download className="mr-2 h-4 w-4" />
                  {syncMutation.isPending
                    ? '执行中...'
                    : selectedGroup.type === 'automatic'
                      ? '立即执行'
                      : '立即同步'}
                </SecondaryButton>
              ) : null}
              {selectedGroup?.type === 'automatic' ? (
                <SecondaryButton
                  type="button"
                  onClick={() => setIsCapturedIPsModalOpen(true)}
                >
                  <Eye className="mr-2 h-4 w-4" />
                  查看已抓取 IP
                </SecondaryButton>
              ) : null}
              <PrimaryButton
                type="button"
                disabled={saveMutation.isPending}
                onClick={saveDraft}
              >
                <Save className="mr-2 h-4 w-4" />
                {saveMutation.isPending ? '保存中...' : '保存 IP 组'}
              </PrimaryButton>
            </div>
          }
        >
          <div className="space-y-6">
            <div className="grid gap-5 xl:grid-cols-2">
              <ResourceField label="IP 组名称">
                <ResourceInput
                  value={draft.name}
                  onChange={(event) =>
                    setDraft((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                />
              </ResourceField>
              <ResourceField label="类型">
                <ResourceSelect
                  value={draft.type}
                  onChange={(event) =>
                    setDraft((current) => ({
                      ...current,
                      type: event.target.value as WAFIPGroupType,
                    }))
                  }
                >
                  <option value="manual">手动</option>
                  <option value="automatic">自动</option>
                  <option value="subscription">订阅</option>
                </ResourceSelect>
              </ResourceField>
              <ToggleField
                label="启用 IP 组"
                description="关闭后保留配置，但发布时不会展开到 WAF 运行时名单。"
                checked={draft.enabled}
                onChange={(checked) =>
                  setDraft((current) => ({ ...current, enabled: checked }))
                }
              />
              <ResourceField label="备注">
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

            {draft.type === 'subscription' ? (
              <div className="grid gap-5 xl:grid-cols-2">
                <ResourceField label="订阅 URL">
                  <ResourceInput
                    value={draft.subscription_url}
                    placeholder="https://example.com/ip-list.txt"
                    onChange={(event) =>
                      setDraft((current) => ({
                        ...current,
                        subscription_url: event.target.value,
                      }))
                    }
                  />
                </ResourceField>
                <ResourceField label="订阅格式">
                  <ResourceSelect
                    value={draft.subscription_format}
                    onChange={(event) =>
                      setDraft((current) => ({
                        ...current,
                        subscription_format: event.target
                          .value as WAFIPGroupSubscriptionFormat,
                      }))
                    }
                  >
                    <option value="text">文本列表</option>
                    <option value="json">JSON</option>
                  </ResourceSelect>
                </ResourceField>
                <ResourceField
                  label="同步间隔（分钟）"
                  hint="最小 5 分钟，默认 1440 分钟。"
                >
                  <ResourceInput
                    type="number"
                    min={5}
                    value={draft.sync_interval_minutes}
                    onChange={(event) =>
                      setDraft((current) => ({
                        ...current,
                        sync_interval_minutes: Number(event.target.value),
                      }))
                    }
                  />
                </ResourceField>
                <ResourceField
                  label="JSON 映射规则"
                  hint="留空表示根数组；示例：data.items[]。文本格式无需填写。"
                >
                  <ResourceInput
                    value={draft.subscription_mapping_rule}
                    disabled={draft.subscription_format !== 'json'}
                    onChange={(event) =>
                      setDraft((current) => ({
                        ...current,
                        subscription_mapping_rule: event.target.value,
                      }))
                    }
                  />
                </ResourceField>
              </div>
            ) : null}

            {draft.type === 'automatic' ? (
              <div className="space-y-4">
                <div className="grid gap-5 xl:grid-cols-2">
                  <ResourceField
                    label="同步间隔（分钟）"
                    hint="定时从请求日志挖掘恶意 IP 的周期。最小 5 分钟，默认 1440 分钟。"
                  >
                    <ResourceInput
                      type="number"
                      min={5}
                      value={draft.sync_interval_minutes}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          sync_interval_minutes: Number(event.target.value),
                        }))
                      }
                    />
                  </ResourceField>
                </div>
                <ResourceField
                  label="预设规则"
                  hint="表达式按单个 IP 的请求日志聚合指标计算。"
                  container="div"
                >
                  <div className="flex flex-wrap gap-2">
                    {automaticPresetRules.map((rule) => (
                      <SecondaryButton
                        key={rule.expr}
                        type="button"
                        onClick={() => {
                          try {
                            setDraft((current) => ({
                              ...current,
                              auto_config_text: appendAutomaticPresetRule(
                                current.auto_config_text,
                                rule,
                              ),
                            }));
                          } catch (error) {
                            setFeedback({
                              tone: 'danger',
                              message: getErrorMessage(error),
                            });
                          }
                        }}
                      >
                        <Plus className="mr-2 h-4 w-4" />
                        {rule.name}
                      </SecondaryButton>
                    ))}
                  </div>
                </ResourceField>
                <ResourceField
                  label="自动配置 JSON"
                  hint="可用字段：request_count、status_404_count、status_404_ratio、ip_host_count、ip_host_ratio。支持 ttl（秒，默认 -1 永久拉黑）。"
                >
                  <ResourceTextarea
                    value={draft.auto_config_text}
                    className="min-h-64 font-mono"
                    onChange={(event) =>
                      setDraft((current) => ({
                        ...current,
                        auto_config_text: event.target.value,
                      }))
                    }
                  />
                </ResourceField>
                {autoTestResult ? (
                  <div className="space-y-3 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-muted)] p-4">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="text-sm font-semibold text-[var(--foreground-primary)]">
                        测试结果
                      </div>
                      <div className="text-xs text-[var(--foreground-secondary)]">
                        回看 {autoTestResult.lookback_minutes} 分钟 · 规则{' '}
                        {autoTestResult.rule_count} 条
                      </div>
                    </div>
                    <InlineMessage
                      tone={
                        autoTestResult.matched_count > 0 ? 'success' : 'info'
                      }
                      message={
                        autoTestResult.matched_count > 0
                          ? `命中 ${autoTestResult.matched_count} 个 IP。`
                          : '当前没有匹配到任何 IP。'
                      }
                    />
                    {autoTestResult.matched_count > 0 ? (
                      <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] p-3">
                        <div className="mb-2 text-xs font-medium text-[var(--foreground-secondary)]">
                          命中 IP 列表
                        </div>
                        <pre className="overflow-x-auto whitespace-pre-wrap break-all text-sm text-[var(--foreground-primary)]">
                          {autoTestResult.matched_ips.join('\n')}
                        </pre>
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </div>
            ) : (
              <ResourceField
                label="IP / IP 段"
                hint={
                  draft.type === 'subscription'
                    ? '订阅同步会覆盖此列表；也可以先手动保存当前内容。'
                    : '支持单个 IP 或 CIDR，每行一个。'
                }
              >
                <ResourceTextarea
                  value={draft.ip_list_text}
                  className="min-h-72 font-mono"
                  placeholder={'203.0.113.10\n198.51.100.0/24'}
                  onChange={(event) =>
                    setDraft((current) => ({
                      ...current,
                      ip_list_text: event.target.value,
                    }))
                  }
                />
              </ResourceField>
            )}

            {selectedGroup ? (
              <div className="flex flex-wrap justify-between gap-3 border-t border-[var(--border-default)] pt-6">
                <div className="text-sm text-[var(--foreground-secondary)]">
                  {selectedGroup.last_sync_status
                    ? `${selectedGroup.last_sync_status}: ${selectedGroup.last_sync_message}`
                    : '尚无同步记录'}
                </div>
                <DangerButton
                  type="button"
                  disabled={deleteMutation.isPending}
                  onClick={() => {
                    if (
                      window.confirm(
                        `确认删除 IP 组 ${selectedGroup.name} 吗？`,
                      )
                    ) {
                      deleteMutation.mutate(selectedGroup.id);
                    }
                  }}
                >
                  <Trash2 className="mr-2 h-4 w-4" />
                  删除
                </DangerButton>
              </div>
            ) : null}
          </div>
        </AppCard>
      </div>
      {selectedGroup?.type === 'automatic' ? (
        <AppModal
          isOpen={isCapturedIPsModalOpen}
          title={`已抓取 IP 列表 - ${selectedGroup.name}`}
          description="展示当前自动挖掘并放入黑名单的 IP 列表及其到期状态。"
          size="lg"
          onClose={() => setIsCapturedIPsModalOpen(false)}
          footer={
            <div className="flex justify-end">
              <SecondaryButton type="button" onClick={() => setIsCapturedIPsModalOpen(false)}>
                关闭
              </SecondaryButton>
            </div>
          }
        >
          <div className="space-y-4">
            {!selectedGroup.ext_ips || selectedGroup.ext_ips.length === 0 ? (
              <EmptyState
                title="暂无抓取记录"
                description="该自动规则暂未抓取或命中任何恶意 IP，点击「立即执行」手动触发抓取。"
              />
            ) : (
              <div className="overflow-hidden rounded-2xl border border-[var(--border-default)]">
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-[var(--border-default)]">
                    <thead className="bg-[var(--surface-muted)]">
                      <tr>
                        <th className="px-4 py-3 text-left text-xs font-semibold text-[var(--foreground-secondary)]">IP 地址</th>
                        <th className="px-4 py-3 text-left text-xs font-semibold text-[var(--foreground-secondary)]">抓取时间</th>
                        <th className="px-4 py-3 text-left text-xs font-semibold text-[var(--foreground-secondary)]">封禁剩余时间</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--border-default)] bg-[var(--surface-elevated)]">
                      {selectedGroup.ext_ips.map((item) => {
                        const autoConfig = selectedGroup.auto_config as { ttl?: number } | undefined;
                        const ttl = autoConfig?.ttl ?? -1;
                        let expireText = '永久';
                        if (ttl > 0) {
                          const capturedDate = new Date(item.captured_at);
                          const expireDate = new Date(capturedDate.getTime() + ttl * 1000);
                          const now = new Date();
                          if (expireDate.getTime() <= now.getTime()) {
                            expireText = '已过期';
                          } else {
                            const diffMs = expireDate.getTime() - now.getTime();
                            const diffMins = Math.round(diffMs / (60 * 1000));
                            if (diffMins < 60) {
                              expireText = `${diffMins} 分钟后`;
                            } else {
                              const diffHours = Math.round(diffMins / 60);
                              expireText = `${diffHours} 小时后`;
                            }
                          }
                        }
                        return (
                          <tr key={item.ip}>
                            <td className="whitespace-nowrap px-4 py-3 text-sm font-medium text-[var(--foreground-primary)] font-mono">{item.ip}</td>
                            <td className="whitespace-nowrap px-4 py-3 text-sm text-[var(--foreground-secondary)]">{new Date(item.captured_at).toLocaleString()}</td>
                            <td className={cn("whitespace-nowrap px-4 py-3 text-sm font-medium", ttl > 0 ? "text-amber-500" : "text-emerald-500")}>{expireText}</td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
            <div className="rounded-2xl bg-[var(--surface-muted)] p-4 text-xs text-[var(--foreground-secondary)] leading-relaxed">
              💡 提示：抓取记录持久化存储在 IP 组的 <code>ext_ips</code> 扩展字段中。未来如需为已抓取到的 IP 记录更多维度的扩展元数据，可直接在此 JSON 对象中添加字段以供扩展。
            </div>
          </div>
        </AppModal>
      ) : null}
    </div>
  );
}
