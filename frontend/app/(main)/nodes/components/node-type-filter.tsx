'use client';

import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useTranslations } from 'next-intl';

import { cn } from '@/lib/utils';

export type NodeFilter = 'all' | 'edge' | 'relay' | 'tunnel';

const filters: Array<{ key: NodeFilter; href: string }> = [
  { key: 'all', href: '/nodes' },
  { key: 'edge', href: '/nodes?filter=edge' },
  { key: 'relay', href: '/nodes?filter=relay' },
  { key: 'tunnel', href: '/nodes?filter=tunnel' },
];

export function getNodeFilter(searchParams: URLSearchParams): NodeFilter {
  const current = searchParams.get('filter')?.trim().toLowerCase() ?? '';
  if (
    current === 'relay' ||
    current === 'tunnel' ||
    current === 'edge' ||
    current === 'all'
  ) {
    return current;
  }
  return 'all';
}

export function filterNodesByType<T extends { node_type: string }>(
  nodes: T[],
  filter: NodeFilter,
): T[] {
  switch (filter) {
    case 'relay':
      return nodes.filter((node) => node.node_type === 'tunnel_relay');
    case 'tunnel':
      return nodes.filter((node) => node.node_type === 'tunnel_client');
    case 'edge':
      return nodes.filter((node) => node.node_type === 'edge_node');
    case 'all':
    default:
      return nodes;
  }
}

export function getFilterDescription(
  filter: NodeFilter,
  t: (key: string) => string,
) {
  switch (filter) {
    case 'relay':
      return t('filter.descRelay');
    case 'tunnel':
      return t('filter.descTunnel');
    case 'edge':
      return t('filter.descEdge');
    case 'all':
    default:
      return t('filter.descAll');
  }
}

export function NodeTypeFilter() {
  const t = useTranslations('nodes');
  const searchParams = useSearchParams();
  const activeFilter = getNodeFilter(searchParams);
  const filterLabels: Record<NodeFilter, string> = {
    all: t('filter.all'),
    edge: 'Edge',
    relay: 'Relay',
    tunnel: 'Tunnel',
  };

  return (
    <div className='flex flex-wrap gap-2'>
      {filters.map((item) => (
        <Link
          key={item.key}
          href={item.href}
          className={cn(
            'inline-flex items-center rounded-full border px-3 py-1.5 text-xs transition',
            activeFilter === item.key
              ? 'border-foreground/30 bg-accent text-foreground'
              : 'border-border text-muted-foreground hover:bg-muted/50',
          )}
        >
          {filterLabels[item.key]}
        </Link>
      ))}
    </div>
  );
}
