'use client';

import { Download, Eye, Pencil, Play, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { WAFIPGroup, WAFIPGroupType } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

interface IPGroupsTableProps {
  groups: WAFIPGroup[];
  syncingId: number | null;
  onView: (group: WAFIPGroup) => void;
  onEdit: (group: WAFIPGroup) => void;
  onDelete: (group: WAFIPGroup) => void;
  onSync: (group: WAFIPGroup) => void;
  onTest: (group: WAFIPGroup) => void;
}

export function IPGroupsTable({
  groups,
  syncingId,
  onView,
  onEdit,
  onDelete,
  onSync,
  onTest,
}: IPGroupsTableProps) {
  const t = useTranslations('ipGroups');
  const typeLabel = (type: WAFIPGroupType) => t(`types.${type}`);
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('columns.name')}</TableHead>
          <TableHead>{t('columns.type')}</TableHead>
          <TableHead>{t('columns.status')}</TableHead>
          <TableHead>{t('columns.ipCount')}</TableHead>
          <TableHead>{t('columns.refCount')}</TableHead>
          <TableHead>{t('columns.syncStatus')}</TableHead>
          <TableHead>{t('columns.updatedAt')}</TableHead>
          <TableHead className='w-[168px] text-right'>
            {t('columns.actions')}
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {groups.map((group) => (
          <TableRow key={group.id}>
            <TableCell className='font-medium'>{group.name}</TableCell>
            <TableCell>
              <Badge variant='outline'>{typeLabel(group.type)}</Badge>
            </TableCell>
            <TableCell>
              <Badge variant={group.enabled ? 'default' : 'secondary'}>
                {group.enabled ? t('enabled') : t('disabled')}
              </Badge>
            </TableCell>
            <TableCell>{group.ip_list.length}</TableCell>
            <TableCell>{group.referenced_by_rule_count}</TableCell>
            <TableCell className='max-w-[200px] truncate text-sm text-muted-foreground'>
              {group.last_sync_status
                ? `${group.last_sync_status}: ${group.last_sync_message}`
                : t('noSyncRecord')}
            </TableCell>
            <TableCell className='text-sm text-muted-foreground'>
              {group.updated_at ? formatDateTime(group.updated_at) : '—'}
            </TableCell>
            <TableCell className='text-right'>
              <div className='flex items-center justify-end gap-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-8'
                  title={t('actions.view')}
                  aria-label={t('actions.view')}
                  onClick={() => onView(group)}
                >
                  <Eye />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-8'
                  title={t('actions.edit')}
                  aria-label={t('actions.edit')}
                  onClick={() => onEdit(group)}
                >
                  <Pencil />
                </Button>
                {group.type === 'automatic' ? (
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='size-8'
                    title={t('actions.test')}
                    aria-label={t('actions.test')}
                    onClick={() => onTest(group)}
                  >
                    <Play />
                  </Button>
                ) : null}
                {group.type === 'subscription' || group.type === 'automatic' ? (
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='size-8'
                    title={
                      group.type === 'automatic'
                        ? t('actions.runNow')
                        : t('actions.syncNow')
                    }
                    aria-label={
                      group.type === 'automatic'
                        ? t('actions.runNow')
                        : t('actions.syncNow')
                    }
                    disabled={syncingId === group.id}
                    onClick={() => onSync(group)}
                  >
                    <Download />
                  </Button>
                ) : null}
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-8 text-destructive hover:text-destructive'
                  title={t('actions.delete')}
                  aria-label={t('actions.delete')}
                  onClick={() => onDelete(group)}
                >
                  <Trash2 />
                </Button>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
