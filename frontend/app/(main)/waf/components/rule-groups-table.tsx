'use client';

import { Globe2, Pencil, ShieldCheck, Trash2 } from 'lucide-react';

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
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>名称</TableHead>
          <TableHead>类型</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>节点数</TableHead>
          <TableHead>应用范围</TableHead>
          <TableHead>更新时间</TableHead>
          <TableHead className='w-[88px] text-right'>操作</TableHead>
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
                {group.is_global ? '全局' : '自定义'}
              </Badge>
            </TableCell>
            <TableCell>
              <Badge variant={group.enabled ? 'default' : 'secondary'}>
                {group.enabled ? '启用' : '停用'}
              </Badge>
            </TableCell>
            <TableCell>{group.graph.nodes.length}</TableCell>
            <TableCell>
              {group.is_global
                ? '全部网站'
                : `${group.applied_site_count} 个网站`}
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
                  title='编排'
                  aria-label='编排'
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
                    title='删除'
                    aria-label='删除'
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
