'use client';

import {RankChart} from '@/components/data/rank-chart';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type {DashboardNodeHealth} from '@/lib/services/openflare';

function buildNodeRankItems(
  nodes: DashboardNodeHealth[],
  selector: (node: DashboardNodeHealth) => number,
  limit = 5,
) {
  return [...nodes]
    .sort((left, right) => {
      const leftValue = selector(left);
      const rightValue = selector(right);
      if (leftValue === rightValue) {
        return left.name.localeCompare(right.name, 'zh-CN');
      }
      return rightValue - leftValue;
    })
    .slice(0, limit)
    .filter((node) => selector(node) > 0)
    .map((node) => ({
      label: node.name,
      value: selector(node),
    }));
}

export function NodeRankPanel({nodes}: {nodes: DashboardNodeHealth[]}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader>
        <CardTitle className="text-sm font-semibold">Top 节点榜单</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-6">
        <div>
          <p className="mb-3 text-xs tracking-[0.22em] text-muted-foreground uppercase">
            流量最高节点
          </p>
          <RankChart
            items={buildNodeRankItems(nodes, (node) => node.request_count)}
            color="#38bdf8"
            emptyMessage="暂无流量榜单"
          />
        </div>
        <div>
          <p className="mb-3 text-xs tracking-[0.22em] text-muted-foreground uppercase">
            容量压力节点
          </p>
          <RankChart
            items={buildNodeRankItems(nodes, (node) =>
              Math.round(
                Math.max(
                  node.cpu_usage_percent,
                  node.memory_usage_percent,
                  node.storage_usage_percent,
                ),
              ),
            )}
            color="#ef4444"
            valueFormatter={(value) => `${value}%`}
            emptyMessage="暂无容量压力数据"
          />
        </div>
      </CardContent>
    </Card>
  );
}