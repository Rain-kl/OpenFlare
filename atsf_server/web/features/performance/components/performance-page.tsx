'use client';

import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { useAuth } from '@/components/providers/auth-provider';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import { getConfigVersionPreview } from '@/features/config-versions/api/config-versions';
import { getOptions, updateOption } from '@/features/settings/api/settings';
import type { OptionItem } from '@/features/settings/types';
import {
  CodeBlock,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  ResourceTextarea,
  SecondaryButton,
  ToggleField,
} from '@/features/shared/components/resource-primitives';

const settingsQueryKey = ['settings', 'options'] as const;
const previewQueryKey = ['performance', 'preview'] as const;

const defaultPerformanceFields = {
  OpenRestyWorkerProcesses: 'auto',
  OpenRestyWorkerConnections: '4096',
  OpenRestyWorkerRlimitNofile: '65535',
  OpenRestyEventsUse: '',
  OpenRestyEventsMultiAcceptEnabled: false,
  OpenRestyKeepaliveTimeout: '65',
  OpenRestyKeepaliveRequests: '1000',
  OpenRestyClientHeaderTimeout: '15',
  OpenRestyClientBodyTimeout: '15',
  OpenRestySendTimeout: '30',
  OpenRestyProxyConnectTimeout: '5',
  OpenRestyProxySendTimeout: '60',
  OpenRestyProxyReadTimeout: '60',
  OpenRestyProxyBufferingEnabled: true,
  OpenRestyProxyBuffers: '16 16k',
  OpenRestyProxyBufferSize: '8k',
  OpenRestyProxyBusyBuffersSize: '64k',
  OpenRestyGzipEnabled: true,
  OpenRestyGzipMinLength: '1024',
  OpenRestyGzipCompLevel: '5',
  OpenRestyCacheEnabled: false,
  OpenRestyCachePath: '',
  OpenRestyCacheLevels: '1:2',
  OpenRestyCacheInactive: '30m',
  OpenRestyCacheMaxSize: '1g',
  OpenRestyCacheKeyTemplate: '$scheme$proxy_host$request_uri',
  OpenRestyCacheLockEnabled: true,
  OpenRestyCacheLockTimeout: '5s',
  OpenRestyCacheUseStale:
    'error timeout updating http_500 http_502 http_503 http_504',
};

type PerformanceTab = 'settings' | 'editor';

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function optionsToMap(options: OptionItem[] | undefined) {
  return (options ?? []).reduce<Record<string, string>>(
    (accumulator, option) => {
      accumulator[option.key] = option.value;
      return accumulator;
    },
    {},
  );
}

function toBoolean(value: string | undefined, fallback: boolean) {
  if (value === undefined) {
    return fallback;
  }

  return value === 'true';
}

function isPositiveInteger(value: string) {
  const parsed = Number.parseInt(value, 10);
  return !Number.isNaN(parsed) && parsed > 0;
}

function isSizeValue(value: string) {
  return /^\d+[kKmMgG]?$/.test(value.trim());
}

function isProxyBuffersValue(value: string) {
  return /^\d+\s+\d+[kKmMgG]?$/.test(value.trim());
}

function isDurationToken(value: string) {
  return /^\d+[smhdwSMHDW]$/.test(value.trim());
}

function isCacheLevelsValue(value: string) {
  return /^\d{1,2}(?::\d{1,2}){0,2}$/.test(value.trim());
}

