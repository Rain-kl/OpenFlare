'use client';

import { Globe2, Pencil, ShieldCheck, Trash2 } from 'lucide-react';
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
import type { WAFRule } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

interface RuleGroupsTableProps {
  groups: WAFRule[];
  onEdit: (group: WAFRule) => void;
  onDelete: (group: WAFRule) => void;
}

export function RuleGroupsTable({
  groups,
  onEdit,
  onDelete,
}: RuleGroupsTableProps) {
  const t = useTranslations('waf');
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('columns.name')}</TableHead>
          <TableHead>{t('columns.type')}</TableHead>
          <TableHead>{t('columns.status')}</TableHead>
          <TableHead>{t('columns.nodeCount')}</TableHead>
          <TableHead>{t('columns.scope')}</TableHead>
          <TableHead>{t('columns.updatedAt')}</TableHead>
          <TableHead className='w-[88px] text-right'>
            {t('columns.actions')}
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {groups.map((group) => (
          <TableRow key={group.id}>
            <TableCell>
              <div className='flex items-center gap-2'>
                {group.is_global ? (
                  <Globe2 className='size-4 shrink-0 text-primary' />
                ) : (
                  <ShieldCheck className='size-4 shrink-0 text-muted-foreground' />
                )}
                <span className='font-medium'>{group.name}</span>
              </div>
            </TableCell>
            <TableCell>
              <Badge variant='outline'>
                {group.is_global ? t('global') : t('custom')}
              </Badge>
            </TableCell>
            <TableCell>
              <Badge variant={group.enabled ? 'default' : 'secondary'}>
                {group.enabled ? t('enable') : t('disable')}
              </Badge>
            </TableCell>
            <TableCell>{group.graph.nodes.length}</TableCell>
            <TableCell>
              {group.is_global
                ? t('allSites')
                : t('siteCount', { count: group.applied_site_count })}
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
                  title={t('actions.compose')}
                  aria-label={t('actions.compose')}
                  onClick={() => onEdit(group)}
                >
                  <Pencil />
                </Button>
                {!group.is_global ? (
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
                ) : null}
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
