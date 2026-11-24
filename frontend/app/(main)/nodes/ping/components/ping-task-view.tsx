'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  DndContext,
  KeyboardSensor,
  MouseSensor,
  TouchSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  arrayMove,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical, Pencil, Server, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
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
  PingTaskPayload,
} from '@/lib/services/openflare';

import { NodeMultiSelector } from '../../components/node-multi-selector';

function SortableTaskRow({
  task,
  nodeMap,
  onEdit,
  onDelete,
  onQuickNodes,
}: {
  task: PingTask;
  nodeMap: Map<string, NodeItem>;
  onEdit: (task: PingTask) => void;
  onDelete: (task: PingTask) => void;
  onQuickNodes: (task: PingTask) => void;
}) {
  const t = useTranslations('nodePing');
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: task.id });

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.6 : 1,
  };

  const activeClients = (task.clients ?? []).filter((id) => nodeMap.has(id));
  const previewNames = activeClients
    .slice(0, 3)
    .map((id) => nodeMap.get(id)?.name ?? id)
    .join(', ');

  return (
    <TableRow ref={setNodeRef} style={style}>
      <TableCell className='w-10'>
        <button
          type='button'
          className='flex size-7 cursor-grab items-center justify-center rounded text-muted-foreground hover:text-foreground active:cursor-grabbing'
          aria-label={t('taskView.dragToSort')}
          {...attributes}
          {...listeners}
        >
          <GripVertical className='size-4' />
        </button>
      </TableCell>
      <TableCell className='font-medium'>
        <div className='flex flex-wrap items-center gap-2'>
          <span>{task.name}</span>
          {task.default_on ? (
            <Badge variant='secondary' className='text-[11px]'>
              {t('taskView.defaultOnBadge')}
            </Badge>
          ) : null}
        </div>
      </TableCell>
      <TableCell>
        <div className='flex items-center gap-2'>
          <span className='max-w-[240px] truncate text-xs text-muted-foreground'>
            {activeClients.length > 0
              ? `${previewNames}${activeClients.length > 3 ? ` +${activeClients.length - 3}` : ''}`
              : t('taskView.noNodes')}
          </span>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='h-6.5 px-2 text-xs'
            onClick={() => onQuickNodes(task)}
          >
            <Server className='mr-1 size-3' />
            {activeClients.length}
          </Button>
        </div>
      </TableCell>
      <TableCell className='font-mono text-xs'>{task.target}</TableCell>
      <TableCell>
        <Badge variant='outline' className='font-mono text-[11px] uppercase'>
          {task.type}
        </Badge>
      </TableCell>
      <TableCell className='tabular-nums text-xs'>{task.interval}s</TableCell>
      <TableCell className='text-right'>
        <div className='inline-flex items-center gap-1'>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='size-7'
            aria-label={t('taskView.edit')}
            onClick={() => onEdit(task)}
          >
            <Pencil className='size-3.5' />
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='size-7 text-destructive hover:text-destructive'
            aria-label={t('taskView.delete')}
            onClick={() => onDelete(task)}
          >
            <Trash2 className='size-3.5' />
          </Button>
        </div>
      </TableCell>
    </TableRow>
  );
}

