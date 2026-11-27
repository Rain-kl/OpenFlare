'use client';

import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ChevronDown, Loader2, Server, X } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible';
import { Input } from '@/components/ui/input';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { NodeService, type NodeItem } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

/**
 * NodeSelection encodes selected edge node IDs:
 * - `null`: default "all online edge nodes selected" state
 * - `string[]`: explicit subset (including `[]` when cleared by the user)
 */
export type NodeSelection = string[] | null;

export function useEdgeNodes() {
  const nodesQuery = useQuery({
    queryKey: ['openflare', 'nodes', 'all'],
    queryFn: () => NodeService.listNodes(),
  });

  const edgeNodes = useMemo(
    () =>
      (nodesQuery.data ?? []).filter(
        (n) => !n.node_type || n.node_type === 'edge_node',
      ),
    [nodesQuery.data],
  );

  const onlineNodes = useMemo(
    () => edgeNodes.filter((n) => n.status === 'online'),
    [edgeNodes],
  );

  const offlineNodes = useMemo(
    () => edgeNodes.filter((n) => n.status !== 'online'),
    [edgeNodes],
  );

  const onlineIds = useMemo(
    () => onlineNodes.map((n) => n.node_id),
    [onlineNodes],
  );

  return {
    nodesQuery,
    edgeNodes,
    onlineNodes,
    offlineNodes,
    onlineIds,
  };
}

function matchesNodeQuery(node: NodeItem, needle: string): boolean {
  if (!needle) return true;
  const fields = [
    node.name,
    node.node_id,
    node.geo_name,
    node.ip,
    node.asn ? `AS${node.asn}` : '',
  ];
  return fields.some((field) => field?.toLowerCase().includes(needle));
}

interface NodeSelectorProps {
  selectedIds: NodeSelection;
  onChange: (ids: NodeSelection) => void;
}

