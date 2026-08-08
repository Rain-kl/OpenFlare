'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2, Save } from 'lucide-react';
import { toast } from 'sonner';

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
import { HtmlEditorWorkspace } from '@/components/common/html-editor-workspace';
import { previewOriginErrorPageHTML } from '@/lib/openflare/default-origin-error-page-html';
import {
  validateStatusCodeTagMessage,
  validateStatusCodeTags,
} from '@/lib/openflare/status-code-tags';
import { OptionService } from '@/lib/services/openflare';
import { cn } from '@/lib/utils';

import {
  defaultErrorPageFields,
  invalidateResponseQueries,
  KEY_ENABLED,
  KEY_GET_ONLY,
  KEY_HTML,
  KEY_STATUS_CODES,
  mapOptionsToErrorFields,
  type ErrorPageFields,
} from './shared';

export function ErrorPageTab({
  optionMap,
}: {
  optionMap: Record<string, string>;
}) {
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<ErrorPageFields>(
    defaultErrorPageFields,
  );
  const [tagError, setTagError] = useState<string | null>(null);

  useEffect(() => {
    setFields(mapOptionsToErrorFields(optionMap));
    setTagError(null);
  }, [optionMap]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      validateStatusCodeTags(fields.statusCodes);
      await OptionService.updateBatch([
        { key: KEY_ENABLED, value: String(fields.enabled) },
        { key: KEY_GET_ONLY, value: String(fields.getOnly) },
        {
          key: KEY_STATUS_CODES,
          value: JSON.stringify(fields.statusCodes),
        },
        { key: KEY_HTML, value: fields.html },
      ]);
    },
    onSuccess: async () => {
      toast.success('源站错误页已保存，请前往版本发布使配置生效');
      await invalidateResponseQueries(queryClient);
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
    <div className='space-y-6'>
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

      <Card className='border-dashed shadow-none'>
        <CardHeader>
          <CardTitle className='text-base'>错误页 HTML</CardTitle>
          <CardDescription>留空则使用内置默认模板。</CardDescription>
        </CardHeader>
        <CardContent
          className={cn(!fields.enabled && 'pointer-events-none opacity-60')}
        >
          <HtmlEditorWorkspace
            value={fields.html}
            onChange={(v) => setFields((prev) => ({ ...prev, html: v }))}
            preview={previewOriginErrorPageHTML}
          />
        </CardContent>
      </Card>
    </div>
  );
}
