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
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
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
  SmartDNSPolicy,
} from '@/lib/services/openflare';

const emptySmartDNSPolicies: SmartDNSPolicy[] = [];

export function GroupDialog({
  open,
  onOpenChange,
  group,
  nodes,
  smartdnsPolicies = emptySmartDNSPolicies,
  pending,
  onSubmit,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  group?: CloudflareGroup | null;
  nodes: NodeItem[];
  smartdnsPolicies?: SmartDNSPolicy[];
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
  const [targetMode, setTargetMode] = useState<'node' | 'custom' | 'smartdns'>(
    'node',
  );
  const [recordType, setRecordType] = useState<'CNAME' | 'A' | 'AAAA'>('A');
  const [recordContent, setRecordContent] = useState('');
  const [smartdnsPolicyID, setSmartdnsPolicyID] = useState('');
  const [smartdnsTargetHost, setSmartdnsTargetHost] = useState('');
  const [primaryNodeID, setPrimaryNodeID] = useState('');
  const [backupNodeID, setBackupNodeID] = useState('none');
  const [defaultProxied, setDefaultProxied] = useState(true);
  const [enabled, setEnabled] = useState(true);
  const [ttlMode, setTtlMode] = useState<'auto' | 'manual'>('auto');
  const [manualTTL, setManualTTL] = useState('300');
  const ttl = ttlMode === 'auto' ? 1 : Number(manualTTL);
  const invalidTTL =
    ttlMode === 'manual' &&
    (!manualTTL.trim() || !Number.isInteger(ttl) || ttl < 60 || ttl > 86400);

  useEffect(() => {
    if (!open) return;
    setName(group?.name ?? '');
    const mode =
      group?.target_mode === 'smartdns'
        ? 'smartdns'
        : group?.target_mode === 'custom'
          ? 'custom'
          : 'node';
    setTargetMode(mode);
    setRecordType(group?.record_type ?? 'A');
    setRecordContent(group?.record_content ?? '');
    const defaultPolicy =
      smartdnsPolicies.find((p) => p.is_default) ?? smartdnsPolicies[0];
    setSmartdnsPolicyID(
      group?.smartdns_policy_id
        ? String(group.smartdns_policy_id)
        : defaultPolicy
          ? String(defaultPolicy.id)
          : '',
    );
    setSmartdnsTargetHost(
      group?.smartdns_target_host ??
        (mode === 'smartdns' ? (group?.record_content ?? '') : ''),
    );
    setPrimaryNodeID(
      group?.target_mode === 'node' && group
        ? String(group.primary_node.id)
        : '',
    );
    setBackupNodeID(
      group?.target_mode === 'node' && group?.backup_node
        ? String(group.backup_node.id)
        : 'none',
    );
    setDefaultProxied(
      mode === 'smartdns' ? false : (group?.default_proxied ?? true),
    );
    setEnabled(group?.enabled ?? true);
    const groupTTL = group ? (group.ttl ?? 300) : 1;
    setTtlMode(groupTTL === 1 ? 'auto' : 'manual');
    setManualTTL(String(groupTTL === 1 ? 300 : groupTTL));
    // Intentionally omit smartdnsPolicies so lazy fetch never resets user-edited fields.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [group, open]);

  useEffect(() => {
    if (
      open &&
      targetMode === 'smartdns' &&
      !smartdnsPolicyID &&
      smartdnsPolicies.length > 0
    ) {
      const def =
        smartdnsPolicies.find((p) => p.is_default) ?? smartdnsPolicies[0];
      setSmartdnsPolicyID(String(def.id));
    }
  }, [open, smartdnsPolicies, smartdnsPolicyID, targetMode]);

  const effectiveDefaultProxied =
    targetMode === 'smartdns' ? false : defaultProxied;
  const submitDisabled =
    pending ||
    invalidTTL ||
    !name.trim() ||
    (targetMode === 'node'
      ? !primaryNodeID
      : targetMode === 'custom'
        ? !recordContent.trim()
        : !smartdnsPolicyID);

  return (
    <Dialog open={open} onOpenChange={(next) => !pending && onOpenChange(next)}>
      <DialogContent className='max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-lg'>
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
              className='grid w-full grid-cols-1 sm:grid-cols-3'
              onValueChange={(value) => {
                if (
                  value === 'node' ||
                  value === 'custom' ||
                  value === 'smartdns'
                ) {
                  setTargetMode(value);
                  if (value === 'smartdns') {
                    if (!smartdnsPolicyID && smartdnsPolicies.length > 0) {
                      const def =
                        smartdnsPolicies.find((p) => p.is_default) ??
                        smartdnsPolicies[0];
                      setSmartdnsPolicyID(String(def.id));
                    }
                  }
                }
              }}
            >
              <ToggleGroupItem value='node' className='w-full'>
                {t('bindNode')}
              </ToggleGroupItem>
              <ToggleGroupItem value='custom' className='w-full'>
                {t('customTarget')}
              </ToggleGroupItem>
              <ToggleGroupItem value='smartdns' className='w-full'>
                {t('smartdnsTarget')}
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
          ) : targetMode === 'custom' ? (
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
          ) : (
            <>
              <Field>
                <FieldLabel htmlFor='cf-smartdns-policy'>
                  {t('smartdnsPolicy')}
                </FieldLabel>
                <Select
                  value={smartdnsPolicyID}
                  onValueChange={setSmartdnsPolicyID}
                >
                  <SelectTrigger id='cf-smartdns-policy' className='w-full'>
                    <SelectValue placeholder={t('smartdnsPolicyPlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {smartdnsPolicies.map((policy) => (
                        <SelectItem key={policy.id} value={String(policy.id)}>
                          {policy.name} ({policy.strategy_type})
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                {smartdnsPolicies.length === 0 ? (
                  <FieldDescription>{t('smartdnsNoPolicies')}</FieldDescription>
                ) : null}
              </Field>
              <Field>
                <FieldLabel htmlFor='cf-smartdns-host'>
                  {t('smartdnsTargetHost')}
                </FieldLabel>
                <Input
                  id='cf-smartdns-host'
                  value={smartdnsTargetHost}
                  placeholder={t('smartdnsHostPlaceholder')}
                  onChange={(event) =>
                    setSmartdnsTargetHost(event.target.value)
                  }
                />
                <FieldDescription>{t('smartdnsHostHint')}</FieldDescription>
              </Field>
            </>
          )}
          <details className='rounded-lg border border-dashed px-4 py-3'>
            <summary className='cursor-pointer text-sm font-medium'>
              {t('recordSettings')}
            </summary>
            <FieldGroup className='mt-4'>
              <Field>
                <FieldLabel id='cf-ttl-mode-label'>{t('ttl')}</FieldLabel>
                <ToggleGroup
                  type='single'
                  variant='outline'
                  value={ttlMode}
                  aria-labelledby='cf-ttl-mode-label'
                  aria-describedby='cf-ttl-hint'
                  className='grid w-full grid-cols-2'
                  onValueChange={(value) => {
                    if (value === 'auto' || value === 'manual') {
                      setTtlMode(value);
                    }
                  }}
                >
                  <ToggleGroupItem value='auto' className='w-full'>
                    {t('ttlAuto')}
                  </ToggleGroupItem>
                  <ToggleGroupItem value='manual' className='w-full'>
                    {t('ttlManual')}
                  </ToggleGroupItem>
                </ToggleGroup>
                <FieldDescription id='cf-ttl-hint'>
                  {t('ttlHint')}
                </FieldDescription>
              </Field>
              {ttlMode === 'manual' && (
                <Field data-invalid={invalidTTL}>
                  <FieldLabel htmlFor='cf-ttl-seconds'>
                    {t('ttlSeconds')}
                  </FieldLabel>
                  <Input
                    id='cf-ttl-seconds'
                    type='number'
                    min={60}
                    max={86400}
                    step={1}
                    required
                    value={manualTTL}
                    aria-invalid={invalidTTL}
                    aria-describedby={
                      invalidTTL ? 'cf-ttl-hint cf-ttl-error' : 'cf-ttl-hint'
                    }
                    onChange={(event) => setManualTTL(event.target.value)}
                  />
                  {invalidTTL && (
                    <FieldError id='cf-ttl-error'>{t('ttlError')}</FieldError>
                  )}
                </Field>
              )}
              <Field>
                <div className='flex items-center justify-between gap-2'>
                  <FieldLabel htmlFor='cf-default-proxied'>
                    {t('defaultProxied')}
                  </FieldLabel>
                  <Switch
                    id='cf-default-proxied'
                    checked={effectiveDefaultProxied}
                    disabled={targetMode === 'smartdns'}
                    onCheckedChange={setDefaultProxied}
                  />
                </div>
                {targetMode === 'smartdns' ? (
                  <FieldDescription>
                    {t('smartdnsProxyDisabledHint')}
                  </FieldDescription>
                ) : null}
              </Field>
            </FieldGroup>
          </details>
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
          <Button
            variant='outline'
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            {tCommon('cancel')}
          </Button>
          <Button
            disabled={submitDisabled}
            onClick={() => {
              if (invalidTTL) return;
              onSubmit({
                name: name.trim(),
                target_mode: targetMode,
                record_type: targetMode === 'smartdns' ? 'CNAME' : recordType,
                record_content:
                  targetMode === 'smartdns'
                    ? smartdnsTargetHost.trim()
                    : recordContent.trim(),
                smartdns_policy_id:
                  targetMode === 'smartdns' && smartdnsPolicyID
                    ? Number(smartdnsPolicyID)
                    : null,
                smartdns_target_host:
                  targetMode === 'smartdns' ? smartdnsTargetHost.trim() : '',
                primary_node_id:
                  targetMode === 'node' ? Number(primaryNodeID) : 0,
                backup_node_id:
                  targetMode === 'node' && backupNodeID !== 'none'
                    ? Number(backupNodeID)
                    : null,
                default_proxied: effectiveDefaultProxied,
                ttl,
                enabled,
              });
            }}
          >
            {t('save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
