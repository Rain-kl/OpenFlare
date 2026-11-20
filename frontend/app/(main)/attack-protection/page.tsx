'use client';

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { List, Plus, Shield } from 'lucide-react';
import { useFormatter, useTranslations } from 'next-intl';
import { useState } from 'react';

import { EmptyState } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingState } from '@/components/layout/loading';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  attackLevels,
  type AttackGroup,
} from '@/lib/services/openflare/attack-types';
import {
  attackGroupsQueryKey,
  WafService,
} from '@/lib/services/openflare/waf.service';

import { AttackGroupEditor } from './attack-group-editor';
import { AttackRuleCatalog } from './attack-rule-catalog';

export default function AttackProtectionPage() {
  const t = useTranslations('attackProtection');
  const format = useFormatter();
  const client = useQueryClient();
  const groups = useQuery({
    queryKey: attackGroupsQueryKey,
    queryFn: () => WafService.listAttackGroups(),
  });
  const [editor, setEditor] = useState<{ group?: AttackGroup } | null>(null);

  return (
    <div className='w-full space-y-5 px-1 py-6'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <Shield className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('title')}
          </h1>
        </div>
        <div className='flex flex-wrap gap-2'>
          <AttackRuleCatalog>
            <Button variant='outline'>
              <List data-icon='inline-start' />
              {t('catalog.title')}
            </Button>
          </AttackRuleCatalog>
          {!editor && (
            <Button onClick={() => setEditor({})}>
              <Plus data-icon='inline-start' />
              {t('create')}
            </Button>
          )}
        </div>
      </div>
      {editor ? (
        <AttackGroupEditor
          group={editor.group}
          onCancel={() => setEditor(null)}
          onSaved={async () => {
            await client.invalidateQueries({ queryKey: attackGroupsQueryKey });
            setEditor(null);
          }}
        />
      ) : (
        <Card className='py-1'>
          <CardContent className='px-0'>
            {groups.isPending && <LoadingState />}
            {groups.isError && (
              <ErrorInline
                className='p-5'
                message={t('loadFailed')}
                onRetry={() => void groups.refetch()}
              />
            )}
            {groups.isSuccess &&
              (groups.data.length === 0 ? (
                <EmptyState title={t('empty')} description={t('description')} />
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className='pl-5'>{t('name')}</TableHead>
                      <TableHead>{t('configuredModules')}</TableHead>
                      <TableHead>{t('updatedAt')}</TableHead>
                      <TableHead className='pr-5 text-right'>
                        {t('edit')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {groups.data.map((group) => (
                      <TableRow key={group.id}>
                        <TableCell className='pl-5 font-medium'>
                          {group.name}
                        </TableCell>
                        <TableCell>
                          {attackLevels
                            .slice(1)
                            .map((level) => ({
                              level,
                              count: Object.values(group.levels).filter(
                                (value) => value === level,
                              ).length,
                            }))
                            .filter(({ count }) => count > 0)
                            .map(({ level, count }) => (
                              <span
                                key={level}
                                className='mr-4 text-muted-foreground'
                              >
                                {t(`levels.${level}`)} · {format.number(count)}
                              </span>
                            ))}
                          {!Object.values(group.levels).some(
                            (level) => level !== 'off',
                          ) && t('levels.off')}
                        </TableCell>
                        <TableCell>
                          {format.dateTime(new Date(group.updated_at), {
                            dateStyle: 'medium',
                            timeStyle: 'short',
                          })}
                        </TableCell>
                        <TableCell className='pr-5 text-right'>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() => setEditor({ group })}
                          >
                            {t('edit')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
