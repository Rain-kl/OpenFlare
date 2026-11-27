'use client';

import { useEffect, useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

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
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import type {
  NodeItem,
  NodeMutationPayload,
  NodeTagBindingInput,
} from '@/lib/services/openflare';
import { NodeTagService, nodeTagQueryKey } from '@/lib/services/openflare';

function buildNodeSchema(t: (key: string) => string) {
  return z
    .object({
      node_type: z.literal('edge_node'),
      name: z.string().trim().min(1, t('editor.errName')).max(255),
      ip: z.string(),
      ip_manual_override: z.boolean(),
      auto_update_enabled: z.boolean(),
      network_group: z.string().max(64),
      capacity_bandwidth_mbps: z.string(),
      capacity_connections: z.string(),
      capacity_weight: z.string(),
      geo_manual_override: z.boolean(),
      geo_name: z.string(),
      geo_latitude: z.string(),
      geo_longitude: z.string(),
    })
    .superRefine((value, context) => {
      if (value.ip_manual_override && !value.ip.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['ip'],
          message: t('editor.errLockIp'),
        });
      }
    });
}

type NodeFormValues = z.infer<ReturnType<typeof buildNodeSchema>>;

const defaultForm: NodeFormValues = {
  node_type: 'edge_node',
  name: '',
  ip: '',
  ip_manual_override: false,
  auto_update_enabled: false,
  network_group: '',
  capacity_bandwidth_mbps: '0',
  capacity_connections: '0',
  capacity_weight: '100',
  geo_manual_override: false,
  geo_name: '',
  geo_latitude: '',
  geo_longitude: '',
};

function buildFormValues(node?: NodeItem | null): NodeFormValues {
  if (!node) return defaultForm;

  return {
    node_type: 'edge_node',
    name: node.name,
    ip: node.ip,
    ip_manual_override: node.ip_manual_override,
    auto_update_enabled: node.auto_update_enabled,
    network_group: node.network_group ?? '',
    capacity_bandwidth_mbps: String(node.capacity_bandwidth_mbps ?? 0),
    capacity_connections: String(node.capacity_connections ?? 0),
    capacity_weight: String(node.capacity_weight ?? 100),
    geo_manual_override: node.geo_manual_override,
    geo_name: node.geo_name,
    geo_latitude:
      node.geo_latitude === undefined || node.geo_latitude === null
        ? ''
        : String(node.geo_latitude),
    geo_longitude:
      node.geo_longitude === undefined || node.geo_longitude === null
        ? ''
        : String(node.geo_longitude),
  };
}

function parseNonNegInt(raw: string, fallback: number): number {
  const n = Number(raw);
  return Number.isFinite(n) && n >= 0 ? Math.trunc(n) : fallback;
}

function toPayload(
  form: NodeFormValues,
  tagState: Record<string, string | undefined>,
): NodeMutationPayload {
  const tags: NodeTagBindingInput[] = Object.entries(tagState)
    .filter(([, override]) => override !== undefined)
    .map(([slug, override]) => {
      const trimmed = (override ?? '').trim();
      if (!trimmed) return { slug };
      const parsed = Number(trimmed);
      return Number.isFinite(parsed) && parsed > 0
        ? { slug, weight_override: parsed }
        : { slug };
    });

  return {
    node_type: form.node_type,
    name: form.name.trim(),
    ip: form.ip.trim(),
    ip_manual_override: form.ip_manual_override,
    auto_update_enabled: form.auto_update_enabled,
    network_group: form.network_group.trim().toLowerCase(),
    capacity_bandwidth_mbps: parseNonNegInt(form.capacity_bandwidth_mbps, 0),
    capacity_connections: parseNonNegInt(form.capacity_connections, 0),
    capacity_weight: parseNonNegInt(form.capacity_weight, 100),
    tags,
    geo_manual_override: form.geo_manual_override,
    geo_name: form.geo_manual_override ? form.geo_name.trim() : '',
    geo_latitude:
      form.geo_manual_override && form.geo_latitude
        ? Number(form.geo_latitude)
        : null,
    geo_longitude:
      form.geo_manual_override && form.geo_longitude
        ? Number(form.geo_longitude)
        : null,
  };
}

