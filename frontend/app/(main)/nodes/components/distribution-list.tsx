'use client';

import { useTranslations } from 'next-intl';

import { Progress } from '@/components/ui/progress';

import { formatCompactNumber } from '../../components/dashboard/dashboard-utils';

export function DistributionList({
  items,
  emptyMessage,
}: {
  items: Array<{ label: string; value: number }>;
  emptyMessage?: string;
}) {
  const t = useTranslations('nodes.obs');
  const resolvedEmpty = emptyMessage ?? t('distEmpty');
  const sortedItems = [...items]
    .sort((left, right) => right.value - left.value)
    .slice(0, 8);
  const maxValue = sortedItems[0]?.value ?? 0;

  if (sortedItems.length === 0) {
    return (
      <div className='flex min-h-[180px] items-center justify-center text-xs text-muted-foreground'>
        {resolvedEmpty}
      </div>
    );
  }

  return (
    <div className='space-y-3'>
      {sortedItems.map((item) => {
        const ratio = maxValue > 0 ? (item.value / maxValue) * 100 : 0;
        return (
          <div key={item.label} className='space-y-1.5'>
            <div className='flex items-center justify-between gap-3 text-xs'>
              <span className='truncate font-medium'>
                {item.label || t('unknownLabel')}
              </span>
              <span className='shrink-0 font-mono tabular-nums text-muted-foreground'>
                {formatCompactNumber(item.value)}
              </span>
            </div>
            <Progress value={ratio} className='h-1.5' />
          </div>
        );
      })}
    </div>
  );
}
