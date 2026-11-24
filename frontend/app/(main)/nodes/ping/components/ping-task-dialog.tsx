'use client';

import { useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import type {
  NodeItem,
  PingTask,
  PingTaskPayload,
  PingTaskType,
} from '@/lib/services/openflare';

import { NodeMultiSelector } from '../../components/node-multi-selector';

const TARGET_PLACEHOLDER: Record<PingTaskType, string> = {
  icmp: '1.1.1.1',
  tcp: '1.1.1.1:443',
  http: 'https://www.cloudflare.com',
};

export function PingTaskDialog({
  open,
  task,
  nodes,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean;
  task: PingTask | null;
  nodes: NodeItem[];
  submitting: boolean;
  onClose: () => void;
  onSubmit: (payload: PingTaskPayload) => Promise<void>;
}) {
  const t = useTranslations('nodePing');
  const tc = useTranslations('common');

  const [name, setName] = useState('');
  const [type, setType] = useState<PingTaskType>('icmp');
  const [target, setTarget] = useState('');
  const [interval, setInterval] = useState('60');
  const [defaultOn, setDefaultOn] = useState(false);
  const [clients, setClients] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setError(null);
    if (task) {
      setName(task.name);
      setType(task.type);
      setTarget(task.target);
      setInterval(String(task.interval));
      setDefaultOn(task.default_on);
      setClients(task.clients ?? []);
    } else {
      setName('');
      setType('icmp');
      setTarget('');
      setInterval('60');
      setDefaultOn(false);
      setClients([]);
    }
  }, [open, task]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const trimmedName = name.trim();
    const trimmedTarget = target.trim();
    const parsedInterval = Number(interval);

    if (!trimmedName) {
      setError(t('dialog.errName'));
      return;
    }
    if (!trimmedTarget) {
      setError(t('dialog.errTarget'));
      return;
    }
    if (
      !Number.isInteger(parsedInterval) ||
      parsedInterval < 10 ||
      parsedInterval > 86400
    ) {
      setError(t('dialog.errInterval'));
      return;
    }

    await onSubmit({
      name: trimmedName,
      type,
      target: trimmedTarget,
      interval: parsedInterval,
      default_on: defaultOn,
      clients,
    });
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => (!next ? onClose() : undefined)}
    >
      <DialogContent className='max-w-xl'>
        <DialogHeader>
          <DialogTitle>
            {task ? t('dialog.editTitle') : t('dialog.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('dialog.description')}</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className='space-y-4'>
          <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
            <div className='space-y-1.5'>
              <Label htmlFor='ping-task-name'>{t('dialog.name')}</Label>
              <Input
                id='ping-task-name'
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t('dialog.namePlaceholder')}
                maxLength={255}
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='ping-task-type'>{t('dialog.type')}</Label>
              <Select
                value={type}
                onValueChange={(value) => setType(value as PingTaskType)}
              >
                <SelectTrigger id='ping-task-type'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='icmp'>ICMP</SelectItem>
                  <SelectItem value='tcp'>TCP</SelectItem>
                  <SelectItem value='http'>HTTP</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className='grid grid-cols-1 gap-4 sm:grid-cols-3'>
            <div className='space-y-1.5 sm:col-span-2'>
              <Label htmlFor='ping-task-target'>{t('dialog.target')}</Label>
              <Input
                id='ping-task-target'
                value={target}
                onChange={(e) => setTarget(e.target.value)}
                placeholder={TARGET_PLACEHOLDER[type]}
                maxLength={255}
              />
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='ping-task-interval'>{t('dialog.interval')}</Label>
              <Input
                id='ping-task-interval'
                type='number'
                min={10}
                max={86400}
                value={interval}
                onChange={(e) => setInterval(e.target.value)}
              />
            </div>
          </div>

          <div className='flex items-center justify-between gap-4 rounded-lg border border-dashed px-3 py-2.5'>
            <div className='space-y-0.5'>
              <Label htmlFor='ping-task-default-on' className='text-sm'>
                {t('dialog.defaultOn')}
              </Label>
              <p className='text-xs text-muted-foreground'>
                {t('dialog.defaultOnDesc')}
              </p>
            </div>
            <Switch
              id='ping-task-default-on'
              checked={defaultOn}
              onCheckedChange={setDefaultOn}
            />
          </div>

          <div className='space-y-1.5'>
            <Label>{t('dialog.nodes', { count: clients.length })}</Label>
            <NodeMultiSelector
              nodes={nodes}
              value={clients}
              onChange={setClients}
              maxHeightClass='max-h-52'
            />
          </div>

          {error ? <p className='text-xs text-destructive'>{error}</p> : null}

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={submitting}
              onClick={onClose}
            >
              {tc('cancel')}
            </Button>
            <Button type='submit' disabled={submitting}>
              {submitting ? (
                <Loader2 className='mr-1.5 size-4 animate-spin' />
              ) : null}
              {task ? t('dialog.save') : t('dialog.create')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