export function NodeEditorDialog({
  open,
  node,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean;
  node: NodeItem | null;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (payload: NodeMutationPayload) => Promise<void>;
}) {
  const t = useTranslations('nodes');
  const tc = useTranslations('common');
  const nodeSchema = buildNodeSchema(t);
  const form = useForm<NodeFormValues>({
    resolver: zodResolver(nodeSchema),
    defaultValues: buildFormValues(node),
  });
  const [tagState, setTagState] = useState<Record<string, string | undefined>>(
    {},
  );

  const tagsQuery = useQuery({
    queryKey: nodeTagQueryKey,
    queryFn: () => NodeTagService.list(),
    enabled: open,
  });

  useEffect(() => {
    if (!open) return;
    form.reset(buildFormValues(node));
    const initialTags: Record<string, string | undefined> = {};
    for (const item of node?.tags ?? []) {
      initialTags[item.slug] =
        item.weight_override !== undefined && item.weight_override !== null
          ? String(item.weight_override)
          : '';
    }
    setTagState(initialTags);
  }, [form, node, open]);

  const ipManualOverride = form.watch('ip_manual_override');
  const geoManualOverride = form.watch('geo_manual_override');

  const submitForm = async (values: NodeFormValues) => {
    await onSubmit(toPayload(values, tagState));
  };

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onClose()}>
      <DialogContent className='max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>
            {node ? t('editor.editTitle') : t('editor.createTitle')}
          </DialogTitle>
          <DialogDescription>{t('editor.description')}</DialogDescription>
        </DialogHeader>

        <form
          className='space-y-4'
          onSubmit={form.handleSubmit(submitForm)}
          noValidate
        >
          <div className='space-y-2'>
            <Label htmlFor='node-name'>{t('editor.name')}</Label>
            <Input
              id='node-name'
              placeholder={t('editor.namePlaceholder')}
              {...form.register('name')}
            />
            {form.formState.errors.name ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.name.message}
              </p>
            ) : null}
          </div>

          <div className='space-y-2'>
            <Label htmlFor='node-ip'>{t('editor.ip')}</Label>
            <Input
              id='node-ip'
              placeholder={t('editor.ipPlaceholder')}
              {...form.register('ip')}
            />
            {form.formState.errors.ip ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.ip.message}
              </p>
            ) : null}
          </div>

          <div className='space-y-2'>
            <Label htmlFor='node-network-group'>
              {t('editor.networkGroup')}
            </Label>
            <Input
              id='node-network-group'
              placeholder={t('editor.networkGroupPlaceholder')}
              {...form.register('network_group')}
            />
            <p className='text-xs text-muted-foreground'>
              {t('editor.networkGroupDesc')}
            </p>
          </div>

          <div className='grid gap-3 sm:grid-cols-3'>
            <div className='space-y-1.5'>
              <Label htmlFor='node-cap-bw'>{t('editor.capBandwidth')}</Label>
              <Input
                id='node-cap-bw'
                type='number'
                min={0}
                step={100}
                {...form.register('capacity_bandwidth_mbps')}
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='node-cap-conn'>{t('editor.capConns')}</Label>
              <Input
                id='node-cap-conn'
                type='number'
                min={0}
                step={1000}
                {...form.register('capacity_connections')}
              />
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='node-cap-weight'>{t('editor.capWeight')}</Label>
              <Input
                id='node-cap-weight'
                type='number'
                min={0}
                step={10}
                {...form.register('capacity_weight')}
              />
            </div>
          </div>

          <div className='space-y-2 rounded-lg border border-dashed p-3'>
            <div>
              <p className='text-sm font-medium'>{t('editor.routingTags')}</p>
              <p className='text-xs text-muted-foreground'>
                {t('editor.routingTagsDesc')}
              </p>
            </div>
            {(tagsQuery.data ?? []).length === 0 ? (
              <p className='text-xs text-muted-foreground'>
                {t('editor.noRoutingTags')}
              </p>
            ) : (
              <div className='space-y-2'>
                {(tagsQuery.data ?? []).map((tag) => {
                  const selected = tagState[tag.slug] !== undefined;
                  return (
                    <div
                      key={tag.id}
                      className='flex items-center justify-between gap-2 rounded-md border border-dashed px-2.5 py-1.5'
                    >
                      <label className='flex cursor-pointer items-center gap-2 text-sm'>
                        <Checkbox
                          checked={selected}
                          onCheckedChange={(checked) => {
                            setTagState((prev) => {
                              const next = { ...prev };
                              if (checked) {
                                next[tag.slug] = prev[tag.slug] ?? '';
                              } else {
                                delete next[tag.slug];
                              }
                              return next;
                            });
                          }}
                        />
                        <span>{tag.name}</span>
                        <Badge
                          variant='outline'
                          className='font-mono text-[10px]'
                        >
                          {tag.slug}
                        </Badge>
                      </label>
                      {selected ? (
                        <Input
                          type='number'
                          step='0.1'
                          min='0.01'
                          max='100'
                          className='h-7 w-28 text-xs'
                          placeholder={String(tag.default_weight)}
                          value={tagState[tag.slug] ?? ''}
                          onChange={(e) =>
                            setTagState((prev) => ({
                              ...prev,
                              [tag.slug]: e.target.value,
                            }))
                          }
                        />
                      ) : null}
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
            <div>
              <p className='text-sm font-medium'>{t('editor.lockIp')}</p>
              <p className='text-xs text-muted-foreground'>
                {t('editor.lockIpDesc')}
              </p>
            </div>
            <Switch
              checked={ipManualOverride}
              onCheckedChange={(checked) =>
                form.setValue('ip_manual_override', checked)
              }
            />
          </div>

          <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
            <div>
              <p className='text-sm font-medium'>{t('editor.autoUpdate')}</p>
              <p className='text-xs text-muted-foreground'>
                {t(
                  node && !node.edge_update_supported
                    ? 'edge.updateUnsupported'
                    : 'editor.autoUpdateDesc',
                )}
              </p>
            </div>
            <Switch
              aria-label={t('editor.autoUpdate')}
              checked={form.watch('auto_update_enabled')}
              onCheckedChange={(checked) =>
                form.setValue('auto_update_enabled', checked)
              }
            />
          </div>

          <div className='flex items-center justify-between rounded-lg border px-3 py-2'>
            <div>
              <p className='text-sm font-medium'>{t('editor.manualGeo')}</p>
              <p className='text-xs text-muted-foreground'>
                {t('editor.manualGeoDesc')}
              </p>
            </div>
            <Switch
              checked={geoManualOverride}
              onCheckedChange={(checked) =>
                form.setValue('geo_manual_override', checked)
              }
            />
          </div>

          {geoManualOverride ? (
            <div className='grid gap-3 sm:grid-cols-3'>
              <div className='space-y-2 sm:col-span-3'>
                <Label htmlFor='geo-name'>{t('editor.geoName')}</Label>
                <Input id='geo-name' {...form.register('geo_name')} />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='geo-lat'>{t('editor.latitude')}</Label>
                <Input id='geo-lat' {...form.register('geo_latitude')} />
              </div>
              <div className='space-y-2 sm:col-span-2'>
                <Label htmlFor='geo-lng'>{t('editor.longitude')}</Label>
                <Input id='geo-lng' {...form.register('geo_longitude')} />
              </div>
            </div>
          ) : null}

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={onClose}
              disabled={submitting}
            >
              {tc('cancel')}
            </Button>
            <Button type='submit' disabled={submitting}>
              {submitting
                ? t('editor.saving')
                : node
                  ? t('editor.saveEdit')
                  : t('editor.createAction')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
