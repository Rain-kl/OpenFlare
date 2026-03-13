'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useFieldArray, useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { PageHeader } from '@/components/layout/page-header';
import { AppModal } from '@/components/ui/app-modal';
import { AppCard } from '@/components/ui/app-card';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  getConfigVersionDiff,
  publishConfigVersion,
} from '@/features/config-versions/api/config-versions';
import {
  createProxyRoute,
  deleteProxyRoute,
  getProxyRoutes,
  getTlsCertificates,
  matchManagedDomainCertificate,
  updateProxyRoute,
} from '@/features/proxy-routes/api/proxy-routes';
import type {
  ManagedDomainMatchResult,
  ProxyRouteCustomHeader,
  ProxyRouteItem,
  ProxyRouteMutationPayload,
  TlsCertificateItem,
} from '@/features/proxy-routes/types';
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
import { formatDateTime } from '@/lib/utils/date';

const customHeaderSchema = z.object({
  key: z.string(),
  value: z.string(),
});

const proxyRouteSchema = z
  .object({
    domain: z.string().trim().min(1, '请输入域名'),
    origin_url: z
      .string()
      .trim()
      .min(1, '请输入源站地址')
      .refine(
        (value) => /^https?:\/\//.test(value),
        '源站地址必须以 http:// 或 https:// 开头',
      )
      .refine((value) => {
        try {
          new URL(value);
          return true;
        } catch {
          return false;
        }
      }, '请输入合法的源站地址'),
    enabled: z.boolean(),
    enable_https: z.boolean(),
    cert_id: z.string(),
    redirect_http: z.boolean(),
    custom_headers: z.array(customHeaderSchema).min(1),
    remark: z.string().max(255, '备注不能超过 255 个字符'),
  })
  .superRefine((value, context) => {
    if (value.enable_https && !value.cert_id) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['cert_id'],
        message: '启用 HTTPS 时必须选择证书',
      });
    }

    value.custom_headers.forEach((header, index) => {
      const key = header.key.trim();
      const headerValue = header.value.trim();

      if (!key && !headerValue) {
        return;
      }

      if (!key) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['custom_headers', index, 'key'],
          message: '请求头名称不能为空',
        });
      }

      if (key && !/^[A-Za-z0-9_-]+$/.test(key)) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['custom_headers', index, 'key'],
          message: '请求头名称仅支持字母、数字、下划线和中划线',
        });
      }

      if (/\r|\n/.test(key) || /\r|\n/.test(headerValue)) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['custom_headers', index, 'value'],
          message: '请求头不能包含换行',
        });
      }
    });
  });

type ProxyRouteFormValues = z.infer<typeof proxyRouteSchema>;

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

const defaultValues: ProxyRouteFormValues = {
  domain: '',
  origin_url: '',
  enabled: true,
  enable_https: false,
  cert_id: '',
  redirect_http: false,
  custom_headers: [{ key: '', value: '' }],
  remark: '',
};

const routesQueryKey = ['proxy-routes'];
const certificatesQueryKey = ['tls-certificates'];
const versionsQueryKey = ['config-versions'];

