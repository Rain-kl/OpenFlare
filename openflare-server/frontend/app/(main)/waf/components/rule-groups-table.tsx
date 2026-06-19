'use client';

import {Globe2, MoreHorizontal, Pencil, ShieldCheck, Trash2, Users} from 'lucide-react';

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
import type {WAFRuleGroup} from '@/lib/services/openflare';
import {formatDateTime} from '@/lib/utils';

import {countRuleEntries} from './helpers';

interface RuleGroupsTableProps {
  groups: WAFRuleGroup[];
  onEdit: (group: WAFRuleGroup) => void;
  onDelete: (group: WAFRuleGroup) => void;
  onBindSites: (group: WAFRuleGroup) => void;
}

export function RuleGroupsTable({
  groups,
  onEdit,
  onDelete,
  onBindSites,
}: RuleGroupsTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>名称</TableHead>
          <TableHead>类型</TableHead>
          <TableHead>状态</TableHead>
          <TableHead>规则数</TableHead>
          <TableHead>应用范围</TableHead>
          <TableHead>更新时间</TableHead>
          <TableHead className="w-[80px] text-right">操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {groups.map((group) => (
          <TableRow key={group.id}>
            <TableCell>
              <div className="flex items-center gap-2">
                {group.is_global ? (
                  <Globe2 className="size-4 text-primary shrink-0" />
                ) : (
                  <ShieldCheck className="size-4 text-muted-foreground shrink-0" />
                )}
                <span className="font-medium">{group.name}</span>
              </div>
            </TableCell>
            <TableCell>
              <Badge variant="outline">{group.is_global ? '全局' : '自定义'}</Badge>
            </TableCell>
            <TableCell>
              <Badge variant={group.enabled ? 'default' : 'secondary'}>
                {group.enabled ? '启用' : '停用'}
              </Badge>
            </TableCell>
            <TableCell>{countRuleEntries(group)}</TableCell>
            <TableCell>
              {group.is_global
                ? '全部网站'
                : `${group.applied_site_count} 个网站`}
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
                  <DropdownMenuItem onClick={() => onEdit(group)}>
                    <Pencil className="size-4 mr-2" />
                    编辑
                  </DropdownMenuItem>
                  {!group.is_global ? (
                    <DropdownMenuItem onClick={() => onBindSites(group)}>
                      <Users className="size-4 mr-2" />
                      绑定网站
                    </DropdownMenuItem>
                  ) : null}
                  {!group.is_global ? (
                    <>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        className="text-destructive focus:text-destructive"
                        onClick={() => onDelete(group)}
                      >
                        <Trash2 className="size-4 mr-2" />
                        删除
                      </DropdownMenuItem>
                    </>
                  ) : null}
                </DropdownMenuContent>
              </DropdownMenu>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
