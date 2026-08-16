'use client';

import { useTranslations } from 'next-intl';

import { RankChart } from '@/components/data/rank-chart';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type {
  DistributionItem,
  TrafficDistributions,
} from '@/lib/services/openflare';

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
  const t = useTranslations('dashboard.distributions');
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>
          {t('sourceTitle')}
        </CardTitle>
        <CardDescription className='text-xs'>{t('sourceDesc')}</CardDescription>
      </CardHeader>
      <CardContent>
        <RankChart
          items={toRankItems(items)}
          color='#38bdf8'
          emptyMessage={t('sourceEmpty')}
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
  const t = useTranslations('dashboard.distributions');
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>
          {t('statusTitle')}
        </CardTitle>
        <CardDescription className='text-xs'>{t('statusDesc')}</CardDescription>
      </CardHeader>
      <CardContent>
        <RankChart
          items={toRankItems(items).map((item) => ({
            ...item,
            label: t('httpLabel', { code: item.label }),
          }))}
          color='#f59e0b'
          emptyMessage={t('statusEmpty')}
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
  const t = useTranslations('dashboard.distributions');
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader>
        <CardTitle className='text-sm font-semibold'>
          {t('domainTitle')}
        </CardTitle>
        <CardDescription className='text-xs'>{t('domainDesc')}</CardDescription>
      </CardHeader>
      <CardContent>
        <RankChart
          items={toRankItems(items)}
          color='#34d399'
          emptyMessage={t('domainEmpty')}
        />
      </CardContent>
    </Card>
  );
}
