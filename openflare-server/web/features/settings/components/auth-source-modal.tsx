'use client';

import { useEffect, useState } from 'react';

import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { AppModal } from '@/components/ui/app-modal';
import {
  createAuthSource,
  deleteAuthSource,
  toggleAuthSource,
  updateAuthSource,
} from '@/features/settings/api/settings';
import type {
  AuthSource,
  AuthSourcePayload,
  AuthSourceType,
} from '@/features/settings/types';
import {
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  ResourceSelect,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';

const emptyForm: AuthSourcePayload = {
  name: '',
  type: 'github',
  display_name: '',
  is_active: false,
  client_id: '',
  client_secret: '',
  openid_discovery_url: '',
  scopes: 'user:email',
  icon_url: '',
};

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

function sourceToForm(source: AuthSource): AuthSourcePayload {
  return {
    id: source.id,
    name: source.name,
    type: source.type,
    display_name: source.display_name,
    is_active: source.is_active,
    client_id: source.client_id,
    client_secret: '',
    openid_discovery_url: source.openid_discovery_url,
    scopes:
      source.scopes ||
      (source.type === 'oidc' ? 'openid profile email' : 'user:email'),
    icon_url: source.icon_url,
  };
}

function buildCallbackURL(origin: string, sourceName: string) {
  const normalizedName = sourceName.trim() || '认证源名称';
  return `${origin || '当前访问地址'}/oauth/${encodeURIComponent(normalizedName)}`;
}

export function AuthSourceModal({
  isOpen,
  sources,
  isLoading,
  error,
  onClose,
  onChanged,
}: {
  isOpen: boolean;
  sources: AuthSource[];
  isLoading: boolean;
  error: unknown;
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const [mode, setMode] = useState<'list' | 'edit'>('list');
  const [editingSource, setEditingSource] = useState<AuthSource | null>(null);
  const [form, setForm] = useState<AuthSourcePayload>(emptyForm);
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [message, setMessage] = useState<{
    tone: 'success' | 'danger' | 'info';
    text: string;
  } | null>(null);
  const [browserOrigin, setBrowserOrigin] = useState('');

  useEffect(() => {
    if (!isOpen) {
      setMode('list');
      setEditingSource(null);
      setForm(emptyForm);
      setBusyKey(null);
      setMessage(null);
    }
  }, [isOpen]);

  useEffect(() => {
    if (typeof window !== 'undefined') {
      setBrowserOrigin(window.location.origin);
    }
  }, []);

  const startCreate = () => {
    setEditingSource(null);
    setForm(emptyForm);
    setMessage(null);
    setMode('edit');
  };

  const startEdit = (source: AuthSource) => {
    setEditingSource(source);
    setForm(sourceToForm(source));
    setMessage(null);
    setMode('edit');
  };

  const updateType = (type: AuthSourceType) => {
    setForm((previous) => ({
      ...previous,
      type,
      scopes:
        previous.scopes === 'user:email' ||
        previous.scopes === 'openid profile email' ||
        !previous.scopes
          ? type === 'oidc'
            ? 'openid profile email'
            : 'user:email'
          : previous.scopes,
    }));
  };

  const runAction = async (key: string, action: () => Promise<void>) => {
    setBusyKey(key);
    setMessage(null);
    try {
      await action();
    } catch (actionError) {
      setMessage({ tone: 'danger', text: getErrorMessage(actionError) });
    } finally {
      setBusyKey(null);
    }
  };

  const saveForm = () => {
    void runAction('save', async () => {
      const payload: AuthSourcePayload = {
        ...form,
        name: form.name.trim(),
        display_name: form.display_name.trim(),
        client_id: form.client_id.trim(),
        client_secret: form.client_secret.trim(),
        openid_discovery_url: form.openid_discovery_url.trim(),
        scopes: form.scopes.trim(),
        icon_url: form.icon_url.trim(),
      };
      if (editingSource) {
        await updateAuthSource(editingSource.id, payload);
      } else {
        await createAuthSource(payload);
      }
      await onChanged();
      setMessage({ tone: 'success', text: '认证源已保存。' });
      setMode('list');
      setEditingSource(null);
      setForm(emptyForm);
    });
  };

  const removeSource = (source: AuthSource) => {
    if (
      !window.confirm(
        `确定删除认证源「${source.display_name || source.name}」吗？`,
      )
    ) {
      return;
    }
    void runAction(`delete-${source.id}`, async () => {
      await deleteAuthSource(source.id);
      await onChanged();
      setMessage({ tone: 'success', text: '认证源已删除。' });
    });
  };

  const toggleSource = (source: AuthSource) => {
    void runAction(`toggle-${source.id}`, async () => {
      await toggleAuthSource(source.id, !source.is_active);
      await onChanged();
      setMessage({ tone: 'success', text: '认证源状态已更新。' });
    });
  };

  return (
    <AppModal
      isOpen={isOpen}
      title="认证源"
      description="配置 GitHub 或标准 OIDC 登录入口。启用后会显示在登录页。"
      size="xl"
      onClose={onClose}
      footer={
        mode === 'edit' ? (
          <div className="flex justify-end gap-3">
            <SecondaryButton type="button" onClick={() => setMode('list')}>
              返回列表
            </SecondaryButton>
            <PrimaryButton
              type="button"
              onClick={saveForm}
              disabled={busyKey === 'save'}
            >
              {busyKey === 'save' ? '保存中...' : '保存认证源'}
            </PrimaryButton>
          </div>
        ) : (
          <div className="flex justify-end">
            <SecondaryButton type="button" onClick={onClose}>
              关闭
            </SecondaryButton>
          </div>
        )
      }
    >
      <div className="space-y-5">
        {message ? (
          <InlineMessage tone={message.tone} message={message.text} />
        ) : null}

        {mode === 'list' ? (
          <div className="space-y-4">
            <div className="flex justify-end">
              <PrimaryButton type="button" onClick={startCreate}>
                新增认证源
              </PrimaryButton>
            </div>
            {isLoading ? <LoadingState /> : null}
            {error ? (
              <ErrorState
                title="认证源加载失败"
                description={getErrorMessage(error)}
              />
            ) : null}
            {!isLoading && !error && sources.length === 0 ? (
              <div className="rounded-2xl border border-dashed border-[var(--border-default)] px-5 py-8 text-center text-sm text-[var(--foreground-secondary)]">
                暂无认证源。
              </div>
            ) : null}
            {!isLoading && !error && sources.length > 0 ? (
              <div className="overflow-hidden rounded-2xl border border-[var(--border-default)]">
                <table className="w-full min-w-[720px] text-left text-sm">
                  <thead className="bg-[var(--surface-elevated)] text-xs text-[var(--foreground-secondary)] uppercase">
                    <tr>
                      <th className="px-4 py-3 font-medium">名称</th>
                      <th className="px-4 py-3 font-medium">类型</th>
                      <th className="px-4 py-3 font-medium">状态</th>
                      <th className="px-4 py-3 font-medium">Client ID</th>
                      <th className="px-4 py-3 text-right font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--border-default)]">
                    {sources.map((source) => (
                      <tr key={source.id}>
                        <td className="px-4 py-3">
                          <div className="font-medium text-[var(--foreground-primary)]">
                            {source.display_name || source.name}
                          </div>
                          <div className="text-xs text-[var(--foreground-secondary)]">
                            {source.name}
                          </div>
                        </td>
                        <td className="px-4 py-3 uppercase">{source.type}</td>
                        <td className="px-4 py-3">
                          {source.is_active ? '已启用' : '已禁用'}
                        </td>
                        <td className="max-w-[220px] truncate px-4 py-3">
                          {source.client_id || '-'}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex flex-wrap justify-end gap-2">
                            <SecondaryButton
                              type="button"
                              onClick={() => toggleSource(source)}
                              disabled={busyKey === `toggle-${source.id}`}
                            >
                              {source.is_active ? '禁用' : '启用'}
                            </SecondaryButton>
                            <SecondaryButton
                              type="button"
                              onClick={() => startEdit(source)}
                            >
                              修改
                            </SecondaryButton>
                            <DangerButton
                              type="button"
                              onClick={() => removeSource(source)}
                              disabled={busyKey === `delete-${source.id}`}
                            >
                              删除
                            </DangerButton>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : null}
          </div>
        ) : (
          <div className="grid gap-5 md:grid-cols-2">
            <ResourceField label="认证源名称">
              <ResourceInput
                value={form.name}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    name: event.target.value,
                  }))
                }
                placeholder="GitHub"
              />
            </ResourceField>
            <ResourceField label="展示名称">
              <ResourceInput
                value={form.display_name}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    display_name: event.target.value,
                  }))
                }
                placeholder="GitHub"
              />
            </ResourceField>
            <ResourceField label="类型">
              <ResourceSelect
                value={form.type}
                onChange={(event) =>
                  updateType(event.target.value as AuthSourceType)
                }
              >
                <option value="github">GitHub</option>
                <option value="oidc">OIDC</option>
              </ResourceSelect>
            </ResourceField>
            <ResourceField label="状态">
              <ResourceSelect
                value={form.is_active ? 'true' : 'false'}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    is_active: event.target.value === 'true',
                  }))
                }
              >
                <option value="false">禁用</option>
                <option value="true">启用</option>
              </ResourceSelect>
            </ResourceField>
            <ResourceField label="Client ID">
              <ResourceInput
                value={form.client_id}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    client_id: event.target.value,
                  }))
                }
              />
            </ResourceField>
            <ResourceField
              label="Client Secret"
              hint={editingSource ? '留空表示不更新现有密钥。' : undefined}
            >
              <ResourceInput
                type="password"
                value={form.client_secret}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    client_secret: event.target.value,
                  }))
                }
              />
            </ResourceField>
            <div className="rounded-2xl border border-[var(--status-info-border)] bg-[var(--status-info-soft)] px-4 py-3 text-sm leading-6 text-[var(--status-info-foreground)] md:col-span-2">
              {form.name.trim() ? (
                <>
                  第三方平台的 Redirect URI / Callback URL 请填写：
                  <span className="font-medium">
                    {' '}
                    {buildCallbackURL(browserOrigin, form.name)}
                  </span>
                  。末尾路径使用上方填写的认证源名称，保存后如修改认证源名称，也需要同步更新第三方平台中的回调地址。
                </>
              ) : (
                '请先输入认证源名称获取回调地址'
              )}
            </div>
            {form.type === 'oidc' ? (
              <ResourceField
                label="OIDC Discovery URL"
                className="md:col-span-2"
              >
                <ResourceInput
                  value={form.openid_discovery_url}
                  onChange={(event) =>
                    setForm((previous) => ({
                      ...previous,
                      openid_discovery_url: event.target.value,
                    }))
                  }
                  placeholder="https://auth.example.com/.well-known/openid-configuration"
                />
              </ResourceField>
            ) : null}
            <ResourceField label="Scopes">
              <ResourceInput
                value={form.scopes}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    scopes: event.target.value,
                  }))
                }
              />
            </ResourceField>
            <ResourceField label="图标 URL">
              <ResourceInput
                value={form.icon_url}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    icon_url: event.target.value,
                  }))
                }
              />
            </ResourceField>
          </div>
        )}
      </div>
    </AppModal>
  );
}
