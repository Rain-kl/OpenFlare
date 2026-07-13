'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowUp, GripVertical } from 'lucide-react';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import type { ProxyRouteItem } from '@/lib/services/openflare';
import { WafService } from '@/lib/services/openflare';

import { getErrorMessage } from '../../components/helpers';
import { proxyRouteFormIds } from '../helpers';
import { SectionShell } from './section-shell';

interface WafSectionProps {
  route: ProxyRouteItem;
  onSavingChange?: (saving: boolean) => void;
}

interface SortableRuleProps {
  id: number;
  name: string;
  index: number;
  total: number;
  onMove: (from: number, to: number) => void;
}

export function reorderRuleIDs(
  ids: number[],
  activeID: number,
  overID: number,
) {
  const from = ids.indexOf(activeID);
  const to = ids.indexOf(overID);
  return from < 0 || to < 0 || from === to ? ids : arrayMove(ids, from, to);
}

function SortableRule({ id, name, index, total, onMove }: SortableRuleProps) {
  const { attributes, listeners, setNodeRef, transform, transition } =
    useSortable({ id });
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className='flex items-center gap-2 rounded-lg border p-3'
    >
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        aria-label={`拖动${name}`}
        {...attributes}
        {...listeners}
      >
        <GripVertical data-icon='inline-start' />
      </Button>
      <span className='min-w-0 flex-1 truncate text-sm font-medium'>
        {name}
      </span>
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        aria-label={`上移${name}`}
        disabled={index === 0}
        onClick={() => onMove(index, index - 1)}
      >
        <ArrowUp data-icon='inline-start' />
      </Button>
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        aria-label={`下移${name}`}
        disabled={index === total - 1}
        onClick={() => onMove(index, index + 1)}
      >
        <ArrowDown data-icon='inline-start' />
      </Button>
    </div>
  );
}

export function WafSection({ route, onSavingChange }: WafSectionProps) {
  const queryClient = useQueryClient();
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);
  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  const wafQuery = useQuery({
    queryKey: ['openflare', 'waf', 'site-rule-groups', route.id],
    queryFn: () => WafService.listSiteRuleGroups(route.id),
  });

  const wafMutation = useMutation({
    mutationFn: (ids: number[]) =>
      WafService.updateSiteRuleGroups(route.id, ids),
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
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'waf', 'rule-groups'],
        }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'config-versions', 'diff'],
        }),
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
  const ruleMap = useMemo(
    () =>
      new Map(
        (wafQuery.data?.rule_groups ?? []).map((rule) => [rule.id, rule]),
      ),
    [wafQuery.data?.rule_groups],
  );

  const moveRule = (from: number, to: number) => {
    if (to < 0 || to >= selectedIDs.length) return;
    setSelectedIDs((current) => arrayMove(current, from, to));
  };

  const handleDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) return;
    setSelectedIDs((current) =>
      reorderRuleIDs(current, Number(active.id), Number(over.id)),
    );
  };

  return (
    <SectionShell
      title='WAF'
      description='全局规则组始终生效；这里可以为当前网站叠加自定义规则组。'
      formId={proxyRouteFormIds.waf}
      saving={wafMutation.isPending}
    >
      {wafQuery.isLoading ? (
        <LoadingStateWithBorder description='加载 WAF 规则组...' />
      ) : wafQuery.isError ? (
        <ErrorInline
          message={getErrorMessage(wafQuery.error)}
          onRetry={() => void wafQuery.refetch()}
        />
      ) : (
        <form
          id={proxyRouteFormIds.waf}
          className='flex flex-col gap-5'
          onSubmit={(event) => {
            event.preventDefault();
            wafMutation.mutate(selectedIDs);
          }}
        >
          {wafQuery.data?.global_rule_group ? (
            <div
              className='rounded-lg border bg-muted/30 p-4'
              data-testid='global-waf-rule'
            >
              <div className='flex items-center justify-between gap-3'>
                <div>
                  <p className='text-[10px] font-semibold uppercase tracking-wider text-muted-foreground'>
                    Global Rule Group
                  </p>
                  <p className='mt-1 text-sm font-semibold'>
                    {wafQuery.data.global_rule_group.name}
                  </p>
                </div>
                <Badge variant='outline'>始终生效</Badge>
              </div>
            </div>
          ) : null}

          <FieldSet>
            <FieldLegend variant='label'>选择自定义规则</FieldLegend>
            <FieldDescription>
              选中的规则会按下方执行顺序依次运行。
            </FieldDescription>
            <FieldGroup
              data-slot='checkbox-group'
              className='grid gap-3 md:grid-cols-2'
            >
              {(wafQuery.data?.rule_groups ?? []).map((group) => {
                const checkboxID = `waf-rule-${group.id}`;
                return (
                  <Field
                    key={group.id}
                    orientation='horizontal'
                    className='rounded-lg border p-4'
                  >
                    <Checkbox
                      id={checkboxID}
                      aria-label={`选择${group.name}`}
                      checked={selectedSet.has(group.id)}
                      onCheckedChange={(checked) => {
                        setSelectedIDs((current) =>
                          checked
                            ? [...current, group.id]
                            : current.filter((id) => id !== group.id),
                        );
                      }}
                    />
                    <FieldLabel
                      htmlFor={checkboxID}
                      className='min-w-0 cursor-pointer flex-col items-start gap-1'
                    >
                      <span className='truncate text-sm font-semibold'>
                        {group.name}
                      </span>
                      <span className='text-xs text-muted-foreground'>
                        {group.enabled ? '启用中' : '已停用'} ·{' '}
                        {group.graph.nodes.length} 个节点
                      </span>
                    </FieldLabel>
                  </Field>
                );
              })}
            </FieldGroup>
          </FieldSet>

          {selectedIDs.length > 0 ? (
            <div className='flex flex-col gap-2'>
              <p className='text-sm font-medium'>执行顺序</p>
              <p className='text-xs text-muted-foreground'>
                自定义规则按此顺序执行，可拖动或使用上下移动按钮调整。
              </p>
              <DndContext
                sensors={sensors}
                collisionDetection={closestCenter}
                onDragEnd={handleDragEnd}
              >
                <SortableContext
                  items={selectedIDs}
                  strategy={verticalListSortingStrategy}
                >
                  <div className='flex flex-col gap-2'>
                    {selectedIDs.map((id, index) => {
                      const rule = ruleMap.get(id);
                      return rule ? (
                        <SortableRule
                          key={id}
                          id={id}
                          name={rule.name}
                          index={index}
                          total={selectedIDs.length}
                          onMove={moveRule}
                        />
                      ) : null;
                    })}
                  </div>
                </SortableContext>
              </DndContext>
            </div>
          ) : null}

          {(wafQuery.data?.rule_groups ?? []).length === 0 ? (
            <EmptyStateWithBorder description='暂无自定义 WAF 规则组' />
          ) : null}
        </form>
      )}
    </SectionShell>
  );
}
