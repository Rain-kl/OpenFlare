'use client';

import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Server } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { NodeService } from '@/lib/services/openflare';

interface NodeSelectorProps {
  selectedIds: string[];
  onChange: (ids: string[]) => void;
}

export function NodeSelector({ selectedIds, onChange }: NodeSelectorProps) {
  const t = useTranslations('networkTools');

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

  const onlineIds = useMemo(
    () => edgeNodes.filter((n) => n.status === 'online').map((n) => n.node_id),
    [edgeNodes],
  );

  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds]);

  const toggleNode = (nodeId: string) => {
    if (selectedSet.has(nodeId)) {
      onChange(selectedIds.filter((id) => id !== nodeId));
    } else {
      onChange([...selectedIds, nodeId]);
    }
  };

  return (
    <Card>
      <CardHeader className='pb-3'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='space-y-1'>
            <CardTitle className='flex items-center gap-2 text-sm font-medium'>
              <Server className='size-4 text-primary' />
              {t('common.nodeSelectorTitle')}
              <Badge variant='secondary'>
                {t('common.selectedCount', {
                  selected: selectedIds.length,
                  online: onlineIds.length,
                })}
              </Badge>
            </CardTitle>
            <p className='text-xs text-muted-foreground'>
              {t('common.nodeSelectorHint')}
            </p>
          </div>
          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={onlineIds.length === 0}
              onClick={() => onChange(onlineIds)}
            >
              {t('common.selectAllOnline')}
            </Button>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              disabled={selectedIds.length === 0}
              onClick={() => onChange([])}
            >
              {t('common.clearSelection')}
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {edgeNodes.length === 0 ? (
          <p className='text-xs text-muted-foreground'>
            {nodesQuery.isLoading
              ? t('common.loading')
              : t('common.noOnlineNodes')}
          </p>
        ) : (
          <div className='grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4'>
            {edgeNodes.map((node) => {
              const checked = selectedSet.has(node.node_id);
              const isOnline = node.status === 'online';
              return (
                <label
                  key={node.node_id}
                  className='flex cursor-pointer items-center justify-between gap-2 rounded-lg border border-dashed px-3 py-2 text-xs transition-colors hover:bg-muted/40'
                >
                  <span className='flex min-w-0 items-center gap-2'>
                    <Checkbox
                      checked={checked}
                      onCheckedChange={() => toggleNode(node.node_id)}
                    />
                    <span className='truncate font-medium'>{node.name}</span>
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
                    <Badge
                      variant={isOnline ? 'default' : 'secondary'}
                      className='text-[10px]'
                    >
                      {node.geo_name || node.status}
                    </Badge>
                  </span>
                </label>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
