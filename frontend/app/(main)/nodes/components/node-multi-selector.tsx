'use client';

import { useMemo, useState } from 'react';
import { Search } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import type { NodeItem } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import { NodeStatusBadge } from './node-status-badge';
import { getNodeStatusLabel, getNodeStatusTone } from './node-utils';

export function NodeMultiSelector({
  nodes,
  value,
  onChange,
  maxHeightClass = 'max-h-60',
}: {
  nodes: NodeItem[];
  value: string[];
  onChange: (next: string[]) => void;
  maxHeightClass?: string;
}) {
  const t = useTranslations('nodes');
  const [search, setSearch] = useState('');

  const filteredNodes = useMemo(() => {
    const q = search.trim().toLowerCase();
    const sorted = [...nodes].sort((a, b) => {
      if (a.status === 'online' && b.status !== 'online') return -1;
      if (a.status !== 'online' && b.status === 'online') return 1;
      return a.name.localeCompare(b.name);
    });
    if (!q) return sorted;
    return sorted.filter(
      (item) =>
        item.name.toLowerCase().includes(q) ||
        item.node_id.toLowerCase().includes(q) ||
        item.ip.toLowerCase().includes(q),
    );
  }, [nodes, search]);

  const selectedSet = useMemo(() => new Set(value), [value]);

  const toggleNode = (nodeId: string) => {
    if (selectedSet.has(nodeId)) {
      onChange(value.filter((id) => id !== nodeId));
    } else {
      onChange([...value, nodeId]);
    }
  };

  const selectAllOnline = () => {
    const onlineIds = nodes
      .filter((item) => item.status === 'online')
      .map((item) => item.node_id);
    onChange(onlineIds);
  };

  const selectAll = () => {
    onChange(nodes.map((item) => item.node_id));
  };

  const clearSelection = () => {
    onChange([]);
  };

  return (
    <div className='space-y-2.5'>
      <div className='flex flex-wrap items-center gap-2'>
        <div className='relative min-w-[180px] flex-1'>
          <Search className='pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground' />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('pagination.searchPlaceholder')}
            className='h-8 pl-8 text-xs'
          />
        </div>
        <div className='flex items-center gap-1'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='h-8 text-xs'
            onClick={selectAllOnline}
          >
            {t('selector.selectOnline')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='h-8 text-xs'
            onClick={selectAll}
          >
            {t('selector.selectAll')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='h-8 text-xs'
            onClick={clearSelection}
          >
            {t('selector.clear')}
          </Button>
        </div>
      </div>

      <div
        className={cn(
          'overflow-y-auto rounded-lg border border-dashed divide-y divide-dashed',
          maxHeightClass,
        )}
      >
        {filteredNodes.length === 0 ? (
          <div className='py-6 text-center text-xs text-muted-foreground'>
            {t('selector.empty')}
          </div>
        ) : (
          filteredNodes.map((node) => {
            const checked = selectedSet.has(node.node_id);
            return (
              <label
                key={node.node_id}
                className='flex cursor-pointer items-center justify-between gap-3 px-3 py-2 text-sm hover:bg-muted/30 transition-colors'
              >
                <div className='flex min-w-0 items-center gap-2.5'>
                  <Checkbox
                    checked={checked}
                    onCheckedChange={() => toggleNode(node.node_id)}
                  />
                  <div className='min-w-0'>
                    <p className='truncate text-sm font-medium text-foreground'>
                      {node.name}
                    </p>
                    <p className='truncate font-mono text-[11px] text-muted-foreground'>
                      {node.node_id}
                      {node.ip ? ` · ${node.ip}` : ''}
                    </p>
                  </div>
                </div>
                <NodeStatusBadge
                  label={getNodeStatusLabel(node.status, t)}
                  tone={getNodeStatusTone(node.status)}
                />
              </label>
            );
          })
        )}
      </div>
    </div>
  );
}
