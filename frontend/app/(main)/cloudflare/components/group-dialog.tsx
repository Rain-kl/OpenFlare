'use client';

import { useTranslations } from 'next-intl';
import { useEffect, useMemo, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import type {
  CloudflareGroup,
  CloudflareGroupPayload,
  NodeItem,
} from '@/lib/services/openflare';

export function GroupDialog({
  open,
  onOpenChange,
  group,
  nodes,
  pending,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  group?: CloudflareGroup | null;
  nodes: NodeItem[];
  pending: boolean;
  onSubmit: (payload: CloudflareGroupPayload) => void;
}) {
  const t = useTranslations('cloudflare.groupDialog');
  const tCommon = useTranslations('common');
  const edgeNodes = useMemo(
    () => nodes.filter((node) => node.node_type === 'edge_node'),
    [nodes],
  );
  const recordContentPlaceholders = {
    CNAME: t('recordContentPlaceholder.CNAME'),
    A: t('recordContentPlaceholder.A'),
    AAAA: t('recordContentPlaceholder.AAAA'),
  };
  const [name, setName] = useState('');
  const [targetMode, setTargetMode] = useState<'node' | 'custom'>('node');
  const [recordType, setRecordType] = useState<'CNAME' | 'A' | 'AAAA'>('A');
  const [recordContent, setRecordContent] = useState('');
  const [primaryNodeID, setPrimaryNodeID] = useState('');
  const [backupNodeID, setBackupNodeID] = useState('none');
  const [defaultProxied, setDefaultProxied] = useState(true);
  const [enabled, setEnabled] = useState(true);

  useEffect(() => {
    if (!open) return;
    setName(group?.name ?? '');
    setTargetMode(group?.target_mode === 'custom' ? 'custom' : 'node');
    setRecordType(group?.record_type ?? 'A');
    setRecordContent(group?.record_content ?? '');
    setPrimaryNodeID(
      group?.target_mode !== 'custom' && group
        ? String(group.primary_node.id)
        : '',
    );
    setBackupNodeID(
      group?.target_mode !== 'custom' && group?.backup_node
        ? String(group.backup_node.id)
        : 'none',
    );
    setDefaultProxied(group?.default_proxied ?? true);
    setEnabled(group?.enabled ?? true);
  }, [group, open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{group ? t('editTitle') : t('createTitle')}</DialogTitle>
          <DialogDescription>{t('description')}</DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='cf-group-name'>{t('name')}</FieldLabel>
            <Input
              id='cf-group-name'
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </Field>
          <Field>
            <FieldLabel id='cf-target-mode-label'>{t('targetMode')}</FieldLabel>
            <ToggleGroup
              type='single'
              variant='outline'
              value={targetMode}
              aria-labelledby='cf-target-mode-label'
              className='grid w-full grid-cols-2'
              onValueChange={(value) => {
                if (value === 'node' || value === 'custom') {
                  setTargetMode(value);
                }
              }}
            >
              <ToggleGroupItem value='node' className='w-full'>
                {t('bindNode')}
              </ToggleGroupItem>
              <ToggleGroupItem value='custom' className='w-full'>
                {t('customTarget')}
              </ToggleGroupItem>
            </ToggleGroup>
          </Field>
          {targetMode === 'node' ? (
            <>
              <Field>
                <FieldLabel htmlFor='cf-primary-node'>
                  {t('primary')}
                </FieldLabel>
                <Select value={primaryNodeID} onValueChange={setPrimaryNodeID}>
                  <SelectTrigger id='cf-primary-node' className='w-full'>
                    <SelectValue placeholder={t('primaryPlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {edgeNodes.map((node) => (
                        <SelectItem
                          key={node.id}
                          value={String(node.id)}
                          disabled={!node.ip}
                        >
                          {node.name} · {node.ip || t('noIp')}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor='cf-backup-node'>{t('backup')}</FieldLabel>
                <Select value={backupNodeID} onValueChange={setBackupNodeID}>
                  <SelectTrigger id='cf-backup-node' className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value='none'>{t('none')}</SelectItem>
                      {edgeNodes
                        .filter((node) => String(node.id) !== primaryNodeID)
                        .map((node) => (
                          <SelectItem key={node.id} value={String(node.id)}>
                            {node.name} · {node.ip || t('noIp')}
                          </SelectItem>
                        ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
            </>
          ) : (
            <>
              <Field>
                <FieldLabel htmlFor='cf-record-type'>
                  {t('recordType')}
                </FieldLabel>
                <Select
                  value={recordType}
                  onValueChange={(value: 'CNAME' | 'A' | 'AAAA') =>
                    setRecordType(value)
                  }
                >
                  <SelectTrigger id='cf-record-type' className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value='CNAME'>CNAME</SelectItem>
                      <SelectItem value='A'>A</SelectItem>
                      <SelectItem value='AAAA'>AAAA</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor='cf-record-content'>
                  {t('recordContent')}
                </FieldLabel>
                <Input
                  id='cf-record-content'
                  value={recordContent}
                  placeholder={recordContentPlaceholders[recordType]}
                  onChange={(event) => setRecordContent(event.target.value)}
                />
                <p className='text-sm text-muted-foreground'>
                  {t('recordContentHint')}
                </p>
              </Field>
            </>
          )}
          <Field orientation='horizontal'>
            <FieldLabel htmlFor='cf-default-proxied'>
              {t('defaultProxied')}
            </FieldLabel>
            <Switch
              id='cf-default-proxied'
              checked={defaultProxied}
              onCheckedChange={setDefaultProxied}
            />
          </Field>
          <Field orientation='horizontal'>
            <FieldLabel htmlFor='cf-enabled'>{t('enableSync')}</FieldLabel>
            <Switch
              id='cf-enabled'
              checked={enabled}
              onCheckedChange={setEnabled}
            />
          </Field>
        </FieldGroup>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {tCommon('cancel')}
          </Button>
          <Button
            disabled={
              pending ||
              !name.trim() ||
              (targetMode === 'node' ? !primaryNodeID : !recordContent.trim())
            }
            onClick={() =>
              onSubmit({
                name: name.trim(),
                target_mode: targetMode,
                record_type: recordType,
                record_content: recordContent.trim(),
                primary_node_id:
                  targetMode === 'node' ? Number(primaryNodeID) : 0,
                backup_node_id:
                  targetMode === 'node' && backupNodeID !== 'none'
                    ? Number(backupNodeID)
                    : null,
                default_proxied: defaultProxied,
                enabled,
              })
            }
          >
            {t('save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
