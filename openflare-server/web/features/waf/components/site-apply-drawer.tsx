import { useEffect, useMemo, useState } from 'react';
import { Check, Search } from 'lucide-react';
import { Drawer } from '@/components/ui/drawer';
import type { ProxyRouteItem } from '@/features/proxy-routes/types';
import {
  PrimaryButton,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import type { WAFRuleGroup } from '@/features/waf/types';
import { cn } from '@/lib/utils/cn';

export function SiteApplyDrawer({
  group,
  routes,
  open,
  onOpenChange,
  onSave,
  pending,
}: {
  group: WAFRuleGroup | null;
  routes: ProxyRouteItem[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSave: (ids: number[]) => void;
  pending: boolean;
}) {
  const [keyword, setKeyword] = useState('');
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);

  useEffect(() => {
    setSelectedIDs(group?.applied_site_ids ?? []);
    setKeyword('');
  }, [group, open]);

  const filteredRoutes = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) {
      return routes;
    }
    return routes.filter((route) =>
      [route.site_name, route.primary_domain, ...route.domains]
        .join(' ')
        .toLowerCase()
        .includes(normalized),
    );
  }, [keyword, routes]);

  const selectedSet = useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const toggleID = (id: number) => {
    setSelectedIDs((current) =>
      current.includes(id)
        ? current.filter((item) => item !== id)
        : [...current, id].sort((left, right) => left - right),
    );
  };

  const selectFiltered = () => {
    const next = new Set(selectedIDs);
    filteredRoutes.forEach((route) => next.add(route.id));
    setSelectedIDs([...next].sort((left, right) => left - right));
  };

  return (
    <Drawer
      open={open}
      onOpenChange={onOpenChange}
      direction="right"
      title={group ? `应用 ${group.name}` : '应用规则组'}
      description="选择这个自定义规则组要叠加到哪些网站。"
      footer={
        <div className="flex justify-end gap-3">
          <SecondaryButton type="button" onClick={() => onOpenChange(false)}>
            取消
          </SecondaryButton>
          <PrimaryButton
            type="button"
            disabled={!group || pending}
            onClick={() => onSave(selectedIDs)}
          >
            {pending ? '保存中...' : '保存应用范围'}
          </PrimaryButton>
        </div>
      }
    >
      <div className="space-y-4">
        <div className="flex items-center gap-3 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-3">
          <Search className="h-4 w-4 text-[var(--foreground-secondary)]" />
          <input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder="搜索网站或域名"
            className="min-w-0 flex-1 bg-transparent text-sm text-[var(--foreground-primary)] outline-none placeholder:text-[var(--foreground-muted)]"
          />
          <button
            type="button"
            onClick={selectFiltered}
            className="text-xs font-medium text-[var(--brand-primary)]"
          >
            全选当前
          </button>
        </div>
        <div className="space-y-2">
          {filteredRoutes.map((route) => (
            <button
              key={route.id}
              type="button"
              onClick={() => toggleID(route.id)}
              className={cn(
                'flex w-full items-center gap-3 rounded-2xl border px-4 py-3 text-left transition',
                selectedSet.has(route.id)
                  ? 'border-[var(--border-strong)] bg-[var(--accent-soft)]'
                  : 'border-[var(--border-default)] bg-[var(--surface-elevated)] hover:bg-[var(--surface-muted)]',
              )}
            >
              <span
                className={cn(
                  'flex h-5 w-5 items-center justify-center rounded-md border',
                  selectedSet.has(route.id)
                    ? 'border-[var(--brand-primary)] bg-[var(--brand-primary)] text-[var(--foreground-inverse)]'
                    : 'border-[var(--border-default)]',
                )}
              >
                {selectedSet.has(route.id) ? (
                  <Check className="h-3 w-3" />
                ) : null}
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium text-[var(--foreground-primary)]">
                  {route.site_name}
                </span>
                <span className="block truncate text-xs text-[var(--foreground-secondary)]">
                  {route.domains.join(', ')}
                </span>
              </span>
            </button>
          ))}
        </div>
      </div>
    </Drawer>
  );
}
