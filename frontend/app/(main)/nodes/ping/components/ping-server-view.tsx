'use client';

import { useMemo, useState } from 'react';
import { SlidersHorizontal } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type {
  NodeItem,
  PingTask,
  PingTaskBatchUpdateItem,
} from '@/lib/services/openflare';

import { NodeStatusBadge } from '../../components/node-status-badge';
import {
  getNodeStatusLabel,
  getNodeStatusTone,
} from '../../components/node-utils';

export function PingServerView({
  tasks,
  nodes,
  onBatchUpdate,
}: {
  tasks: PingTask[];
  nodes: NodeItem[];
  onBatchUpdate: (items: PingTaskBatchUpdateItem[]) => Promise<void>;
}) {
  const t = useTranslations('nodePing');
  const tn = useTranslations('nodes');
  const tc = useTranslations('common');

  const [editingNode, setEditingNode] = useState<NodeItem | null>(null);
  const [selectedTaskIds, setSelectedTaskIds] = useState<number[]>([]);
  const [taskSearch, setTaskSearch] = useState('');
  const [saving, setSaving] = useState(false);

  const sortedNodes = useMemo(
    () =>
      [...nodes].sort((a, b) => {
        if (a.status === 'online' && b.status !== 'online') return -1;
        if (a.status !== 'online' && b.status === 'online') return 1;
        return a.name.localeCompare(b.name);
      }),
    [nodes],
  );

  const filteredTasks = useMemo(() => {
    const q = taskSearch.trim().toLowerCase();
    if (!q) return tasks;
    return tasks.filter(
      (item) =>
        item.name.toLowerCase().includes(q) ||
        item.target.toLowerCase().includes(q),
    );
  }, [taskSearch, tasks]);

  const openEditor = (node: NodeItem) => {
    setEditingNode(node);
    setTaskSearch('');
    setSelectedTaskIds(
      tasks
        .filter((item) => (item.clients ?? []).includes(node.node_id))
        .map((item) => item.id),
    );
  };

  const toggleTaskId = (taskId: number) => {
    setSelectedTaskIds((prev) =>
      prev.includes(taskId)
        ? prev.filter((id) => id !== taskId)
        : [...prev, taskId],
    );
  };

  const handleSave = async () => {
    if (!editingNode) return;
    const changed: PingTaskBatchUpdateItem[] = [];
    for (const task of tasks) {
      const hadNode = (task.clients ?? []).includes(editingNode.node_id);
      const hasNode = selectedTaskIds.includes(task.id);
      if (hadNode === hasNode) continue;

      const nextClients = new Set(task.clients ?? []);
      if (hasNode) nextClients.add(editingNode.node_id);
      else nextClients.delete(editingNode.node_id);

      changed.push({
        id: task.id,
        name: task.name,
        type: task.type,
        target: task.target,
        interval: task.interval,
        default_on: task.default_on,
        clients: Array.from(nextClients),
      });
    }

    if (changed.length === 0) {
      setEditingNode(null);
      return;
    }

    setSaving(true);
    try {
      await onBatchUpdate(changed);
      setEditingNode(null);
    } finally {
      setSaving(false);
    }
  };

  if (sortedNodes.length === 0) {
    return (
      <div className='rounded-xl border border-dashed py-12 text-center text-sm text-muted-foreground'>
        {t('serverView.empty')}
      </div>
    );
  }

  return (
    <>
      <div className='overflow-x-auto rounded-xl border border-dashed bg-card'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-64'>{t('serverView.colNode')}</TableHead>
              <TableHead>{t('serverView.colTasks')}</TableHead>
              <TableHead className='w-28 text-right'>
                {t('taskView.colActions')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sortedNodes.map((node) => {
              const ownedTasks = tasks.filter((item) =>
                (item.clients ?? []).includes(node.node_id),
              );
              return (
                <TableRow key={node.node_id}>
                  <TableCell>
                    <div className='flex items-center gap-2'>
                      <div className='min-w-0'>
                        <p className='truncate font-medium'>{node.name}</p>
                        <p className='truncate font-mono text-[11px] text-muted-foreground'>
                          {node.node_id}
                        </p>
                      </div>
                      <NodeStatusBadge
                        label={getNodeStatusLabel(node.status, tn)}
                        tone={getNodeStatusTone(node.status)}
                      />
                    </div>
                  </TableCell>
                  <TableCell>
                    {ownedTasks.length > 0 ? (
                      <div className='flex flex-wrap gap-1.5'>
                        {ownedTasks.map((task) => (
                          <Badge
                            key={task.id}
                            variant='secondary'
                            className='text-xs font-normal'
                          >
                            {task.name}
                          </Badge>
                        ))}
                      </div>
                    ) : (
                      <span className='text-xs text-muted-foreground'>
                        {t('serverView.noTasks')}
                      </span>
                    )}
                  </TableCell>
                  <TableCell className='text-right'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      className='h-7 text-xs'
                      onClick={() => openEditor(node)}
                    >
                      <SlidersHorizontal className='mr-1 size-3' />
                      {t('serverView.configure')}
                    </Button>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>

      <Dialog
        open={Boolean(editingNode)}
        onOpenChange={(open) => (!open ? setEditingNode(null) : undefined)}
      >
        <DialogContent className='max-w-lg'>
          <DialogHeader>
            <DialogTitle>
              {t('serverView.dialogTitle', {
                name: editingNode?.name ?? '',
              })}
            </DialogTitle>
            <DialogDescription>{t('serverView.dialogDesc')}</DialogDescription>
          </DialogHeader>

          <div className='space-y-3'>
            <Input
              value={taskSearch}
              onChange={(e) => setTaskSearch(e.target.value)}
              placeholder={t('serverView.searchPlaceholder')}
              className='h-8 text-xs'
            />
            <div className='max-h-64 overflow-y-auto rounded-lg border border-dashed divide-y divide-dashed'>
              {filteredTasks.length === 0 ? (
                <div className='py-6 text-center text-xs text-muted-foreground'>
                  {t('taskView.empty')}
                </div>
              ) : (
                filteredTasks.map((task) => (
                  <label
                    key={task.id}
                    className='flex cursor-pointer items-center justify-between gap-3 px-3 py-2 text-sm hover:bg-muted/30 transition-colors'
                  >
                    <div className='flex min-w-0 items-center gap-2.5'>
                      <Checkbox
                        checked={selectedTaskIds.includes(task.id)}
                        onCheckedChange={() => toggleTaskId(task.id)}
                      />
                      <div className='min-w-0'>
                        <p className='truncate font-medium'>{task.name}</p>
                        <p className='truncate font-mono text-[11px] text-muted-foreground'>
                          {task.target}
                        </p>
                      </div>
                    </div>
                    <Badge
                      variant='outline'
                      className='shrink-0 font-mono text-[11px] uppercase'
                    >
                      {task.type}/{task.interval}s
                    </Badge>
                  </label>
                ))
              )}
            </div>
          </div>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={saving}
              onClick={() => setEditingNode(null)}
            >
              {tc('cancel')}
            </Button>
            <Button type='button' disabled={saving} onClick={handleSave}>
              {t('dialog.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
