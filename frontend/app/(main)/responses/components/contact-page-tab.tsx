'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, Plus, Save, X } from 'lucide-react';
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
import { HtmlEditorWorkspace } from '@/components/common/html-editor-workspace';
import {
  OptionService,
  ZoneService,
  zoneQueryKey,
} from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import { ScopeDomainDialog } from './scope-domain-dialog';
import {
  defaultContactPageFields,
  invalidateResponseQueries,
  KEY_SW_DOMAINS,
  KEY_SW_ENABLED,
  KEY_SW_HTML,
  mapOptionsToContactFields,
  type ContactPageFields,
} from './shared';

export function ContactPageTab({
  optionMap,
}: {
  optionMap: Record<string, string>;
}) {
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<ContactPageFields>(
    defaultContactPageFields,
  );
  const [scopeOpen, setScopeOpen] = useState(false);

  useEffect(() => {
    setFields(mapOptionsToContactFields(optionMap));
  }, [optionMap]);

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
      toast.success('联系页已保存，请前往版本发布使配置生效');
      await invalidateResponseQueries(queryClient);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  return (
    <div className='space-y-6'>
      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-start justify-between gap-4 space-y-0'>
          <div className='space-y-1.5'>
            <CardTitle className='text-base'>离线兜底</CardTitle>
            <CardDescription>
              启用后给启用 HTTPS 的网站下发 Service
              Worker，域名被墙时浏览器从缓存展示此联系页。
            </CardDescription>
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
            保存
          </Button>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex items-start justify-between gap-6'>
            <div className='space-y-1'>
              <Label className='text-sm font-medium'>
                启用 Service Worker 离线兜底
              </Label>
              <p className='text-sm text-muted-foreground'>
                仅对 HTTPS 网站生效；未启用的站点不受影响。
              </p>
            </div>
            <Switch
              checked={fields.enabled}
              onCheckedChange={(enabled) =>
                setFields((prev) => ({ ...prev, enabled }))
              }
              aria-label='启用离线兜底'
              className='mt-0.5 shrink-0'
            />
          </div>
          <div className='space-y-2'>
            <div className='flex items-center justify-between gap-3'>
              <div>
                <Label className='text-sm font-medium'>生效范围</Label>
                <p className='text-sm text-muted-foreground'>
                  仅对选中的 HTTPS 域名生效；留空则不注入。
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
                添加域名
              </Button>
            </div>
            {fields.domains.length === 0 ? (
              <p className='text-sm text-muted-foreground'>
                {fields.enabled
                  ? '尚未选择生效域名，保存后不注入任何站点。'
                  : '启用离线兜底后可选择生效域名。'}
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
                      aria-label={`移除 ${domain}`}
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
      <Card className='border-dashed shadow-none'>
        <CardHeader>
          <CardTitle className='text-base'>联系页 HTML</CardTitle>
          <CardDescription>留空则使用内置默认模板。</CardDescription>
        </CardHeader>
        <CardContent
          className={cn(!fields.enabled && 'pointer-events-none opacity-60')}
        >
          <HtmlEditorWorkspace
            value={fields.html}
            onChange={(v) => setFields((prev) => ({ ...prev, html: v }))}
            preview={(html) => html}
            footerHint={null}
            showPreviewLink={false}
            previewTitle='离线联系页实时预览'
          />
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
