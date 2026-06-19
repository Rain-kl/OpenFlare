'use client';

import Link from 'next/link';
import {useMemo} from 'react';
import {useSearchParams} from 'next/navigation';
import {useQuery} from '@tanstack/react-query';
import {ArrowLeft, Server} from 'lucide-react';

import {Button} from '@/components/ui/button';
import {EmptyStateWithBorder} from '@/components/layout/empty';
import {ErrorInline} from '@/components/layout/error';
import {LoadingStateWithBorder} from '@/components/layout/loading';
import {NodeService} from '@/lib/services/openflare';

import {EdgeNodeDetail} from '../components/edge-node-detail';
import {RelayNodeDetail} from '../components/relay-node-detail';
import {TunnelNodeDetail} from '../components/tunnel-node-detail';
import {getErrorMessage} from '../components/node-utils';

const nodesQueryKey = ['openflare', 'nodes'];

export function NodeDetailPageClient() {
  const searchParams = useSearchParams();
  const nodeId = searchParams.get('id')?.trim() ?? '';

  const nodesQuery = useQuery({
    queryKey: nodesQueryKey,
    queryFn: () => NodeService.listNodes(),
    refetchInterval: 5000,
  });

  const node = useMemo(() => {
    if (!nodeId) return null;
    return (nodesQuery.data ?? []).find((item) => String(item.id) === nodeId) ?? null;
  }, [nodeId, nodesQuery.data]);

  if (!nodeId) {
    return (
      <div className="py-6 px-1">
        <EmptyStateWithBorder
          icon={Server}
          description="缺少节点 ID，请从节点列表进入详情页。"
        />
      </div>
    );
  }

  if (nodesQuery.isLoading) {
    return (
      <div className="py-6 px-1">
        <LoadingStateWithBorder icon={Server} description="加载节点详情中..." />
      </div>
    );
  }

  if (nodesQuery.isError) {
    return (
      <div className="py-6 px-1">
        <div className="p-8 border border-dashed rounded-lg">
          <ErrorInline
            message={getErrorMessage(nodesQuery.error)}
            onRetry={() => void nodesQuery.refetch()}
            className="justify-center"
          />
        </div>
      </div>
    );
  }

  if (!node) {
    return (
      <div className="py-6 px-1 space-y-4">
        <Button variant="ghost" size="sm" className="h-8 px-2" asChild>
          <Link href="/nodes">
            <ArrowLeft className="size-4 mr-1" />
            返回节点列表
          </Link>
        </Button>
        <EmptyStateWithBorder
          icon={Server}
          description="节点不存在，可能已被删除或 ID 无效。"
        />
      </div>
    );
  }

  if (node.node_type === 'tunnel_relay') {
    return <RelayNodeDetail node={node} />;
  }

  if (node.node_type === 'tunnel_client') {
    return <TunnelNodeDetail node={node} />;
  }

  return <EdgeNodeDetail node={node} />;
}
