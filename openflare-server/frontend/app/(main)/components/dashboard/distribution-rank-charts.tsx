'use client';

import {RankChart} from '@/components/data/rank-chart';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type {DistributionItem, TrafficDistributions} from '@/lib/services/openflare';

function toRankItems(items: DistributionItem[]) {
  return items.map((item) => ({
    label: item.key,
    value: item.value,
  }));
}

export function SourceDistributionChart({
  items,
}: {
  items: TrafficDistributions['source_countries'];
}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader>
        <CardTitle className="text-sm font-semibold">来源分布</CardTitle>
        <CardDescription className="text-xs">
          聚合最近 24 小时主要来源国家。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <RankChart
          items={toRankItems(items)}
          color="#38bdf8"
          emptyMessage="暂无来源分布数据"
        />
      </CardContent>
    </Card>
  );
}

export function StatusCodeDistributionChart({
  items,
}: {
  items: TrafficDistributions['status_codes'];
}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader>
        <CardTitle className="text-sm font-semibold">状态码分布</CardTitle>
        <CardDescription className="text-xs">
          快速判断成功响应是否仍是主流，以及错误码是否有抬升。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <RankChart
          items={toRankItems(items).map((item) => ({
            ...item,
            label: `HTTP ${item.label}`,
          }))}
          color="#f59e0b"
          emptyMessage="暂无状态码分布"
        />
      </CardContent>
    </Card>
  );
}

export function TopDomainChart({
  items,
}: {
  items: TrafficDistributions['top_domains'];
}) {
  return (
    <Card className="border-dashed shadow-none">
      <CardHeader>
        <CardTitle className="text-sm font-semibold">Top Domain</CardTitle>
        <CardDescription className="text-xs">
          观察主要流量集中在哪些域名。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <RankChart
          items={toRankItems(items)}
          color="#34d399"
          emptyMessage="暂无域名分布"
        />
      </CardContent>
    </Card>
  );
}