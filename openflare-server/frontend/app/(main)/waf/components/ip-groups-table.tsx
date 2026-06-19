'use client';

import {Download, Eye, MoreHorizontal, Pencil, Play, Trash2} from 'lucide-react';

import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from '@/components/ui/table';
import type {WAFIPGroup} from '@/lib/services/openflare';
import {formatDateTime} from '@/lib/utils';

import {ipGroupTypeLabels} from './helpers';

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
          <TableHead className="w-[80px] text-right">操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {groups.map((group) => (
          <TableRow key={group.id}>
            <TableCell className="font-medium">{group.name}</TableCell>
            <TableCell>
              <Badge variant="outline">{ipGroupTypeLabels[group.type]}</Badge>
            </TableCell>
            <TableCell>
              <Badge variant={group.enabled ? 'default' : 'secondary'}>
                {group.enabled ? '启用' : '停用'}
              </Badge>
            </TableCell>
            <TableCell>{group.ip_list.length}</TableCell>
            <TableCell>{group.referenced_by_rule_count}</TableCell>
            <TableCell className="text-sm text-muted-foreground max-w-[200px] truncate">
              {group.last_sync_status
                ? `${group.last_sync_status}: ${group.last_sync_message}`
                : '尚无同步记录'}
            </TableCell>
            <TableCell className="text-muted-foreground text-sm">
              {group.updated_at ? formatDateTime(group.updated_at) : '—'}
            </TableCell>
            <TableCell className="text-right">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon" className="size-8">
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => onView(group)}>
                    <Eye className="size-4 mr-2" />
                    查看
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => onEdit(group)}>
                    <Pencil className="size-4 mr-2" />
                    编辑
                  </DropdownMenuItem>
                  {group.type === 'automatic' ? (
                    <DropdownMenuItem onClick={() => onTest(group)}>
                      <Play className="size-4 mr-2" />
                      测试规则
                    </DropdownMenuItem>
                  ) : null}
                  {group.type === 'subscription' || group.type === 'automatic' ? (
                    <DropdownMenuItem
                      disabled={syncingId === group.id}
                      onClick={() => onSync(group)}
                    >
                      <Download className="size-4 mr-2" />
                      {syncingId === group.id
                        ? '同步中...'
                        : group.type === 'automatic'
                          ? '立即执行'
                          : '立即同步'}
                    </DropdownMenuItem>
                  ) : null}
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onClick={() => onDelete(group)}
                  >
                    <Trash2 className="size-4 mr-2" />
                    删除
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
