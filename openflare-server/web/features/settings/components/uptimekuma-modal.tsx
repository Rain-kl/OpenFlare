'use client';

import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import { ErrorState } from '@/components/feedback/error-state';
import { LoadingState } from '@/components/feedback/loading-state';
import { AppModal } from '@/components/ui/app-modal';
import { getProxyRoutes } from '@/features/proxy-routes/api/proxy-routes';
import {
  PrimaryButton,
  ResourceField,
  ResourceInput,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '加载站点列表失败。';
}

export function UptimeKumaSiteSelectModal({
  isOpen,
  selectedSites,
  onClose,
  onSave,
}: {
  isOpen: boolean;
  selectedSites: string[];
  onClose: () => void;
  onSave: (sites: string[]) => void;
}) {
  const [searchTerm, setSearchTerm] = useState('');
  const [tempSelected, setTempSelected] = useState<Set<string>>(new Set());

  const {
    data: routes = [],
    isLoading,
    error,
  } = useQuery({
    queryKey: ['proxy-routes'],
    queryFn: getProxyRoutes,
    enabled: isOpen,
  });

  useEffect(() => {
    if (isOpen) {
      setTempSelected(
        new Set(selectedSites.map((s) => s.trim()).filter(Boolean)),
      );
      setSearchTerm('');
    }
  }, [isOpen, selectedSites]);

  const toggleSite = (siteName: string) => {
    setTempSelected((prev) => {
      const next = new Set(prev);
      if (next.has(siteName)) {
        next.delete(siteName);
      } else {
        next.add(siteName);
      }
      return next;
    });
  };

  const handleSelectAll = () => {
    setTempSelected((prev) => {
      const next = new Set(prev);
      filteredRoutes.forEach((route) => {
        next.add(route.site_name);
      });
      return next;
    });
  };

  const handleDeselectAll = () => {
    setTempSelected((prev) => {
      const next = new Set(prev);
      filteredRoutes.forEach((route) => {
        next.delete(route.site_name);
      });
      return next;
    });
  };

  const handleSave = () => {
    onSave(Array.from(tempSelected));
    onClose();
  };

  const filteredRoutes = routes.filter(
    (route) =>
      route.site_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      route.primary_domain.toLowerCase().includes(searchTerm.toLowerCase()),
  );

  return (
    <AppModal
      isOpen={isOpen}
      title="选择监控站点"
      description="请选择要同步到 Uptime Kuma 监控的站点，支持按站点名称和域名搜索。"
      size="lg"
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-3">
          <SecondaryButton type="button" onClick={onClose}>
            取消
          </SecondaryButton>
          <PrimaryButton type="button" onClick={handleSave}>
            保存选择
          </PrimaryButton>
        </div>
      }
    >
      <div className="space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <div className="flex-1">
            <ResourceField label="搜索站点">
              <ResourceInput
                value={searchTerm}
                onChange={(event) => setSearchTerm(event.target.value)}
                placeholder="按名称或域名搜索..."
              />
            </ResourceField>
          </div>
          <div className="flex gap-2 sm:mt-6">
            <SecondaryButton type="button" onClick={handleSelectAll}>
              全选过滤项
            </SecondaryButton>
            <SecondaryButton type="button" onClick={handleDeselectAll}>
              清空过滤项
            </SecondaryButton>
          </div>
        </div>

        {isLoading ? <LoadingState /> : null}

        {error ? (
          <ErrorState
            title="站点加载失败"
            description={getErrorMessage(error)}
          />
        ) : null}

        {!isLoading && !error && routes.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-[var(--border-default)] px-5 py-8 text-center text-sm text-[var(--foreground-secondary)]">
            暂无可用的代理站点。
          </div>
        ) : null}

        {!isLoading && !error && routes.length > 0 ? (
          <div className="max-h-[350px] overflow-y-auto rounded-2xl border border-[var(--border-default)] bg-[var(--surface-base)]">
            <table className="w-full text-left text-sm">
              <thead className="sticky top-0 bg-[var(--surface-elevated)] text-xs text-[var(--foreground-secondary)] uppercase">
                <tr>
                  <th className="w-12 px-4 py-3">选择</th>
                  <th className="px-4 py-3 font-medium">站点名称</th>
                  <th className="px-4 py-3 font-medium">主域名</th>
                  <th className="px-4 py-3 font-medium">状态</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--border-default)]">
                {filteredRoutes.map((route) => {
                  const isChecked = tempSelected.has(route.site_name);
                  return (
                    <tr
                      key={route.id}
                      onClick={() => toggleSite(route.site_name)}
                      className="cursor-pointer hover:bg-[var(--surface-elevated)]"
                    >
                      <td
                        className="px-4 py-3"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <input
                          type="checkbox"
                          checked={isChecked}
                          onChange={() => toggleSite(route.site_name)}
                          className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                        />
                      </td>
                      <td className="px-4 py-3 font-medium text-[var(--foreground-primary)]">
                        {route.site_name}
                      </td>
                      <td className="px-4 py-3 text-[var(--foreground-secondary)]">
                        {route.primary_domain}
                      </td>
                      <td className="px-4 py-3">
                        <span
                          className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                            route.enabled
                              ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                              : 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400'
                          }`}
                        >
                          {route.enabled ? '启用' : '禁用'}
                        </span>
                      </td>
                    </tr>
                  );
                })}
                {filteredRoutes.length === 0 ? (
                  <tr>
                    <td
                      colSpan={4}
                      className="px-4 py-8 text-center text-[var(--foreground-secondary)]"
                    >
                      无匹配的站点
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        ) : null}

        <div className="text-right text-xs text-[var(--foreground-muted)]">
          已选择 {tempSelected.size} 个监控站点
        </div>
      </div>
    </AppModal>
  );
}
