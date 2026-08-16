'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Expand, Loader2, Pencil, Plus, Save, X } from 'lucide-react';
import { toast } from 'sonner';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { effectiveOfflinePageHTML } from '@/lib/openflare/offline-page-templates';
import {
  OptionService,
  ZoneService,
  zoneQueryKey,
} from '@/lib/services/openflare';

import { useTranslations } from 'next-intl';

import { ScopeDomainDialog } from './scope-domain-dialog';
import {
  defaultOfflinePageFields,
  invalidateResponseQueries,
  KEY_SW_DOMAINS,
  KEY_SW_ENABLED,
  KEY_SW_HTML,
  mapOptionsToOfflineFields,
  type OfflinePageFields,
} from './shared';

export function OfflinePageTab({
  optionMap,
}: {
  optionMap: Record<string, string>;
}) {
  const t = useTranslations('responses');
  const tc = useTranslations('common');
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<OfflinePageFields>(
    defaultOfflinePageFields,
  );
  const [scopeOpen, setScopeOpen] = useState(false);

  useEffect(() => {
    setFields(mapOptionsToOfflineFields(optionMap));
  }, [optionMap]);

  const previewSrcDoc = useMemo(
    () => effectiveOfflinePageHTML(fields.html),
    [fields.html],
  );

  const zonesQuery = useQuery({
    queryKey: [...zoneQueryKey, 'sw-scope'],
    queryFn: async () => {
      const zones = await ZoneService.list();
      const overviews = await Promise.all(
        zones.map((zone) => ZoneService.getOverview(zone.id)),
      );
      return overviews.map((ov) => ({
        zoneDomain: ov.zone.domain,
        domains: [ov.zone.domain, ...ov.domains.map((d) => d.domain)],
      }));
    },
  });

  const saveMutation = useMutation({
    mutationFn: async () => {
      await OptionService.updateBatch([
        { key: KEY_SW_ENABLED, value: String(fields.enabled) },
        { key: KEY_SW_HTML, value: fields.html },
        { key: KEY_SW_DOMAINS, value: JSON.stringify(fields.domains) },
      ]);
    },
    onSuccess: async () => {
      toast.success(t('offlineSaved'));
      await invalidateResponseQueries(queryClient);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('saveFailed'));
    },
  });

  return (
    <div className='space-y-6'>
      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-start justify-between gap-4 space-y-0'>
          <div className='space-y-1.5'>
            <CardTitle className='text-base'>{t('offlinePage')}</CardTitle>
            <CardDescription>{t('offlinePageDesc')}</CardDescription>
          </div>
          <Button
            size='sm'
            className='shrink-0'
            disabled={saveMutation.isPending}
            onClick={() => saveMutation.mutate()}
          >
            {saveMutation.isPending ? (
              <Loader2 className='size-3.5 animate-spin' />
            ) : (
              <Save className='size-3.5' />
            )}
            {tc('save')}
          </Button>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex items-start justify-between gap-6'>
            <div className='space-y-1'>
              <Label className='text-sm font-medium'>
                {t('enableOffline')}
              </Label>
              <p className='text-sm text-muted-foreground'>
                {t('enableOfflineDesc')}
              </p>
            </div>
            <Switch
              checked={fields.enabled}
              onCheckedChange={(enabled) =>
                setFields((prev) => ({ ...prev, enabled }))
              }
              aria-label={t('enableOfflineAria')}
              className='mt-0.5 shrink-0'
            />
          </div>
          <div className='space-y-2'>
            <div className='flex items-center justify-between gap-3'>
              <div>
                <Label className='text-sm font-medium'>{t('scope')}</Label>
                <p className='text-sm text-muted-foreground'>
                  {t('scopeDesc')}
                </p>
              </div>
              <Button
                type='button'
                size='sm'
                variant='outline'
                disabled={!fields.enabled || saveMutation.isPending}
                onClick={() => setScopeOpen(true)}
              >
                <Plus className='size-3.5' />
                {t('addDomain')}
              </Button>
            </div>
            {fields.domains.length === 0 ? (
              <p className='text-sm text-muted-foreground'>
                {fields.enabled
                  ? t('noScopeSelected')
                  : t('enableThenSelectScope')}
              </p>
            ) : (
              <div className='flex flex-wrap gap-2'>
                {fields.domains.map((domain) => (
                  <Badge
                    key={domain}
                    variant='secondary'
                    className='gap-1 font-normal'
                  >
                    {domain}
                    <button
                      type='button'
                      className='hover:text-destructive'
                      disabled={!fields.enabled}
                      aria-label={t('removeDomain', { domain })}
                      onClick={() =>
                        setFields((prev) => ({
                          ...prev,
                          domains: prev.domains.filter((d) => d !== domain),
                        }))
                      }
                    >
                      <X className='size-3' />
                    </button>
                  </Badge>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
      <Card className='border-dashed shadow-none overflow-hidden'>
        <CardHeader className='flex flex-row items-start justify-between gap-3 space-y-0'>
          <div className='space-y-1.5'>
            <CardTitle className='text-base'>{t('pagePreview')}</CardTitle>
          </div>
          <div className='flex shrink-0 flex-wrap gap-2'>
            <Button variant='outline' size='sm' asChild>
              <Link href='/responses/offline/preview'>
                <Expand className='size-3.5' />
                {t('preview')}
              </Link>
            </Button>
            <Button size='sm' asChild>
              <Link href='/responses/offline/edit'>
                <Pencil className='size-3.5' />
                {t('edit')}
              </Link>
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className='overflow-hidden rounded-md border bg-muted/30'>
            <iframe
              title={t('offlinePreview')}
              sandbox=''
              srcDoc={previewSrcDoc}
              className='h-[32rem] w-full bg-background'
            />
          </div>
        </CardContent>
      </Card>
      <ScopeDomainDialog
        open={scopeOpen}
        onOpenChange={setScopeOpen}
        zones={zonesQuery.data ?? []}
        selected={fields.domains}
        pending={saveMutation.isPending}
        onSubmit={(domains) => {
          setFields((prev) => ({ ...prev, domains }));
          setScopeOpen(false);
        }}
      />
    </div>
  );
}