export function PerformancePage() {
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState<PerformanceTab>('settings');
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [performanceFields, setPerformanceFields] = useState(
    defaultPerformanceFields,
  );
  const [templateContent, setTemplateContent] = useState('');

  const isRoot = (user?.role ?? 0) >= 100;

  const optionsQuery = useQuery({
    queryKey: settingsQueryKey,
    queryFn: getOptions,
    enabled: isRoot,
  });

  const previewQuery = useQuery({
    queryKey: previewQueryKey,
    queryFn: getConfigVersionPreview,
    enabled: isRoot,
  });

  useEffect(() => {
    if (!optionsQuery.data) {
      return;
    }

    const optionMap = optionsToMap(optionsQuery.data);
    setPerformanceFields({
      OpenRestyWorkerProcesses: optionMap.OpenRestyWorkerProcesses ?? 'auto',
      OpenRestyWorkerConnections:
        optionMap.OpenRestyWorkerConnections ?? '4096',
      OpenRestyWorkerRlimitNofile:
        optionMap.OpenRestyWorkerRlimitNofile ?? '65535',
      OpenRestyEventsUse: optionMap.OpenRestyEventsUse ?? '',
      OpenRestyEventsMultiAcceptEnabled: toBoolean(
        optionMap.OpenRestyEventsMultiAcceptEnabled,
        false,
      ),
      OpenRestyKeepaliveTimeout: optionMap.OpenRestyKeepaliveTimeout ?? '65',
      OpenRestyKeepaliveRequests:
        optionMap.OpenRestyKeepaliveRequests ?? '1000',
      OpenRestyClientHeaderTimeout:
        optionMap.OpenRestyClientHeaderTimeout ?? '15',
      OpenRestyClientBodyTimeout: optionMap.OpenRestyClientBodyTimeout ?? '15',
      OpenRestySendTimeout: optionMap.OpenRestySendTimeout ?? '30',
      OpenRestyProxyConnectTimeout:
        optionMap.OpenRestyProxyConnectTimeout ?? '5',
      OpenRestyProxySendTimeout: optionMap.OpenRestyProxySendTimeout ?? '60',
      OpenRestyProxyReadTimeout: optionMap.OpenRestyProxyReadTimeout ?? '60',
      OpenRestyProxyBufferingEnabled: toBoolean(
        optionMap.OpenRestyProxyBufferingEnabled,
        true,
      ),
      OpenRestyProxyBuffers: optionMap.OpenRestyProxyBuffers ?? '16 16k',
      OpenRestyProxyBufferSize: optionMap.OpenRestyProxyBufferSize ?? '8k',
      OpenRestyProxyBusyBuffersSize:
        optionMap.OpenRestyProxyBusyBuffersSize ?? '64k',
      OpenRestyGzipEnabled: toBoolean(optionMap.OpenRestyGzipEnabled, true),
      OpenRestyGzipMinLength: optionMap.OpenRestyGzipMinLength ?? '1024',
      OpenRestyGzipCompLevel: optionMap.OpenRestyGzipCompLevel ?? '5',
      OpenRestyCacheEnabled: toBoolean(optionMap.OpenRestyCacheEnabled, false),
      OpenRestyCachePath: optionMap.OpenRestyCachePath ?? '',
      OpenRestyCacheLevels: optionMap.OpenRestyCacheLevels ?? '1:2',
      OpenRestyCacheInactive: optionMap.OpenRestyCacheInactive ?? '30m',
      OpenRestyCacheMaxSize: optionMap.OpenRestyCacheMaxSize ?? '1g',
      OpenRestyCacheKeyTemplate:
        optionMap.OpenRestyCacheKeyTemplate ?? '$scheme$proxy_host$request_uri',
      OpenRestyCacheLockEnabled: toBoolean(
        optionMap.OpenRestyCacheLockEnabled,
        true,
      ),
      OpenRestyCacheLockTimeout: optionMap.OpenRestyCacheLockTimeout ?? '5s',
      OpenRestyCacheUseStale:
        optionMap.OpenRestyCacheUseStale ??
        'error timeout updating http_500 http_502 http_503 http_504',
    });
    setTemplateContent(optionMap.OpenRestyMainConfigTemplate ?? '');
  }, [optionsQuery.data]);

  const runBusyAction = async (key: string, action: () => Promise<void>) => {
    setBusyKey(key);
    setFeedback(null);

    try {
      await action();
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    } finally {
      setBusyKey(null);
    }
  };

  const saveOptionEntries = async (
    entries: Array<[string, string]>,
    successMessage: string,
  ) => {
    for (const [key, value] of entries) {
      await updateOption(key, value);
    }

    await Promise.all([
      queryClient.invalidateQueries({ queryKey: settingsQueryKey }),
      queryClient.invalidateQueries({ queryKey: previewQueryKey }),
      queryClient.invalidateQueries({ queryKey: ['config-versions'] }),
    ]);
    setFeedback({ tone: 'success', message: successMessage });
  };

  const handleRuntimeSave = () => {
    void runBusyAction('performance-runtime', async () => {
      if (
        performanceFields.OpenRestyWorkerProcesses !== 'auto' &&
        !isPositiveInteger(performanceFields.OpenRestyWorkerProcesses)
      ) {
        throw new Error('worker_processes 必须为 auto 或大于 0 的整数。');
      }

      const integerFields = [
        ['worker_connections', performanceFields.OpenRestyWorkerConnections],
        ['worker_rlimit_nofile', performanceFields.OpenRestyWorkerRlimitNofile],
        ['keepalive_timeout', performanceFields.OpenRestyKeepaliveTimeout],
        ['keepalive_requests', performanceFields.OpenRestyKeepaliveRequests],
        [
          'client_header_timeout',
          performanceFields.OpenRestyClientHeaderTimeout,
        ],
        ['client_body_timeout', performanceFields.OpenRestyClientBodyTimeout],
        ['send_timeout', performanceFields.OpenRestySendTimeout],
      ] as const;

      for (const [label, value] of integerFields) {
        if (!isPositiveInteger(value)) {
          throw new Error(`${label} 必须为大于 0 的整数。`);
        }
      }

      await saveOptionEntries(
        [
          [
            'OpenRestyWorkerProcesses',
            performanceFields.OpenRestyWorkerProcesses.trim(),
          ],
          [
            'OpenRestyWorkerConnections',
            performanceFields.OpenRestyWorkerConnections.trim(),
          ],
          [
            'OpenRestyWorkerRlimitNofile',
            performanceFields.OpenRestyWorkerRlimitNofile.trim(),
          ],
          ['OpenRestyEventsUse', performanceFields.OpenRestyEventsUse.trim()],
          [
            'OpenRestyEventsMultiAcceptEnabled',
            String(performanceFields.OpenRestyEventsMultiAcceptEnabled),
          ],
          [
            'OpenRestyKeepaliveTimeout',
            performanceFields.OpenRestyKeepaliveTimeout.trim(),
          ],
          [
            'OpenRestyKeepaliveRequests',
            performanceFields.OpenRestyKeepaliveRequests.trim(),
          ],
          [
            'OpenRestyClientHeaderTimeout',
            performanceFields.OpenRestyClientHeaderTimeout.trim(),
          ],
          [
            'OpenRestyClientBodyTimeout',
            performanceFields.OpenRestyClientBodyTimeout.trim(),
          ],
          [
            'OpenRestySendTimeout',
            performanceFields.OpenRestySendTimeout.trim(),
          ],
        ],
        'OpenResty 连接与事件参数已保存。',
      );
    });
  };

  const handleProxySave = () => {
    void runBusyAction('performance-proxy', async () => {
      const timeoutFields = [
        performanceFields.OpenRestyProxyConnectTimeout,
        performanceFields.OpenRestyProxySendTimeout,
        performanceFields.OpenRestyProxyReadTimeout,
      ];

      for (const value of timeoutFields) {
        if (!isPositiveInteger(value)) {
          throw new Error('代理超时参数必须为大于 0 的整数秒。');
        }
      }
      if (!isProxyBuffersValue(performanceFields.OpenRestyProxyBuffers)) {
        throw new Error('proxy_buffers 格式必须类似 "16 16k"。');
      }
      if (
        !isSizeValue(performanceFields.OpenRestyProxyBufferSize) ||
        !isSizeValue(performanceFields.OpenRestyProxyBusyBuffersSize)
      ) {
        throw new Error('缓冲大小必须为整数或带 k/m/g 单位的值。');
      }

      await saveOptionEntries(
        [
          [
            'OpenRestyProxyConnectTimeout',
            performanceFields.OpenRestyProxyConnectTimeout.trim(),
          ],
          [
            'OpenRestyProxySendTimeout',
            performanceFields.OpenRestyProxySendTimeout.trim(),
          ],
          [
            'OpenRestyProxyReadTimeout',
            performanceFields.OpenRestyProxyReadTimeout.trim(),
          ],
          [
            'OpenRestyProxyBufferingEnabled',
            String(performanceFields.OpenRestyProxyBufferingEnabled),
          ],
          [
            'OpenRestyProxyBuffers',
            performanceFields.OpenRestyProxyBuffers.trim(),
          ],
          [
            'OpenRestyProxyBufferSize',
            performanceFields.OpenRestyProxyBufferSize.trim(),
          ],
          [
            'OpenRestyProxyBusyBuffersSize',
            performanceFields.OpenRestyProxyBusyBuffersSize.trim(),
          ],
        ],
        'OpenResty 反代缓冲参数已保存。',
      );
    });
  };

  const handleCacheSave = () => {
    void runBusyAction('performance-cache', async () => {
      if (!isPositiveInteger(performanceFields.OpenRestyGzipMinLength)) {
        throw new Error('gzip_min_length 必须为大于 0 的整数。');
      }
      const gzipLevel = Number.parseInt(
        performanceFields.OpenRestyGzipCompLevel,
        10,
      );
      if (Number.isNaN(gzipLevel) || gzipLevel < 1 || gzipLevel > 9) {
        throw new Error('gzip_comp_level 必须在 1 到 9 之间。');
      }
      if (performanceFields.OpenRestyCacheEnabled) {
        if (!performanceFields.OpenRestyCachePath.trim()) {
          throw new Error('启用缓存时必须填写 proxy_cache_path 目录。');
        }
        if (
          !isCacheLevelsValue(performanceFields.OpenRestyCacheLevels) ||
          !isDurationToken(performanceFields.OpenRestyCacheInactive) ||
          !isSizeValue(performanceFields.OpenRestyCacheMaxSize) ||
          !isDurationToken(performanceFields.OpenRestyCacheLockTimeout)
        ) {
          throw new Error(
            '缓存 levels、inactive、max_size 或 lock_timeout 格式不合法。',
          );
        }
        if (!performanceFields.OpenRestyCacheKeyTemplate.trim()) {
          throw new Error('启用缓存时必须填写缓存 Key 模板。');
        }
      }

      await saveOptionEntries(
        [
          [
            'OpenRestyGzipEnabled',
            String(performanceFields.OpenRestyGzipEnabled),
          ],
          [
            'OpenRestyGzipMinLength',
            performanceFields.OpenRestyGzipMinLength.trim(),
          ],
          [
            'OpenRestyGzipCompLevel',
            performanceFields.OpenRestyGzipCompLevel.trim(),
          ],
          ['OpenRestyCachePath', performanceFields.OpenRestyCachePath.trim()],
          [
            'OpenRestyCacheLevels',
            performanceFields.OpenRestyCacheLevels.trim(),
          ],
          [
            'OpenRestyCacheInactive',
            performanceFields.OpenRestyCacheInactive.trim(),
          ],
          [
            'OpenRestyCacheMaxSize',
            performanceFields.OpenRestyCacheMaxSize.trim(),
          ],
          [
            'OpenRestyCacheKeyTemplate',
            performanceFields.OpenRestyCacheKeyTemplate.trim(),
          ],
          [
            'OpenRestyCacheLockEnabled',
            String(performanceFields.OpenRestyCacheLockEnabled),
          ],
          [
            'OpenRestyCacheLockTimeout',
            performanceFields.OpenRestyCacheLockTimeout.trim(),
          ],
          [
            'OpenRestyCacheUseStale',
            performanceFields.OpenRestyCacheUseStale.trim(),
          ],
          [
            'OpenRestyCacheEnabled',
            String(performanceFields.OpenRestyCacheEnabled),
          ],
        ],
        'OpenResty 压缩与缓存参数已保存。',
      );
    });
  };

  const handleTemplateSave = () => {
    void runBusyAction('performance-template', async () => {
      await saveOptionEntries(
        [['OpenRestyMainConfigTemplate', templateContent]],
        'OpenResty 主配置模板已保存。',
      );
    });
  };

  if (!isRoot) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="性能"
          description="集中管理 OpenResty 性能参数和主配置模板。"
        />
        <EmptyState
          title="权限不足"
          description="只有超级管理员可以访问性能设置。"
        />
      </div>
    );
  }

  if (optionsQuery.isLoading || previewQuery.isLoading) {
    return <LoadingState />;
  }

  if (optionsQuery.isError) {
    return (
      <ErrorState
        title="性能设置加载失败"
        description={getErrorMessage(optionsQuery.error)}
      />
    );
  }

  if (previewQuery.isError) {
    return (
      <ErrorState
        title="性能预览加载失败"
        description={getErrorMessage(previewQuery.error)}
      />
    );
  }

  const preview = previewQuery.data;
  if (!preview) {
    return (
      <EmptyState
        title="性能预览不可用"
        description="当前未获取到 OpenResty 配置预览。"
      />
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="性能"
        description="从设置页拆出的 OpenResty 专属工作台。这里统一维护结构化性能参数和主配置模板。"
      />

      {feedback ? (
        <InlineMessage tone={feedback.tone} message={feedback.message} />
      ) : null}

      <div className="flex flex-wrap gap-3">
        {[
          {
            key: 'settings' as const,
            label: '设置',
            description: '维护连接、缓冲、压缩与缓存参数。',
          },
          {
            key: 'editor' as const,
            label: '编辑',
            description: '编辑 nginx.conf 模板并查看当前渲染结果。',
          },
        ].map((tab) => (
          <button
            key={tab.key}
            type="button"
            onClick={() => setActiveTab(tab.key)}
            className={[
              'rounded-2xl border px-4 py-3 text-left transition',
              activeTab === tab.key
                ? 'border-[var(--border-strong)] bg-[var(--accent-soft)] text-[var(--foreground-primary)]'
                : 'border-[var(--border-default)] bg-[var(--surface-muted)] text-[var(--foreground-secondary)] hover:border-[var(--border-strong)] hover:text-[var(--foreground-primary)]',
            ].join(' ')}
          >
            <p className="text-sm font-semibold">{tab.label}</p>
            <p className="mt-1 text-xs leading-5 text-inherit/80">
              {tab.description}
            </p>
          </button>
        ))}
      </div>

      {activeTab === 'settings' ? (
        <div className="space-y-6">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                主配置校验
              </p>
              <div className="mt-2">
                <StatusBadge label="发布链路受管" variant="info" />
              </div>
            </div>
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                当前预览规则数
              </p>
              <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                {preview.route_count} 条
              </p>
            </div>
            <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
              <p className="text-xs tracking-[0.2em] text-[var(--foreground-muted)] uppercase">
                当前模板
              </p>
              <p className="mt-2 text-sm text-[var(--foreground-primary)]">
                受控模板 + 占位符渲染
              </p>
            </div>
          </div>

          <AppCard
            title="OpenResty 连接与事件"
            action={
              <PrimaryButton
                type="button"
                onClick={handleRuntimeSave}
                disabled={busyKey === 'performance-runtime'}
              >
                {busyKey === 'performance-runtime'
                  ? '保存中...'
                  : '保存运行调优'}
              </PrimaryButton>
            }
          >
            <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
              <ResourceField label="worker_processes">
                <ResourceInput
                  value={performanceFields.OpenRestyWorkerProcesses}
                  onChange={(event) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestyWorkerProcesses: event.target.value,
                    }))
                  }
                  placeholder="auto"
                />
              </ResourceField>
              <ResourceField label="worker_connections">
                <ResourceInput
                  type="number"
                  value={performanceFields.OpenRestyWorkerConnections}
                  onChange={(event) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestyWorkerConnections: event.target.value,
                    }))
                  }
                />
              </ResourceField>
              <ResourceField label="worker_rlimit_nofile">
                <ResourceInput
                  type="number"
                  value={performanceFields.OpenRestyWorkerRlimitNofile}
                  onChange={(event) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestyWorkerRlimitNofile: event.target.value,
                    }))
                  }
                />
              </ResourceField>
              <ResourceField label="events use" hint="留空表示不显式渲染。">
                <ResourceSelect
                  value={performanceFields.OpenRestyEventsUse}
                  onChange={(event) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestyEventsUse: event.target.value,
                    }))
                  }
                >
                  <option value="">默认</option>
                  <option value="epoll">epoll</option>
                  <option value="kqueue">kqueue</option>
                  <option value="poll">poll</option>
                  <option value="select">select</option>
                </ResourceSelect>
              </ResourceField>
              <ToggleField
                label="multi_accept"
                description=""
                checked={performanceFields.OpenRestyEventsMultiAcceptEnabled}
                onChange={(checked) =>
                  setPerformanceFields((previous) => ({
                    ...previous,
                    OpenRestyEventsMultiAcceptEnabled: checked,
                  }))
                }
              />
              <ResourceField label="keepalive_timeout (秒)">
                <ResourceInput
                  type="number"
                  value={performanceFields.OpenRestyKeepaliveTimeout}
                  onChange={(event) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestyKeepaliveTimeout: event.target.value,
                    }))
                  }
                />
              </ResourceField>
              <ResourceField label="keepalive_requests">
                <ResourceInput
                  type="number"
                  value={performanceFields.OpenRestyKeepaliveRequests}
                  onChange={(event) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestyKeepaliveRequests: event.target.value,
                    }))
                  }
                />
              </ResourceField>
              <ResourceField label="client_header_timeout (秒)">
                <ResourceInput
                  type="number"
                  value={performanceFields.OpenRestyClientHeaderTimeout}
                  onChange={(event) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestyClientHeaderTimeout: event.target.value,
                    }))
                  }
                />
              </ResourceField>
              <ResourceField label="client_body_timeout (秒)">
                <ResourceInput
                  type="number"
                  value={performanceFields.OpenRestyClientBodyTimeout}
                  onChange={(event) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestyClientBodyTimeout: event.target.value,
                    }))
                  }
                />
              </ResourceField>
              <ResourceField label="send_timeout (秒)">
                <ResourceInput
                  type="number"
                  value={performanceFields.OpenRestySendTimeout}
                  onChange={(event) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestySendTimeout: event.target.value,
                    }))
                  }
                />
              </ResourceField>
            </div>
          </AppCard>

          <div className="grid gap-6 xl:grid-cols-[1fr_1fr]">
            <AppCard
              title="OpenResty 反代缓冲与超时"
              description="用于控制 upstream 连接、发送、读取超时，以及常用代理缓冲参数。"
              action={
                <PrimaryButton
                  type="button"
                  onClick={handleProxySave}
                  disabled={busyKey === 'performance-proxy'}
                >
                  {busyKey === 'performance-proxy'
                    ? '保存中...'
                    : '保存代理调优'}
                </PrimaryButton>
              }
            >
              <div className="grid gap-5 md:grid-cols-2">
                <ResourceField label="proxy_connect_timeout (秒)">
                  <ResourceInput
                    type="number"
                    value={performanceFields.OpenRestyProxyConnectTimeout}
                    onChange={(event) =>
                      setPerformanceFields((previous) => ({
                        ...previous,
                        OpenRestyProxyConnectTimeout: event.target.value,
                      }))
                    }
                  />
                </ResourceField>
                <ResourceField label="proxy_send_timeout (秒)">
                  <ResourceInput
                    type="number"
                    value={performanceFields.OpenRestyProxySendTimeout}
                    onChange={(event) =>
                      setPerformanceFields((previous) => ({
                        ...previous,
                        OpenRestyProxySendTimeout: event.target.value,
                      }))
                    }
                  />
                </ResourceField>
                <ResourceField label="proxy_read_timeout (秒)">
                  <ResourceInput
                    type="number"
                    value={performanceFields.OpenRestyProxyReadTimeout}
                    onChange={(event) =>
                      setPerformanceFields((previous) => ({
                        ...previous,
                        OpenRestyProxyReadTimeout: event.target.value,
                      }))
                    }
                  />
                </ResourceField>
                <ToggleField
                  label="proxy_buffering"
                  description="是否启用 proxy_buffering。"
                  checked={performanceFields.OpenRestyProxyBufferingEnabled}
                  onChange={(checked) =>
                    setPerformanceFields((previous) => ({
                      ...previous,
                      OpenRestyProxyBufferingEnabled: checked,
                    }))
                  }
                />
                <ResourceField label="proxy_buffers">
                  <ResourceInput
                    value={performanceFields.OpenRestyProxyBuffers}
                    onChange={(event) =>
                      setPerformanceFields((previous) => ({
                        ...previous,
                        OpenRestyProxyBuffers: event.target.value,
                      }))
                    }
                    placeholder="16 16k"
                  />
                </ResourceField>
                <ResourceField label="proxy_buffer_size">
                  <ResourceInput
                    value={performanceFields.OpenRestyProxyBufferSize}
                    onChange={(event) =>
                      setPerformanceFields((previous) => ({
                        ...previous,
                        OpenRestyProxyBufferSize: event.target.value,
                      }))
                    }
                    placeholder="8k"
                  />
                </ResourceField>
                <ResourceField label="proxy_busy_buffers_size">
                  <ResourceInput
                    value={performanceFields.OpenRestyProxyBusyBuffersSize}
                    onChange={(event) =>
                      setPerformanceFields((previous) => ({
                        ...previous,
                        OpenRestyProxyBusyBuffersSize: event.target.value,
                      }))
                    }
                    placeholder="64k"
                  />
                </ResourceField>
              </div>
            </AppCard>

            <AppCard
              title="OpenResty 压缩与缓存"
              description="缓存能力仍限定在单节点反代优化场景，不扩展为独立缓存产品。"
              action={
                <PrimaryButton
                  type="button"
                  onClick={handleCacheSave}
                  disabled={busyKey === 'performance-cache'}
                >
                  {busyKey === 'performance-cache'
                    ? '保存中...'
                    : '保存压缩与缓存'}
                </PrimaryButton>
              }
            >
              <div className="space-y-5">
                <div className="grid gap-5 md:grid-cols-2">
                  <ToggleField
                    label="gzip"
                    description="是否启用 gzip on。"
                    checked={performanceFields.OpenRestyGzipEnabled}
                    onChange={(checked) =>
                      setPerformanceFields((previous) => ({
                        ...previous,
                        OpenRestyGzipEnabled: checked,
                      }))
                    }
                  />
                  <ToggleField
                    label="proxy cache"
                    description="是否启用单节点代理缓存。"
                    checked={performanceFields.OpenRestyCacheEnabled}
                    onChange={(checked) =>
                      setPerformanceFields((previous) => ({
                        ...previous,
                        OpenRestyCacheEnabled: checked,
                      }))
                    }
                  />
                  <ResourceField label="gzip_min_length">
                    <ResourceInput
                      type="number"
                      value={performanceFields.OpenRestyGzipMinLength}
                      onChange={(event) =>
                        setPerformanceFields((previous) => ({
                          ...previous,
                          OpenRestyGzipMinLength: event.target.value,
                        }))
                      }
                    />
                  </ResourceField>
                  <ResourceField label="gzip_comp_level">
                    <ResourceInput
                      type="number"
                      min="1"
                      max="9"
                      value={performanceFields.OpenRestyGzipCompLevel}
                      onChange={(event) =>
                        setPerformanceFields((previous) => ({
                          ...previous,
                          OpenRestyGzipCompLevel: event.target.value,
                        }))
                      }
                    />
                  </ResourceField>
                </div>
                <div className="grid gap-5 md:grid-cols-2">
                  <ResourceField label="proxy_cache_path">
                    <ResourceInput
                      value={performanceFields.OpenRestyCachePath}
                      onChange={(event) =>
                        setPerformanceFields((previous) => ({
                          ...previous,
                          OpenRestyCachePath: event.target.value,
                        }))
                      }
                      placeholder="/var/cache/openresty/atsflare"
                    />
                  </ResourceField>
                  <ResourceField label="levels">
                    <ResourceInput
                      value={performanceFields.OpenRestyCacheLevels}
                      onChange={(event) =>
                        setPerformanceFields((previous) => ({
                          ...previous,
                          OpenRestyCacheLevels: event.target.value,
                        }))
                      }
                      placeholder="1:2"
                    />
                  </ResourceField>
                  <ResourceField label="inactive">
                    <ResourceInput
                      value={performanceFields.OpenRestyCacheInactive}
                      onChange={(event) =>
                        setPerformanceFields((previous) => ({
                          ...previous,
                          OpenRestyCacheInactive: event.target.value,
                        }))
                      }
                      placeholder="30m"
                    />
                  </ResourceField>
                  <ResourceField label="max_size">
                    <ResourceInput
                      value={performanceFields.OpenRestyCacheMaxSize}
                      onChange={(event) =>
                        setPerformanceFields((previous) => ({
                          ...previous,
                          OpenRestyCacheMaxSize: event.target.value,
                        }))
                      }
                      placeholder="1g"
                    />
                  </ResourceField>
                  <ResourceField label="cache key template">
                    <ResourceInput
                      value={performanceFields.OpenRestyCacheKeyTemplate}
                      onChange={(event) =>
                        setPerformanceFields((previous) => ({
                          ...previous,
                          OpenRestyCacheKeyTemplate: event.target.value,
                        }))
                      }
                    />
                  </ResourceField>
                  <ToggleField
                    label="proxy_cache_lock"
                    description="是否启用 proxy_cache_lock。"
                    checked={performanceFields.OpenRestyCacheLockEnabled}
                    onChange={(checked) =>
                      setPerformanceFields((previous) => ({
                        ...previous,
                        OpenRestyCacheLockEnabled: checked,
                      }))
                    }
                  />
                  <ResourceField label="proxy_cache_lock_timeout">
                    <ResourceInput
                      value={performanceFields.OpenRestyCacheLockTimeout}
                      onChange={(event) =>
                        setPerformanceFields((previous) => ({
                          ...previous,
                          OpenRestyCacheLockTimeout: event.target.value,
                        }))
                      }
                      placeholder="5s"
                    />
                  </ResourceField>
                  <ResourceField label="proxy_cache_use_stale">
                    <ResourceInput
                      value={performanceFields.OpenRestyCacheUseStale}
                      onChange={(event) =>
                        setPerformanceFields((previous) => ({
                          ...previous,
                          OpenRestyCacheUseStale: event.target.value,
                        }))
                      }
                    />
                  </ResourceField>
                </div>
              </div>
            </AppCard>
          </div>
        </div>
      ) : (
        <div className="space-y-6">
          <AppCard
            title="nginx.conf 模板编辑"
            description="编辑 ATSFlare 管理的主配置模板。系统占位符必须保留，保存后会进入统一发布链路。"
            action={
              <div className="flex flex-wrap gap-2">
                <SecondaryButton
                  type="button"
                  onClick={() =>
                    void queryClient.invalidateQueries({
                      queryKey: previewQueryKey,
                    })
                  }
                >
                  刷新预览
                </SecondaryButton>
                <PrimaryButton
                  type="button"
                  onClick={handleTemplateSave}
                  disabled={busyKey === 'performance-template'}
                >
                  {busyKey === 'performance-template'
                    ? '保存中...'
                    : '保存模板'}
                </PrimaryButton>
              </div>
            }
          >
            <div className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
              <div className="space-y-4">
                <ResourceField
                  label="模板原文"
                  hint="请保留系统占位符，例如 {{OpenRestyRouteConfigInclude}} 和各项 OpenResty 参数占位符。"
                >
                  <ResourceTextarea
                    value={templateContent}
                    onChange={(event) => setTemplateContent(event.target.value)}
                    className="min-h-[560px] font-mono text-xs leading-6"
                    placeholder="请输入受控 nginx.conf 模板"
                  />
                </ResourceField>
              </div>

              <div className="space-y-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                      当前渲染预览
                    </p>
                    <p className="mt-1 text-xs text-[var(--foreground-secondary)]">
                      基于当前结构化参数和模板生成的主配置文件。
                    </p>
                  </div>
                  <StatusBadge
                    label={`${preview.route_count} 条规则`}
                    variant="info"
                  />
                </div>
                <CodeBlock className="max-h-[560px] overflow-auto text-xs leading-6 whitespace-pre-wrap">
                  {preview.main_config}
                </CodeBlock>
              </div>
            </div>
          </AppCard>
        </div>
      )}
    </div>
  );
}
