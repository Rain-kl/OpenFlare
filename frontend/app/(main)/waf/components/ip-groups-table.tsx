'use client';

import { Download, Eye, Pencil, Play, Trash2 } from 'lucide-react';

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
import type { WAFIPGroup } from '@/lib/services/openflare';
import { formatDateTime } from '@/lib/utils';

import { ipGroupTypeLabels } from './helpers';

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
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>名称</TableHead>
          <TableHead>类型</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>IP 数</TableHead>
          <TableHead>引用次数</TableHead>
          <TableHead>同步状态</TableHead>
          <TableHead>更新时间</TableHead>
          <TableHead className='w-[168px] text-right'>操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {groups.map((group) => (
          <TableRow key={group.id}>
            <TableCell className='font-medium'>{group.name}</TableCell>
            <TableCell>
              <Badge variant='outline'>{ipGroupTypeLabels[group.type]}</Badge>
            </TableCell>
            <TableCell>
              <Badge variant={group.enabled ? 'default' : 'secondary'}>
                {group.enabled ? '启用' : '停用'}
              </Badge>
            </TableCell>
            <TableCell>{group.ip_list.length}</TableCell>
            <TableCell>{group.referenced_by_rule_count}</TableCell>
            <TableCell className='max-w-[200px] truncate text-sm text-muted-foreground'>
              {group.last_sync_status
                ? `${group.last_sync_status}: ${group.last_sync_message}`
                : '尚无同步记录'}
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
                  title='查看'
                  aria-label='查看'
                  onClick={() => onView(group)}
                >
                  <Eye />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-8'
                  title='编辑'
                  aria-label='编辑'
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
                    title='测试规则'
                    aria-label='测试规则'
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
                      group.type === 'automatic' ? '立即执行' : '立即同步'
                    }
                    aria-label={
                      group.type === 'automatic' ? '立即执行' : '立即同步'
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
                  title='删除'
                  aria-label='删除'
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
