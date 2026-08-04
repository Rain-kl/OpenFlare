'use client';

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
  const edgeNodes = useMemo(
    () => nodes.filter((node) => node.node_type === 'edge_node'),
    [nodes],
  );
  const [name, setName] = useState('');
  const [primaryNodeID, setPrimaryNodeID] = useState('');
  const [backupNodeID, setBackupNodeID] = useState('none');
  const [defaultProxied, setDefaultProxied] = useState(true);
  const [enabled, setEnabled] = useState(true);

  useEffect(() => {
    if (!open) return;
    setName(group?.name ?? '');
    setPrimaryNodeID(group ? String(group.primary_node.id) : '');
    setBackupNodeID(group?.backup_node ? String(group.backup_node.id) : 'none');
    setDefaultProxied(group?.default_proxied ?? true);
    setEnabled(group?.enabled ?? true);
  }, [group, open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{group ? '编辑指向分组' : '新增指向分组'}</DialogTitle>
          <DialogDescription>
            一期使用主节点作为生效节点；备用节点仅保存，不会自动切换。
          </DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='cf-group-name'>分组名称</FieldLabel>
            <Input
              id='cf-group-name'
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor='cf-primary-node'>主节点</FieldLabel>
            <Select value={primaryNodeID} onValueChange={setPrimaryNodeID}>
              <SelectTrigger id='cf-primary-node' className='w-full'>
                <SelectValue placeholder='选择带 IPv4 的边缘节点' />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {edgeNodes.map((node) => (
                    <SelectItem
                      key={node.id}
                      value={String(node.id)}
                      disabled={!node.ip}
                    >
                      {node.name} · {node.ip || '未配置 IP'}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel htmlFor='cf-backup-node'>备用节点</FieldLabel>
            <Select value={backupNodeID} onValueChange={setBackupNodeID}>
              <SelectTrigger id='cf-backup-node' className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value='none'>不设置</SelectItem>
                  {edgeNodes
                    .filter((node) => String(node.id) !== primaryNodeID)
                    .map((node) => (
                      <SelectItem key={node.id} value={String(node.id)}>
                        {node.name} · {node.ip || '未配置 IP'}
                      </SelectItem>
                    ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field orientation='horizontal'>
            <FieldLabel htmlFor='cf-default-proxied'>
              新成员默认开启橙云
            </FieldLabel>
            <Switch
              id='cf-default-proxied'
              checked={defaultProxied}
              onCheckedChange={setDefaultProxied}
            />
          </Field>
          <Field orientation='horizontal'>
            <FieldLabel htmlFor='cf-enabled'>启用同步</FieldLabel>
            <Switch
              id='cf-enabled'
              checked={enabled}
              onCheckedChange={setEnabled}
            />
          </Field>
        </FieldGroup>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            disabled={pending || !name.trim() || !primaryNodeID}
            onClick={() =>
              onSubmit({
                name: name.trim(),
                primary_node_id: Number(primaryNodeID),
                backup_node_id:
                  backupNodeID === 'none' ? null : Number(backupNodeID),
                default_proxied: defaultProxied,
                enabled,
              })
            }
          >
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