function hasConfigChanges(diff: {
  active_version?: string;
  added_domains: string[];
  removed_domains: string[];
  modified_domains: string[];
  main_config_changed: boolean;
  changed_option_keys: string[];
}) {
  return (
    diff.added_domains.length > 0 ||
    diff.removed_domains.length > 0 ||
    diff.modified_domains.length > 0 ||
    diff.main_config_changed ||
    diff.changed_option_keys.length > 0 ||
    !diff.active_version
  );
}

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function parseCustomHeaders(rawValue: string) {
  if (!rawValue) {
    return [] as ProxyRouteCustomHeader[];
  }

  try {
    const parsed = JSON.parse(rawValue) as ProxyRouteCustomHeader[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function buildCertificateLabel(certificate: TlsCertificateItem) {
  return certificate.not_after
    ? `${certificate.name}（到期：${formatDateTime(certificate.not_after)}）`
    : certificate.name;
}

function toPayload(values: ProxyRouteFormValues): ProxyRouteMutationPayload {
  return {
    domain: values.domain.trim(),
    origin_url: values.origin_url.trim(),
    enabled: values.enabled,
    enable_https: values.enable_https,
    cert_id:
      values.enable_https && values.cert_id ? Number(values.cert_id) : null,
    redirect_http: values.enable_https ? values.redirect_http : false,
    custom_headers: values.custom_headers
      .map((item) => ({ key: item.key.trim(), value: item.value.trim() }))
      .filter((item) => item.key || item.value),
    remark: values.remark.trim(),
  };
}

function toFormValues(route: ProxyRouteItem): ProxyRouteFormValues {
  const headers = parseCustomHeaders(route.custom_headers);

  return {
    domain: route.domain,
    origin_url: route.origin_url,
    enabled: route.enabled,
    enable_https: route.enable_https,
    cert_id: route.cert_id ? String(route.cert_id) : '',
    redirect_http: route.redirect_http,
    custom_headers: headers.length > 0 ? headers : [{ key: '', value: '' }],
    remark: route.remark || '',
  };
}

function getMatchMessage(
  matchResult: ManagedDomainMatchResult | null,
  isMatching: boolean,
  domain: string,
  enabled: boolean,
) {
  if (!enabled) {
    return '启用 HTTPS 后，系统会根据输入域名尝试自动匹配托管证书。';
  }

  if (isMatching) {
    return '正在按域名自动匹配托管证书...';
  }

  if (!domain.trim()) {
    return '输入域名后会自动匹配证书，并优先推荐精确匹配规则。';
  }

  if (matchResult?.matched && matchResult.candidate) {
    return `已匹配${matchResult.candidate.match_type === 'exact' ? '精确' : '通配符'}规则 ${matchResult.candidate.domain}，推荐证书：${matchResult.candidate.certificate_name}`;
  }

  return '未找到匹配证书，可继续手动选择。';
}

export function ProxyRoutesPage() {
  const queryClient = useQueryClient();
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [editingRouteId, setEditingRouteId] = useState<number | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [matchResult, setMatchResult] =
    useState<ManagedDomainMatchResult | null>(null);
  const [isMatching, setIsMatching] = useState(false);

  const form = useForm<ProxyRouteFormValues>({
    resolver: zodResolver(proxyRouteSchema),
    defaultValues,
  });

  const { fields, append, remove, replace } = useFieldArray({
    control: form.control,
    name: 'custom_headers',
  });

  const watchedDomain = useWatch({ control: form.control, name: 'domain' });
  const watchedEnabled = useWatch({ control: form.control, name: 'enabled' });
  const watchedEnableHttps = useWatch({
    control: form.control,
    name: 'enable_https',
  });
  const watchedRedirectHttp = useWatch({
    control: form.control,
    name: 'redirect_http',
  });
  const watchedCertId = useWatch({ control: form.control, name: 'cert_id' });

  const routesQuery = useQuery({
    queryKey: routesQueryKey,
    queryFn: getProxyRoutes,
  });

  const certificatesQuery = useQuery({
    queryKey: certificatesQueryKey,
    queryFn: getTlsCertificates,
  });

  const saveMutation = useMutation({
    mutationFn: async (values: ProxyRouteFormValues) => {
      const payload = toPayload(values);
      return editingRouteId
        ? updateProxyRoute(editingRouteId, payload)
        : createProxyRoute(payload);
    },
    onSuccess: async () => {
      setFeedback({
        tone: 'success',
        message: editingRouteId ? '规则已更新。' : '规则已创建。',
      });
      setEditingRouteId(null);
      setIsEditorOpen(false);
      setMatchResult(null);
      form.reset(defaultValues);
      await queryClient.invalidateQueries({ queryKey: routesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteProxyRoute,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '规则已删除。' });
      await queryClient.invalidateQueries({ queryKey: routesQueryKey });
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
        message: `发布成功，版本 ${version.version}`,
      });
      await queryClient.invalidateQueries({ queryKey: versionsQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const handlePublish = async () => {
    setFeedback(null);

    try {
      const diff = await getConfigVersionDiff();
      if (!hasConfigChanges(diff)) {
        setFeedback({
          tone: 'info',
          message: '当前规则没有变更，已阻止重复发布。',
        });
        return;
      }

      publishMutation.mutate();
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    }
  };

  useEffect(() => {
    if (!watchedEnableHttps) {
      setMatchResult(null);
      setIsMatching(false);
      return;
    }

    const normalizedDomain = watchedDomain.trim().toLowerCase();
    if (!normalizedDomain) {
      setMatchResult(null);
      return;
    }

    let cancelled = false;
    const timer = window.setTimeout(async () => {
      try {
        setIsMatching(true);
        const result = await matchManagedDomainCertificate(normalizedDomain);
        if (cancelled) {
          return;
        }
        setMatchResult(result);
        if (result.candidate?.certificate_id && !form.getValues('cert_id')) {
          form.setValue('cert_id', String(result.candidate.certificate_id), {
            shouldDirty: true,
            shouldValidate: true,
          });
        }
      } catch (error) {
        if (!cancelled) {
          setMatchResult(null);
          setFeedback({ tone: 'danger', message: getErrorMessage(error) });
        }
      } finally {
        if (!cancelled) {
          setIsMatching(false);
        }
      }
    }, 400);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [watchedDomain, watchedEnableHttps, form]);

  const certificates = useMemo(
    () => certificatesQuery.data ?? [],
    [certificatesQuery.data],
  );

  const handleReset = () => {
    setFeedback(null);
    setEditingRouteId(null);
    setIsEditorOpen(false);
    setMatchResult(null);
    form.reset(defaultValues);
  };

  const handleCreate = () => {
    setFeedback(null);
    setEditingRouteId(null);
    setMatchResult(null);
    form.reset(defaultValues);
    setIsEditorOpen(true);
  };

  const handleSubmit = form.handleSubmit((values) => {
    setFeedback(null);
    saveMutation.mutate(values);
  });

  const handleEdit = (route: ProxyRouteItem) => {
    setFeedback(null);
    setEditingRouteId(route.id);
    setMatchResult(null);
    form.reset(toFormValues(route));
    setIsEditorOpen(true);
  };

  const handleDelete = (route: ProxyRouteItem) => {
    if (!window.confirm(`确认删除规则 ${route.domain} 吗？`)) {
      return;
    }

    setFeedback(null);
    deleteMutation.mutate(route.id);
  };

  const handleRemoveHeader = (index: number) => {
    if (fields.length === 1) {
      replace([{ key: '', value: '' }]);
      return;
    }

    remove(index);
  };

  const routes = routesQuery.data || [];

  return (
    <>
      <div className="space-y-6">
        <PageHeader
          title="反代规则"
          description="维护域名到源站的映射、HTTPS 证书绑定与自定义请求头，并可直接触发配置发布。"
          action={
            <>
              <PrimaryButton
                type="button"
                onClick={() => void handlePublish()}
                disabled={publishMutation.isPending}
              >
                {publishMutation.isPending ? '发布中...' : '发布当前规则'}
              </PrimaryButton>
              <SecondaryButton type="button" onClick={handleCreate}>
                新增规则
              </SecondaryButton>
            </>
          }
        />

        {feedback ? (
          <InlineMessage tone={feedback.tone} message={feedback.message} />
        ) : null}

        <AppCard title="规则列表">
          {routesQuery.isLoading ? (
            <LoadingState />
          ) : routesQuery.isError ? (
            <ErrorState
              title="规则列表加载失败"
              description={getErrorMessage(routesQuery.error)}
            />
          ) : routes.length === 0 ? (
            <EmptyState
              title="暂无反代规则"
              description="请先创建至少一条规则，然后再进行发布。"
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-[var(--border-default)] text-left text-sm">
                <thead>
                  <tr className="text-[var(--foreground-secondary)]">
                    <th className="px-3 py-3 font-medium">域名</th>
                    <th className="px-3 py-3 font-medium">源站地址</th>
                    <th className="px-3 py-3 font-medium">HTTPS</th>
                    <th className="px-3 py-3 font-medium">请求头</th>
                    <th className="px-3 py-3 font-medium">状态</th>
                    <th className="px-3 py-3 font-medium">备注</th>
                    <th className="px-3 py-3 font-medium">更新时间</th>
                    <th className="px-3 py-3 font-medium">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border-default)]">
                  {routes.map((route) => {
                    const headers = parseCustomHeaders(route.custom_headers);

                    return (
                      <tr key={route.id} className="align-top">
                        <td className="px-3 py-4 font-medium text-[var(--foreground-primary)]">
                          {route.domain}
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {route.origin_url}
                        </td>
                        <td className="px-3 py-4">
                          {route.enable_https ? (
                            <div className="space-y-2">
                              <StatusBadge
                                label={
                                  route.redirect_http
                                    ? 'HTTPS + 重定向'
                                    : 'HTTPS'
                                }
                                variant="info"
                              />
                            </div>
                          ) : (
                            <StatusBadge label="HTTP" variant="warning" />
                          )}
                        </td>
                        <td className="px-3 py-4">
                          <StatusBadge
                            label={
                              headers.length > 0 ? `${headers.length} 条` : '无'
                            }
                            variant={headers.length > 0 ? 'success' : 'warning'}
                          />
                        </td>
                        <td className="px-3 py-4">
                          <StatusBadge
                            label={route.enabled ? '启用' : '停用'}
                            variant={route.enabled ? 'success' : 'warning'}
                          />
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {route.remark || '—'}
                        </td>
                        <td className="px-3 py-4 text-[var(--foreground-secondary)]">
                          {formatDateTime(route.updated_at)}
                        </td>
                        <td className="px-3 py-4">
                          <div className="flex flex-wrap gap-2">
                            <SecondaryButton
                              type="button"
                              onClick={() => handleEdit(route)}
                              className="px-3 py-2 text-xs"
                            >
                              编辑
                            </SecondaryButton>
                            <DangerButton
                              type="button"
                              onClick={() => handleDelete(route)}
                              disabled={deleteMutation.isPending}
                              className="px-3 py-2 text-xs"
                            >
                              删除
                            </DangerButton>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </AppCard>
      </div>
      <AppModal
        isOpen={isEditorOpen}
        onClose={handleReset}
        title={editingRouteId ? '编辑规则' : '新增规则'}
        description="新增或修改反代规则后，可直接回到列表页继续发布。"
        size="xl"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <SecondaryButton
              type="button"
              onClick={handleReset}
              disabled={saveMutation.isPending}
            >
              取消
            </SecondaryButton>
            <PrimaryButton
              type="submit"
              form="proxy-route-editor-form"
              disabled={saveMutation.isPending}
            >
              {saveMutation.isPending
                ? '保存中...'
                : editingRouteId
                  ? '保存修改'
                  : '新增规则'}
            </PrimaryButton>
          </div>
        }
      >
        <form
          id="proxy-route-editor-form"
          className="space-y-5"
          onSubmit={handleSubmit}
        >
          <div className="grid gap-4 md:grid-cols-2">
            <ResourceField
              label="域名"
              hint="示例：example.com"
              error={form.formState.errors.domain?.message}
            >
              <ResourceInput
                placeholder="example.com"
                {...form.register('domain')}
              />
            </ResourceField>
            <ResourceField
              label="源站地址"
              hint="示例：https://origin.internal"
              error={form.formState.errors.origin_url?.message}
            >
              <ResourceInput
                placeholder="https://origin.internal"
                {...form.register('origin_url')}
              />
            </ResourceField>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <ToggleField
              label="启用规则"
              description="关闭后该规则不会参与配置渲染与发布。"
              checked={watchedEnabled}
              onChange={(checked) =>
                form.setValue('enabled', checked, { shouldDirty: true })
              }
            />
            <ToggleField
              label="启用 HTTPS"
              description="启用后必须关联 TLS 证书，可选择是否将 HTTP 自动重定向到 HTTPS。"
              checked={watchedEnableHttps}
              onChange={(checked) => {
                form.setValue('enable_https', checked, {
                  shouldDirty: true,
                  shouldValidate: true,
                });
                if (!checked) {
                  form.setValue('cert_id', '', {
                    shouldDirty: true,
                    shouldValidate: true,
                  });
                  form.setValue('redirect_http', false, {
                    shouldDirty: true,
                    shouldValidate: true,
                  });
                }
              }}
            />
          </div>

          <div className="grid gap-4 lg:grid-cols-[1.2fr_0.8fr]">
            <ResourceField
              label="TLS 证书"
              hint="启用 HTTPS 后可自动推荐匹配证书，也支持手动选择。"
              error={form.formState.errors.cert_id?.message}
            >
              <ResourceSelect
                value={watchedCertId}
                disabled={!watchedEnableHttps || certificatesQuery.isLoading}
                onChange={(event) =>
                  form.setValue('cert_id', event.target.value, {
                    shouldDirty: true,
                    shouldValidate: true,
                  })
                }
              >
                <option value="">请选择证书</option>
                {certificates.map((certificate) => (
                  <option key={certificate.id} value={certificate.id}>
                    {buildCertificateLabel(certificate)}
                  </option>
                ))}
              </ResourceSelect>
            </ResourceField>
            <ToggleField
              label="HTTP 跳转 HTTPS"
              description="仅在启用 HTTPS 后可开启。开启后会将 HTTP 请求重定向到 HTTPS。"
              checked={watchedRedirectHttp}
              disabled={!watchedEnableHttps}
              onChange={(checked) =>
                form.setValue('redirect_http', checked, {
                  shouldDirty: true,
                  shouldValidate: true,
                })
              }
            />
          </div>

          <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-[var(--foreground-primary)]">
                  自定义请求头
                </p>
                <p className="mt-1 text-xs leading-5 text-[var(--foreground-secondary)]">
                  可为空。若填写，请保证 Header 名称合法且不包含换行。
                </p>
              </div>
              <SecondaryButton
                type="button"
                onClick={() => append({ key: '', value: '' })}
                className="px-3 py-2 text-xs"
              >
                添加请求头
              </SecondaryButton>
            </div>

            <div className="mt-4 space-y-4">
              {fields.map((field, index) => (
                <div
                  key={field.id}
                  className="grid gap-3 md:grid-cols-[1fr_1fr_auto]"
                >
                  <ResourceField
                    label={
                      index === 0 ? 'Header 名称' : `Header 名称 ${index + 1}`
                    }
                    error={
                      form.formState.errors.custom_headers?.[index]?.key
                        ?.message
                    }
                  >
                    <ResourceInput
                      placeholder="X-Trace-Id"
                      {...form.register(`custom_headers.${index}.key`)}
                    />
                  </ResourceField>
                  <ResourceField
                    label={index === 0 ? 'Header 值' : `Header 值 ${index + 1}`}
                    error={
                      form.formState.errors.custom_headers?.[index]?.value
                        ?.message
                    }
                  >
                    <ResourceInput
                      placeholder="$request_id"
                      {...form.register(`custom_headers.${index}.value`)}
                    />
                  </ResourceField>
                  <div className="flex items-end">
                    <DangerButton
                      type="button"
                      onClick={() => handleRemoveHeader(index)}
                      className="w-full px-3 py-3 text-xs md:w-auto"
                    >
                      删除
                    </DangerButton>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <AppCard
            title="证书匹配提示"
            description="根据域名自动匹配托管证书，优先使用精确匹配规则。"
          >
            <div className="space-y-3">
              <p className="text-sm leading-6 text-[var(--foreground-secondary)]">
                {getMatchMessage(
                  matchResult,
                  isMatching,
                  watchedDomain,
                  watchedEnableHttps,
                )}
              </p>
              {matchResult?.matched && matchResult.candidates.length > 1 ? (
                <div className="flex flex-wrap gap-2">
                  {matchResult.candidates.map((candidate) => (
                    <StatusBadge
                      key={`${candidate.managed_domain_id}-${candidate.certificate_id}`}
                      label={`${candidate.domain} → ${candidate.certificate_name}`}
                      variant={
                        candidate.match_type === 'exact' ? 'success' : 'info'
                      }
                    />
                  ))}
                </div>
              ) : null}
              <div className="rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4 text-sm text-[var(--foreground-secondary)]">
                当前可选证书：
                {certificatesQuery.isLoading
                  ? '加载中...'
                  : `${certificates.length} 张`}
              </div>
            </div>
          </AppCard>

          <ResourceField
            label="备注"
            hint="可选，便于标记用途、责任人或特殊说明。"
            error={form.formState.errors.remark?.message}
          >
            <ResourceTextarea
              placeholder="例如：主站生产流量入口"
              {...form.register('remark')}
            />
          </ResourceField>
        </form>
      </AppModal>
    </>
  );
}
