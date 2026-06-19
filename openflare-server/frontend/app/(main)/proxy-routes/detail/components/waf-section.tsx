'use client';

import {useEffect, useMemo, useState} from 'react';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';

import {Badge} from '@/components/ui/badge';
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {ErrorInline} from '@/components/layout/error';
import {LoadingStateWithBorder} from '@/components/layout/loading';
import type {ProxyRouteItem} from '@/lib/services/openflare';
import {WafService} from '@/lib/services/openflare';
import {cn} from '@/lib/utils';

import {getErrorMessage} from '../../components/helpers';
import {proxyRouteFormIds} from '../helpers';
import {SectionShell} from './section-shell';

interface WafSectionProps {
  route: ProxyRouteItem;
  onSavingChange?: (saving: boolean) => void;
}

export function WafSection({ route, onSavingChange }: WafSectionProps) {
  const queryClient = useQueryClient();
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);

  const wafQuery = useQuery({
    queryKey: ['openflare', 'waf', 'site-rule-groups', route.id],
    queryFn: () => WafService.listSiteRuleGroups(route.id),
  });

  const wafMutation = useMutation({
    mutationFn: (ids: number[]) => WafService.updateSiteRuleGroups(route.id, ids),
    onMutate: () => {
      onSavingChange?.(true);
    },
    onSettled: () => {
      onSavingChange?.(false);
    },
    onSuccess: async (result) => {
      setSelectedIDs(result.applied_ids);
      toast.success('WAF 规则组已更新');
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'waf', 'site-rule-groups', route.id],
        }),
        queryClient.invalidateQueries({ queryKey: ['openflare', 'waf', 'rule-groups'] }),
        queryClient.invalidateQueries({ queryKey: ['openflare', 'config-versions', 'diff'] }),
      ]);
    },
    onError: (error) => {
      toast.error('保存失败', { description: getErrorMessage(error) });
    },
  });

  useEffect(() => {
    if (wafQuery.data) {
      setSelectedIDs(wafQuery.data.applied_ids);
    }
  }, [wafQuery.data]);

  const selectedSet = useMemo(() => new Set(selectedIDs), [selectedIDs]);

  return (
    <SectionShell
      title="WAF"
      description="全局规则组始终生效；这里可以为当前网站叠加自定义规则组。"
      formId={proxyRouteFormIds.waf}
      saving={wafMutation.isPending}
    >
      {wafQuery.isLoading ? (
        <LoadingStateWithBorder description="加载 WAF 规则组..." />
      ) : wafQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(wafQuery.error)}
          onRetry={() => void wafQuery.refetch()}
        />
      ) : (
        <form
          id={proxyRouteFormIds.waf}
          className="space-y-5"
          onSubmit={(event) => {
            event.preventDefault();
            wafMutation.mutate(selectedIDs);
          }}
        >
          {wafQuery.data?.global_rule_group ? (
            <div className="rounded-lg border bg-muted/30 p-4">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                    Global Rule Group
                  </p>
                  <p className="mt-1 text-sm font-semibold">
                    {wafQuery.data.global_rule_group.name}
                  </p>
                </div>
                <Badge variant="outline">始终生效</Badge>
              </div>
            </div>
          ) : null}

          <div className="grid gap-3 md:grid-cols-2">
            {(wafQuery.data?.rule_groups ?? []).map((group) => (
              <label
                key={group.id}
                className={cn(
                  'flex cursor-pointer items-start gap-3 rounded-lg border p-4 transition',
                  selectedSet.has(group.id) && 'border-primary bg-muted/40',
                )}
              >
                <input
                  type="checkbox"
                  checked={selectedSet.has(group.id)}
                  onChange={(event) => {
                    const checked = event.target.checked;
                    setSelectedIDs((current) =>
                      checked
                        ? [...current, group.id].sort((left, right) => left - right)
                        : current.filter((id) => id !== group.id),
                    );
                  }}
                  className="mt-1 size-4 rounded border accent-primary"
                />
                <span className="min-w-0">
                  <span className="block text-sm font-semibold">{group.name}</span>
                  <span className="mt-1 block text-xs text-muted-foreground">
                    {group.enabled ? '启用中' : '已停用'} ·{' '}
                    {group.ip_whitelist.length +
                      group.ip_blacklist.length +
                      group.country_whitelist.length +
                      group.country_blacklist.length}{' '}
                    条规则
                  </span>
                </span>
              </label>
            ))}
          </div>

          {(wafQuery.data?.rule_groups ?? []).length === 0 ? (
            <EmptyStateWithBorder description="暂无自定义 WAF 规则组" />
          ) : null}
        </form>
      )}
    </SectionShell>
  );
}