export function NodeSelector({ selectedIds, onChange }: NodeSelectorProps) {
  const t = useTranslations('networkTools');
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>(
    {
      online: true,
      offline: false,
    },
  );

  const { nodesQuery, edgeNodes, onlineNodes, offlineNodes, onlineIds } =
    useEdgeNodes();

  const effectiveSelectedSet = useMemo(() => {
    if (selectedIds === null) {
      return new Set(onlineIds);
    }
    return new Set(selectedIds);
  }, [onlineIds, selectedIds]);

  const effectiveSelectedCount = useMemo(() => {
    const validIds = new Set(edgeNodes.map((n) => n.node_id));
    let count = 0;
    for (const id of effectiveSelectedSet) {
      if (validIds.has(id)) {
        count += 1;
      }
    }
    return count;
  }, [edgeNodes, effectiveSelectedSet]);

  const isAllOnlineSelected = useMemo(() => {
    if (selectedIds === null) {
      return true;
    }
    if (onlineIds.length === 0) {
      return false;
    }
    return (
      effectiveSelectedSet.size === onlineIds.length &&
      onlineIds.every((id) => effectiveSelectedSet.has(id))
    );
  }, [effectiveSelectedSet, onlineIds, selectedIds]);

  const trimmedQuery = query.trim().toLowerCase();

  const nodeGroups = useMemo(() => {
    const groups = [
      {
        key: 'online' as const,
        label: t('common.onlineNodesGroup'),
        defaultOpen: true,
        nodes: onlineNodes.filter((node) =>
          matchesNodeQuery(node, trimmedQuery),
        ),
      },
      {
        key: 'offline' as const,
        label: t('common.offlineNodesGroup'),
        defaultOpen: false,
        nodes: offlineNodes.filter((node) =>
          matchesNodeQuery(node, trimmedQuery),
        ),
      },
    ];
    return groups.filter((group) => group.nodes.length > 0);
  }, [offlineNodes, onlineNodes, t, trimmedQuery]);

  const commitSelection = (nextSet: Set<string>) => {
    if (
      onlineIds.length > 0 &&
      nextSet.size === onlineIds.length &&
      onlineIds.every((id) => nextSet.has(id))
    ) {
      onChange(null);
      return;
    }
    onChange(Array.from(nextSet));
  };

  const toggleNode = (nodeId: string, checked: boolean) => {
    const next = new Set(effectiveSelectedSet);
    if (checked) {
      next.add(nodeId);
    } else {
      next.delete(nodeId);
    }
    commitSelection(next);
  };

  const toggleGroup = (nodes: NodeItem[], checked: boolean) => {
    const next = new Set(effectiveSelectedSet);
    for (const node of nodes) {
      if (checked) {
        next.add(node.node_id);
      } else {
        next.delete(node.node_id);
      }
    }
    commitSelection(next);
  };

  const badgeLabel = nodesQuery.isLoading
    ? t('common.loading')
    : isAllOnlineSelected && onlineIds.length > 0
      ? t('common.allOnlineSelected', { online: onlineIds.length })
      : t('common.selectedCount', {
          selected: effectiveSelectedCount,
          online: onlineIds.length,
        });

  const isCustomHighlight =
    selectedIds !== null && !(isAllOnlineSelected && onlineIds.length > 0);

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) {
          setQuery('');
        }
      }}
    >
      <PopoverTrigger asChild>
        <Button
          type='button'
          variant='outline'
          className={cn(
            'h-9 gap-2 text-xs',
            isCustomHighlight ? 'border-primary text-primary' : undefined,
          )}
        >
          <Server className='size-3.5 shrink-0 text-primary' />
          <span>{t('common.nodeSelectorTitle')}</span>
          <Badge
            variant='secondary'
            className='px-1.5 py-0 text-[11px] font-normal tabular-nums'
          >
            {badgeLabel}
          </Badge>
          <ChevronDown className='size-3.5 shrink-0 text-muted-foreground' />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align='start'
        className='w-96 max-w-[calc(100vw-2rem)] space-y-3 p-3'
      >
        <div className='flex items-center justify-between gap-2'>
          <p className='text-sm font-medium'>{t('common.nodeSelectorTitle')}</p>
          <div className='flex items-center gap-1'>
            {!isAllOnlineSelected && onlineIds.length > 0 ? (
              <Button
                type='button'
                variant='ghost'
                size='sm'
                className='h-7 px-2 text-xs'
                onClick={() => onChange(null)}
              >
                {t('common.selectAllOnline')}
              </Button>
            ) : null}
            {effectiveSelectedCount > 0 ? (
              <Button
                type='button'
                variant='ghost'
                size='sm'
                className='h-7 px-2 text-xs'
                onClick={() => onChange([])}
              >
                <X className='mr-1 size-3' />
                {t('common.clearSelection')}
              </Button>
            ) : null}
          </div>
        </div>

        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t('common.nodeSearchPlaceholder')}
          className='h-8 text-xs'
        />

        <div className='max-h-72 space-y-2 overflow-y-auto hide-scrollbar'>
          {nodesQuery.isLoading ? (
            <div className='flex items-center justify-center gap-2 py-6 text-xs text-muted-foreground'>
              <Loader2 className='size-3.5 animate-spin' />
              {t('common.loading')}
            </div>
          ) : nodesQuery.isError ? (
            <div className='space-y-2 py-2'>
              <p className='text-xs text-destructive'>
                {t('common.loadNodesFailed')}
              </p>
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='h-7 text-xs'
                onClick={() => void nodesQuery.refetch()}
              >
                {t('common.retry')}
              </Button>
            </div>
          ) : edgeNodes.length === 0 ? (
            <p className='py-6 text-center text-xs text-muted-foreground'>
              {t('common.noOnlineNodes')}
            </p>
          ) : nodeGroups.length === 0 ? (
            <p className='py-6 text-center text-xs text-muted-foreground'>
              {t('common.noMatchingNodes')}
            </p>
          ) : (
            nodeGroups.map((group) => {
              const selectedInGroup = group.nodes.filter((node) =>
                effectiveSelectedSet.has(node.node_id),
              ).length;
              const allSelected = selectedInGroup === group.nodes.length;
              const partialSelected =
                selectedInGroup > 0 && selectedInGroup < group.nodes.length;
              const expanded =
                trimmedQuery.length > 0
                  ? true
                  : (expandedGroups[group.key] ?? group.defaultOpen);

              return (
                <Collapsible
                  key={group.key}
                  open={expanded}
                  onOpenChange={(next) =>
                    setExpandedGroups((prev) => ({
                      ...prev,
                      [group.key]: next,
                    }))
                  }
                  className='rounded-md border border-dashed'
                >
                  <div className='flex items-center gap-1 px-2 py-1.5'>
                    <Checkbox
                      checked={
                        allSelected
                          ? true
                          : partialSelected
                            ? 'indeterminate'
                            : false
                      }
                      onCheckedChange={(checked) =>
                        toggleGroup(group.nodes, checked === true)
                      }
                      aria-label={t('common.selectNodeGroup', {
                        name: group.label,
                      })}
                    />
                    <CollapsibleTrigger asChild>
                      <button
                        type='button'
                        className='flex min-w-0 flex-1 items-center gap-1 rounded-md px-1 py-0.5 text-left text-xs font-medium hover:bg-accent'
                      >
                        <ChevronDown
                          className={cn(
                            'size-3.5 shrink-0 text-muted-foreground transition-transform',
                            expanded ? 'rotate-0' : '-rotate-90',
                          )}
                        />
                        <span className='min-w-0 flex-1 truncate'>
                          {group.label}
                        </span>
                        <span className='text-[10px] text-muted-foreground tabular-nums'>
                          {selectedInGroup}/{group.nodes.length}
                        </span>
                      </button>
                    </CollapsibleTrigger>
                  </div>
                  <CollapsibleContent>
                    <div className='space-y-0.5 border-t border-dashed px-2 py-1.5'>
                      {group.nodes.map((node) => {
                        const checked = effectiveSelectedSet.has(node.node_id);
                        return (
                          <label
                            key={node.node_id}
                            className={cn(
                              'flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-accent',
                              checked ? 'bg-accent/50' : undefined,
                            )}
                          >
                            <Checkbox
                              checked={checked}
                              onCheckedChange={(nextChecked) =>
                                toggleNode(node.node_id, nextChecked === true)
                              }
                              aria-label={t('common.selectNode', {
                                name: node.name,
                              })}
                            />
                            <span
                              className='min-w-0 flex-1 truncate font-medium'
                              title={node.name}
                            >
                              {node.name}
                            </span>
                            <span className='flex shrink-0 items-center gap-1'>
                              {node.asn ? (
                                <Badge
                                  variant='outline'
                                  className='font-mono text-[10px]'
                                >
                                  AS{node.asn}
                                </Badge>
                              ) : null}
                              {node.geo_name ? (
                                <Badge
                                  variant='secondary'
                                  className='max-w-36 truncate text-[10px]'
                                  title={node.geo_name}
                                >
                                  {node.geo_name}
                                </Badge>
                              ) : null}
                            </span>
                          </label>
                        );
                      })}
                    </div>
                  </CollapsibleContent>
                </Collapsible>
              );
            })
          )}
        </div>

        <p className='border-t border-dashed pt-2 text-[11px] text-muted-foreground'>
          {t('common.nodeSelectorHint')}
        </p>
      </PopoverContent>
    </Popover>
  );
}
