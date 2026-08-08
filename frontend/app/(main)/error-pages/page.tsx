'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Expand, FileWarning, Loader2, Pencil, Save } from 'lucide-react';
import { toast } from 'sonner';

import { useAuth } from '@/components/providers/auth-provider';
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
import { TagsInput } from '@/components/ui/tags-input';
import { previewOriginErrorPageHTML } from '@/lib/openflare/default-origin-error-page-html';
import {
  validateStatusCodeTagMessage,
  validateStatusCodeTags,
} from '@/lib/openflare/status-code-tags';
import { OptionService } from '@/lib/services/openflare';

import { ErrorPageGate } from './components/error-page-gate';
import {
  defaultErrorPageFields,
  invalidateErrorPageQueries,
  KEY_ENABLED,
  KEY_GET_ONLY,
  KEY_STATUS_CODES,
  mapOptionsToFields,
  OPTIONS_QUERY_KEY,
  optionsToMap,
  type ErrorPageFields,
} from './components/shared';

export default function ErrorPagesPage() {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<ErrorPageFields>(defaultErrorPageFields);
  const [tagError, setTagError] = useState<string | null>(null);

  const optionsQuery = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  useEffect(() => {
    if (!optionsQuery.data) return;
    setFields(mapOptionsToFields(optionsToMap(optionsQuery.data)));
    setTagError(null);
  }, [optionsQuery.data]);

  const previewSrcDoc = useMemo(
    () => previewOriginErrorPageHTML(fields.html),
    [fields.html],
  );

  /** 仅保存策略项（HTML 在编辑页单独保存） */
  const savePolicyMutation = useMutation({
    mutationFn: async () => {
      validateStatusCodeTags(fields.statusCodes);
      await OptionService.updateBatch([
        { key: KEY_ENABLED, value: String(fields.enabled) },
        { key: KEY_GET_ONLY, value: String(fields.getOnly) },
        {
          key: KEY_STATUS_CODES,
          value: JSON.stringify(fields.statusCodes),
        },
      ]);
    },
    onSuccess: async () => {
      toast.success('触发策略已保存，请前往版本发布使配置生效');
      await invalidateErrorPageQueries(queryClient);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  const handleValidateTag = (tag: string) => {
    const message = validateStatusCodeTagMessage(tag);
    if (message) {
      setTagError(message);
      toast.error(message);
      return message;
    }
    setTagError(null);
    return null;
  };

  return (
    <ErrorPageGate optionsQuery={optionsQuery}>
      <div className='flex flex-col gap-6 py-6 px-1'>
        <div className='flex items-center gap-2'>
          <FileWarning className='size-5 text-primary' />
          <div>
            <h1 className='text-2xl font-semibold tracking-tight'>错误页</h1>
            <p className='text-sm text-muted-foreground'>
              配置源站/网关错误响应时的统一 HTML
              页面。策略与模板保存后需发布配置版本后生效。
            </p>
          </div>
        </div>

        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-start justify-between gap-4 space-y-0'>
            <div className='space-y-1.5'>
              <CardTitle className='text-base'>触发策略</CardTitle>
              <CardDescription>
                配置源站错误页的启用条件与触发状态码。修改后需点击保存。
              </CardDescription>
            </div>
            <Button
              size='sm'
              className='shrink-0'
              disabled={savePolicyMutation.isPending}
              onClick={() => savePolicyMutation.mutate()}
            >
              {savePolicyMutation.isPending ? (
                <Loader2 className='size-3.5 animate-spin' />
              ) : (
                <Save className='size-3.5' />
              )}
              保存
            </Button>
          </CardHeader>
          <CardContent className='space-y-0 divide-y'>
            <div className='flex items-start justify-between gap-6 pb-5'>
              <div className='space-y-1'>
                <Label className='text-sm font-medium'>启用源站错误页</Label>
                <p className='text-sm text-muted-foreground'>
                  关闭后会透传源站或 Nginx 默认错误响应。
                </p>
              </div>
              <Switch
                checked={fields.enabled}
                onCheckedChange={(enabled) =>
                  setFields((prev) => ({ ...prev, enabled }))
                }
                aria-label='启用源站错误页'
                className='mt-0.5 shrink-0'
              />
            </div>

            <div className='flex items-start justify-between gap-6 py-5'>
              <div className='space-y-1'>
                <Label className='text-sm font-medium'>仅针对 GET 请求</Label>
                <p className='text-sm text-muted-foreground'>
                  开启后仅对 GET 请求的匹配错误状态码返回自定义错误页；POST/PUT
                  等其它方法直接透传源站响应。
                </p>
              </div>
              <Switch
                checked={fields.getOnly}
                disabled={!fields.enabled}
                onCheckedChange={(getOnly) =>
                  setFields((prev) => ({ ...prev, getOnly }))
                }
                aria-label='仅针对 GET 请求'
                className='mt-0.5 shrink-0'
              />
            </div>

            <div className='flex flex-col gap-3 pt-5'>
              <div className='space-y-1'>
                <Label
                  htmlFor='origin-error-status-codes'
                  className='text-sm font-medium'
                >
                  触发状态码
                </Label>
                <p className='text-sm text-muted-foreground'>
                  支持单码（如 502）或闭区间（如 500-599），范围 400–599。默认
                  500-599。
                </p>
              </div>
              <TagsInput
                id='origin-error-status-codes'
                value={fields.statusCodes}
                onChange={(statusCodes) => {
                  setFields((prev) => ({ ...prev, statusCodes }));
                  setTagError(null);
                }}
                validateTag={handleValidateTag}
                placeholder='例如 502 或 500-599，回车添加'
                aria-invalid={!!tagError}
                disabled={!fields.enabled}
              />
              {tagError ? (
                <p className='text-xs text-destructive'>{tagError}</p>
              ) : null}
            </div>
          </CardContent>
        </Card>

        <Card className='border-dashed shadow-none overflow-hidden'>
          <CardHeader className='flex flex-row items-start justify-between gap-3 space-y-0'>
            <div className='space-y-1.5'>
              <CardTitle className='text-base'>页面预览</CardTitle>
            </div>
            <div className='flex shrink-0 flex-wrap gap-2'>
              <Button variant='outline' size='sm' asChild>
                <Link href='/error-pages/preview'>
                  <Expand className='size-3.5' />
                  预览
                </Link>
              </Button>
              <Button size='sm' asChild>
                <Link href='/error-pages/edit'>
                  <Pencil className='size-3.5' />
                  编辑
                </Link>
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <div className='overflow-hidden rounded-md border bg-muted/30'>
              <iframe
                title='源站错误页预览'
                sandbox=''
                srcDoc={previewSrcDoc}
                className='h-[32rem] w-full bg-background'
              />
            </div>
          </CardContent>
        </Card>
      </div>
    </ErrorPageGate>
  );
}
