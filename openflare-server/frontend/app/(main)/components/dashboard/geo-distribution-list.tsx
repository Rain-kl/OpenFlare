'use client';

import {Globe2} from 'lucide-react';

import {Card, CardContent, CardDescription, CardHeader, CardTitle,} from '@/components/ui/card';
import {Progress} from '@/components/ui/progress';
import type {DistributionItem} from '@/lib/services/openflare';

import {formatCompactNumber} from './dashboard-utils';

export function GeoDistributionList({ items }: { items: DistributionItem[] }) {
  const sortedItems = [...items]
    .sort((left, right) => right.value - left.value)
    .slice(0, 8);
  const maxValue = sortedItems[0]?.value ?? 0;

  return (
    <Card className="border-dashed shadow-none">
      <CardHeader>
        <CardTitle className="text-sm font-semibold flex items-center gap-1.5">
          <Globe2 className="size-4 text-primary" />
          来源国家分布
        </CardTitle>
        <CardDescription className="text-xs">
          聚合最近 24 小时主要来源国家。
        </CardDescription>
      </CardHeader>
      <CardContent>
        {sortedItems.length === 0 ? (
          <div className="flex min-h-[180px] items-center justify-center text-xs text-muted-foreground">
            暂无来源分布数据
          </div>
        ) : (
          <div className="space-y-3">
            {sortedItems.map((item) => {
              const ratio = maxValue > 0 ? (item.value / maxValue) * 100 : 0;
              return (
                <div key={item.key} className="space-y-1.5">
                  <div className="flex items-center justify-between text-xs">
                    <span className="font-medium">{item.key || '未知'}</span>
                    <span className="font-mono tabular-nums text-muted-foreground">
                      {formatCompactNumber(item.value)}
                    </span>
                  </div>
                  <Progress value={ratio} className="h-1.5" />
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
