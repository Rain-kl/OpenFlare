'use client';

import { useEffect, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { z } from 'zod';

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
import {
  TlsCertificateService,
  ZoneDomainService,
  ZoneService,
  zoneQueryKey,
  type ZoneDomainItem,
  type ZoneItem,
} from '@/lib/services/openflare';

import {
  previewZoneDomainInput,
  resolveZoneDomainInput,
} from './resolve-zone-domain-input';

const schema = z.object({
  zone_id: z.string().min(1, '请选择 Zone'),
  domain_input: z.string().trim().min(1, '请输入域名'),
  cert_id: z.string(),
});

type Values = z.infer<typeof schema>;

export function QuickCreateZoneDomainDialog({
  open,
  onOpenChange,
  fixedZoneId,
  fixedZoneRoot,
  zones: zonesProp,
  onCreated,
}: {
  open: boolean;
  onOpenChange(open: boolean): void;
  /** When set, Zone 选择器隐藏 */
  fixedZoneId?: number;
  fixedZoneRoot?: string;
  zones?: ZoneItem[];
  onCreated(domain: ZoneDomainItem): void | Promise<void>;
}) {
  const queryClient = useQueryClient();
  const zonesQuery = useQuery({
    queryKey: zoneQueryKey,
    queryFn: () => ZoneService.list(),
    enabled: open && !fixedZoneId && !zonesProp,
  });
  const certificatesQuery = useQuery({
    queryKey: ['openflare', 'tls-certificates'],
    queryFn: () => TlsCertificateService.list(),
    enabled: open,
  });

  const zones = useMemo(
    () => zonesProp ?? zonesQuery.data ?? [],
    [zonesProp, zonesQuery.data],
  );
  const fixedZone = useMemo(() => {
    if (!fixedZoneId) {
      return undefined;
    }
    return (
      zones.find((zone) => zone.id === fixedZoneId) ??
      (fixedZoneRoot
        ? ({
            id: fixedZoneId,
            domain: fixedZoneRoot,
            created_at: '',
            updated_at: '',
          } as ZoneItem)
        : undefined)
    );
  }, [fixedZoneId, fixedZoneRoot, zones]);

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      zone_id: fixedZoneId ? String(fixedZoneId) : '',
      domain_input: '',
      cert_id: '',
    },
  });

  useEffect(() => {
    if (!open) {
      return;
    }
    form.reset({
      zone_id: fixedZoneId ? String(fixedZoneId) : '',
      domain_input: '',
      cert_id: '',
    });
  }, [fixedZoneId, form, open]);

  const watchedZoneId = form.watch('zone_id');
  const watchedInput = form.watch('domain_input');
  const selectedZone = useMemo(() => {
    if (fixedZone) {
      return fixedZone;
    }
    const id = Number(watchedZoneId);
    return zones.find((zone) => zone.id === id);
  }, [fixedZone, watchedZoneId, zones]);

  const preview = selectedZone
    ? previewZoneDomainInput(watchedInput, selectedZone.domain)
    : '';

  const mutation = useMutation({
    mutationFn: async (values: Values) => {
      const zoneId = Number(values.zone_id);
      const zone = fixedZone ?? zones.find((item) => item.id === zoneId);
      if (!zone) {
        throw new Error('请选择 Zone');
      }
      const resolved = resolveZoneDomainInput(values.domain_input, zone.domain);
      if (resolved.error || !resolved.domain) {
        throw new Error(resolved.error || '域名格式不合法');
      }
      return ZoneDomainService.create(zone.id, {
        domain: resolved.domain,
        cert_id: values.cert_id ? Number(values.cert_id) : null,
      });
    },
    onSuccess: async (domain) => {
      toast.success('域名已添加', { description: domain.domain });
      await Promise.all([
        onCreated(domain),
        queryClient.invalidateQueries({ queryKey: zoneQueryKey }),
        queryClient.invalidateQueries({
          queryKey: [...zoneQueryKey, 'all-domains'],
        }),
      ]);
      onOpenChange(false);
    },
    onError: (error) =>
      toast.error(error instanceof Error ? error.message : '添加失败'),
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>快捷新增域名</DialogTitle>
          <DialogDescription>
            选择 Zone 后输入：子域名称（如 api）、@（根域）或完整 FQDN。
          </DialogDescription>
        </DialogHeader>

        <form
          id='quick-create-zone-domain'
          className='space-y-4'
          onSubmit={form.handleSubmit((values) => {
            if (!selectedZone) {
              form.setError('zone_id', { message: '请选择 Zone' });
              return;
            }
            const resolved = resolveZoneDomainInput(
              values.domain_input,
              selectedZone.domain,
            );
            if (resolved.error) {
              form.setError('domain_input', { message: resolved.error });
              return;
            }
            mutation.mutate(values);
          })}
        >
          {!fixedZoneId ? (
            <div className='space-y-1.5'>
              <Label>Zone</Label>
              <Select
                value={form.watch('zone_id') || undefined}
                onValueChange={(value) => form.setValue('zone_id', value)}
              >
                <SelectTrigger>
                  <SelectValue placeholder='选择注册根域' />
                </SelectTrigger>
                <SelectContent>
                  {zones.map((zone) => (
                    <SelectItem key={zone.id} value={String(zone.id)}>
                      {zone.domain}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {form.formState.errors.zone_id ? (
                <p className='text-xs text-destructive'>
                  {form.formState.errors.zone_id.message}
                </p>
              ) : null}
            </div>
          ) : (
            <div className='rounded-md border bg-muted/30 px-3 py-2 text-sm'>
              Zone：
              <span className='ml-1 font-medium'>
                {fixedZoneRoot || fixedZone?.domain || `#${fixedZoneId}`}
              </span>
            </div>
          )}

          <div className='space-y-1.5'>
            <Label htmlFor='domain-input'>域名</Label>
            <Input
              id='domain-input'
              placeholder={
                selectedZone
                  ? `api 或 @ 或 api.${selectedZone.domain}`
                  : 'api / @ / 完整域名'
              }
              {...form.register('domain_input')}
            />
            {preview ? (
              <p className='text-xs text-muted-foreground'>
                将创建：
                <code className='ml-1 rounded bg-muted px-1 py-0.5 font-mono text-[11px]'>
                  {preview}
                </code>
              </p>
            ) : (
              <p className='text-xs text-muted-foreground'>
                示例：输入 <code className='font-mono'>api</code> →{' '}
                <code className='font-mono'>api.zone.com</code>；
                <code className='font-mono'>@</code> → 根域本身
              </p>
            )}
            {form.formState.errors.domain_input ? (
              <p className='text-xs text-destructive'>
                {form.formState.errors.domain_input.message}
              </p>
            ) : null}
          </div>

          <div className='space-y-1.5'>
            <Label>证书（可选）</Label>
            <Select
              value={form.watch('cert_id') || '__none'}
              onValueChange={(value) =>
                form.setValue('cert_id', value === '__none' ? '' : value)
              }
            >
              <SelectTrigger>
                <SelectValue placeholder='不绑定证书' />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='__none'>不绑定证书</SelectItem>
                {(certificatesQuery.data ?? []).map((certificate) => (
                  <SelectItem
                    key={certificate.id}
                    value={String(certificate.id)}
                  >
                    {certificate.name} · {certificate.primary_domain}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </form>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            type='submit'
            form='quick-create-zone-domain'
            disabled={mutation.isPending}
          >
            {mutation.isPending ? (
              <Loader2 className='mr-1 size-4 animate-spin' />
            ) : null}
            添加域名
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
