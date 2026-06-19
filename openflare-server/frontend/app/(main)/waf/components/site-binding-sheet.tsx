'use client';

import {useEffect, useMemo, useState} from 'react';
import {Check, Search} from 'lucide-react';

import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle,} from '@/components/ui/sheet';
import {cn} from '@/lib/utils';
import type {ProxyRouteItem, WAFRuleGroup} from '@/lib/services/openflare';

interface SiteBindingSheetProps {
  group: WAFRuleGroup | null;
  routes: ProxyRouteItem[];
  open: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSave: (ids: number[]) => void;
}

export function SiteBindingSheet({
  group,
  routes,
  open,
  pending,
  onOpenChange,
  onSave,
}: SiteBindingSheetProps) {
  const [keyword, setKeyword] = useState('');
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);

  useEffect(() => {
    setSelectedIDs(group?.applied_site_ids ?? []);
    setKeyword('');
  }, [group, open]);

  const filteredRoutes = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) return routes;
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
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{group ? `绑定 ${group.name}` : '绑定规则组'}</SheetTitle>
          <SheetDescription>
            选择这个自定义规则组要叠加到哪些网站。
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-4 px-4 pb-4">
          <div className="flex items-center gap-2 rounded-md border px-3 py-2">
            <Search className="size-4 text-muted-foreground" />
            <Input
              value={keyword}
              placeholder="搜索网站或域名"
              className="border-0 shadow-none focus-visible:ring-0"
              onChange={(event) => setKeyword(event.target.value)}
            />
            <Button type="button" variant="ghost" size="sm" onClick={selectFiltered}>
              全选当前
            </Button>
          </div>

          <div className="space-y-2">
            {filteredRoutes.map((route) => (
              <button
                key={route.id}
                type="button"
                onClick={() => toggleID(route.id)}
                className={cn(
                  'flex w-full items-center gap-3 rounded-md border px-3 py-2 text-left transition',
                  selectedSet.has(route.id) && 'border-primary bg-muted/50',
                )}
              >
                <span
                  className={cn(
                    'flex size-5 items-center justify-center rounded border',
                    selectedSet.has(route.id) && 'border-primary bg-primary text-primary-foreground',
                  )}
                >
                  {selectedSet.has(route.id) ? <Check className="size-3" /> : null}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium">
                    {route.site_name}
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {route.domains.join(', ')}
                  </span>
                </span>
              </button>
            ))}
          </div>
        </div>

        <SheetFooter className="px-4">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            type="button"
            disabled={!group || pending}
            onClick={() => onSave(selectedIDs)}
          >
            {pending ? '保存中...' : '保存应用范围'}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