export function PingTaskView({
  tasks,
  nodes,
  onEdit,
  onDelete,
  onReorder,
  onUpdateTask,
}: {
  tasks: PingTask[];
  nodes: NodeItem[];
  onEdit: (task: PingTask) => void;
  onDelete: (task: PingTask) => void;
  onReorder: (weights: Record<string, number>) => Promise<void>;
  onUpdateTask: (id: number, payload: PingTaskPayload) => Promise<void>;
}) {
  const t = useTranslations('nodePing');
  const tc = useTranslations('common');
  const [localTasks, setLocalTasks] = useState<PingTask[]>(tasks);
  const [quickNodesTask, setQuickNodesTask] = useState<PingTask | null>(null);
  const [quickClients, setQuickClients] = useState<string[]>([]);
  const [savingNodes, setSavingNodes] = useState(false);

  useEffect(() => {
    setLocalTasks(tasks);
  }, [tasks]);

  const nodeMap = useMemo(() => {
    const map = new Map<string, NodeItem>();
    for (const node of nodes) {
      map.set(node.node_id, node);
    }
    return map;
  }, [nodes]);

  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 8 } }),
    useSensor(TouchSensor, {
      activationConstraint: { delay: 180, tolerance: 5 },
    }),
    useSensor(KeyboardSensor),
  );

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = localTasks.findIndex((item) => item.id === active.id);
    const newIndex = localTasks.findIndex((item) => item.id === over.id);
    if (oldIndex < 0 || newIndex < 0) return;

    const previous = localTasks;
    const reordered = arrayMove(localTasks, oldIndex, newIndex);
    setLocalTasks(reordered);

    const weights: Record<string, number> = {};
    reordered.forEach((item, index) => {
      weights[String(item.id)] = index;
    });

    try {
      await onReorder(weights);
    } catch {
      setLocalTasks(previous);
    }
  };

  const openQuickNodes = (task: PingTask) => {
    setQuickNodesTask(task);
    setQuickClients(task.clients ?? []);
  };

  const saveQuickNodes = async () => {
    if (!quickNodesTask) return;
    setSavingNodes(true);
    try {
      await onUpdateTask(quickNodesTask.id, {
        name: quickNodesTask.name,
        type: quickNodesTask.type,
        target: quickNodesTask.target,
        interval: quickNodesTask.interval,
        default_on: quickNodesTask.default_on,
        clients: quickClients,
      });
      setQuickNodesTask(null);
    } finally {
      setSavingNodes(false);
    }
  };

  if (localTasks.length === 0) {
    return (
      <div className='rounded-xl border border-dashed py-12 text-center text-sm text-muted-foreground'>
        {t('taskView.empty')}
      </div>
    );
  }

  return (
    <>
      <div className='overflow-x-auto rounded-xl border border-dashed bg-card'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-10' />
              <TableHead>{t('taskView.colName')}</TableHead>
              <TableHead>{t('taskView.colNodes')}</TableHead>
              <TableHead>{t('taskView.colTarget')}</TableHead>
              <TableHead>{t('taskView.colType')}</TableHead>
              <TableHead>{t('taskView.colInterval')}</TableHead>
              <TableHead className='text-right'>
                {t('taskView.colActions')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            onDragEnd={handleDragEnd}
          >
            <SortableContext
              items={localTasks.map((item) => item.id)}
              strategy={verticalListSortingStrategy}
            >
              <TableBody>
                {localTasks.map((task) => (
                  <SortableTaskRow
                    key={task.id}
                    task={task}
                    nodeMap={nodeMap}
                    onEdit={onEdit}
                    onDelete={onDelete}
                    onQuickNodes={openQuickNodes}
                  />
                ))}
              </TableBody>
            </SortableContext>
          </DndContext>
        </Table>
      </div>

      <Dialog
        open={Boolean(quickNodesTask)}
        onOpenChange={(open) => (!open ? setQuickNodesTask(null) : undefined)}
      >
        <DialogContent className='max-w-lg'>
          <DialogHeader>
            <DialogTitle>
              {t('taskView.bindNodesTitle', {
                name: quickNodesTask?.name ?? '',
              })}
            </DialogTitle>
            <DialogDescription>{t('taskView.bindNodesDesc')}</DialogDescription>
          </DialogHeader>
          <NodeMultiSelector
            nodes={nodes}
            value={quickClients}
            onChange={setQuickClients}
          />
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={savingNodes}
              onClick={() => setQuickNodesTask(null)}
            >
              {tc('cancel')}
            </Button>
            <Button
              type='button'
              disabled={savingNodes}
              onClick={saveQuickNodes}
            >
              {t('dialog.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
