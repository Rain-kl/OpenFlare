'use client';

import { Clock, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import {
  ListActionButton,
  ListActions,
} from '@/components/common/list-actions';
import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import type { ExecTask } from '@/lib/services/openflare';
import { cn, formatDateTime } from '@/lib/utils';

export function ExecHistoryList({
  tasks,
  selectedTaskId,
  onSelect,
  onDelete,
}: {
  tasks: ExecTask[];
  selectedTaskId: string | null;
  onSelect: (task: ExecTask) => void;
  onDelete: (task: ExecTask) => void;
}) {
  const t = useTranslations('nodeExec');

  return (
    <Card>
      <CardHeader className='pb-3'>
        <CardTitle className='text-base font-semibold'>
          {t('history.title')}
        </CardTitle>
        <CardDescription>{t('history.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        {tasks.length === 0 ? (
          <div className='rounded-lg border border-dashed py-8 text-center text-sm text-muted-foreground'>
            {t('history.empty')}
          </div>
        ) : (
          <div className='space-y-2'>
            {tasks.map((task) => {
              const total = task.results?.length ?? task.clients.length;
              const finished = (task.results ?? []).filter(
                (r) => r.finished_at !== null || r.exit_code !== null,
              ).length;
              const failed = (task.results ?? []).filter(
                (r) => r.exit_code !== null && r.exit_code !== 0,
              ).length;
              const active = selectedTaskId === task.task_id;

              return (
                <div
                  key={task.task_id}
                  role='button'
                  tabIndex={0}
                  onClick={() => onSelect(task)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      onSelect(task);
                    }
                  }}
                  className={cn(
                    'flex cursor-pointer items-start justify-between gap-3 rounded-lg border border-dashed px-3.5 py-3 transition-colors hover:border-primary/40 hover:bg-muted/20',
                    active && 'border-primary bg-muted/30',
                  )}
                >
                  <div className='min-w-0 flex-1 space-y-1.5'>
                    <p className='truncate font-mono text-xs font-medium text-foreground'>
                      {task.command}
                    </p>
                    <div className='flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground'>
                      <span className='inline-flex items-center gap-1 tabular-nums'>
                        <Clock className='size-3' />
                        {formatDateTime(task.created_at)}
                      </span>
                      <Badge variant='secondary' className='text-[11px]'>
                        {t('history.progress', { finished, total })}
                      </Badge>
                      {failed > 0 ? (
                        <Badge variant='destructive' className='text-[11px]'>
                          {t('history.failedCount', { count: failed })}
                        </Badge>
                      ) : null}
                    </div>
                  </div>

                  <ListActions align='end' stopPropagation className='shrink-0'>
                    <ListActionButton
                      icon={Trash2}
                      label={t('history.delete')}
                      tone='destructive'
                      onClick={() => onDelete(task)}
                    />
                  </ListActions>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
