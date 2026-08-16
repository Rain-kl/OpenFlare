'use client';

import { useTranslations } from 'next-intl';

import { RankChart } from '@/components/data/rank-chart';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type { DashboardNodeHealth } from '@/lib/services/openflare';

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

export function NodeRankPanel({ nodes }: { nodes: DashboardNodeHealth[] }) {
  const t = useTranslations('dashboard.nodeRank');
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>{t('title')}</CardTitle>
      </CardHeader>
      <CardContent className='grid gap-6'>
        <div>
          <p className='mb-3 text-xs tracking-[0.22em] text-muted-foreground uppercase'>
            {t('traffic')}
          </p>
          <RankChart
            items={buildNodeRankItems(nodes, (node) => node.request_count)}
            color='#38bdf8'
            emptyMessage={t('trafficEmpty')}
          />
        </div>
        <div>
          <p className='mb-3 text-xs tracking-[0.22em] text-muted-foreground uppercase'>
            {t('pressure')}
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
            color='#ef4444'
            valueFormatter={(value) => `${value}%`}
            emptyMessage={t('pressureEmpty')}
          />
        </div>
      </CardContent>
    </Card>
  );
}
