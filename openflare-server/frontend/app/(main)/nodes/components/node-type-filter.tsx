'use client';

import Link from 'next/link';
import {useSearchParams} from 'next/navigation';

import {cn} from '@/lib/utils';

export type NodeFilter = 'all' | 'edge' | 'relay' | 'tunnel';

const filters: Array<{ key: NodeFilter; label: string; href: string }> = [
  { key: 'all', label: '全部节点', href: '/nodes' },
  { key: 'edge', label: 'Edge', href: '/nodes?filter=edge' },
  { key: 'relay', label: 'Relay', href: '/nodes?filter=relay' },
  { key: 'tunnel', label: 'Tunnel', href: '/nodes?filter=tunnel' },
];

export function getNodeFilter(searchParams: URLSearchParams): NodeFilter {
  const current = searchParams.get('filter')?.trim().toLowerCase() ?? '';
  if (current === 'relay' || current === 'tunnel' || current === 'edge' || current === 'all') {
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

export function getFilterDescription(filter: NodeFilter) {
  switch (filter) {
    case 'relay':
      return '当前仅展示 Relay 节点。';
    case 'tunnel':
      return '当前仅展示 Tunnel 节点。';
    case 'edge':
      return '当前仅展示 Edge 节点。';
    case 'all':
    default:
      return '当前展示全部节点。';
  }
}

export function NodeTypeFilter() {
  const searchParams = useSearchParams();
  const activeFilter = getNodeFilter(searchParams);

  return (
    <div className="flex flex-wrap gap-2">
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
          {item.label}
        </Link>
      ))}
    </div>
  );
}
