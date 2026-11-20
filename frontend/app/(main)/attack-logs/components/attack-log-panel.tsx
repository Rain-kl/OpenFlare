import type { ReactNode } from 'react';
import { useFormatter, useTranslations } from 'next-intl';

import { EmptyState } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingState } from '@/components/layout/loading';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
} from '@/components/ui/card';

import { attackLogPageSize } from './attack-log-filters';

export function AttackLogPanel({
  filters,
  children,
  loading,
  failed,
  refresh,
  total,
  page,
  onPage,
}: {
  filters: ReactNode;
  children: ReactNode;
  loading: boolean;
  failed: boolean;
  refresh: () => void;
  total: number;
  page: number;
  onPage: (page: number) => void;
}) {
  const t = useTranslations('attackLogs');
  const format = useFormatter();
  return (
    <Card>
      <CardHeader>{filters}</CardHeader>
      <CardContent className='px-0'>
        {loading ? (
          <LoadingState />
        ) : failed ? (
          <ErrorInline
            className='px-5'
            message={t('loadFailed')}
            onRetry={refresh}
          />
        ) : total === 0 ? (
          <EmptyState title={t('empty')} description={t('defaultPeriod')} />
        ) : (
          children
        )}
      </CardContent>
      {!loading && !failed && (total > 0 || page > 0) && (
        <CardFooter className='flex flex-wrap items-center justify-between gap-3'>
          <p className='text-sm text-muted-foreground'>
            {t('total', { count: format.number(total) })}
          </p>
          <div className='flex gap-2'>
            <Button
              variant='outline'
              size='sm'
              disabled={page === 0}
              onClick={() => onPage(page - 1)}
            >
              {t('previous')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              disabled={(page + 1) * attackLogPageSize >= total}
              onClick={() => onPage(page + 1)}
            >
              {t('next')}
            </Button>
          </div>
        </CardFooter>
      )}
    </Card>
  );
}
