'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ExternalLink,
  FileWarning,
  Loader2,
  RotateCcw,
  Save,
  Sparkles,
} from 'lucide-react';
import { toast } from 'sonner';

import { useAuth } from '@/components/providers/auth-provider';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
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
import { Textarea } from '@/components/ui/textarea';
import {
  DEFAULT_ORIGIN_ERROR_PAGE_HTML,
  ORIGIN_ERROR_PAGE_HTML_MAX_BYTES,
  previewOriginErrorPageHTML,
} from '@/lib/openflare/default-origin-error-page-html';
import {
  DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS,
  parseStatusCodeTagsJSON,
  validateStatusCodeTagMessage,
  validateStatusCodeTags,
} from '@/lib/openflare/status-code-tags';
import { OptionService } from '@/lib/services/openflare';

const optionsQueryKey = ['openflare', 'options'] as const;

const KEY_ENABLED = 'origin_error_page_enabled';
const KEY_STATUS_CODES = 'origin_error_page_status_codes';
const KEY_HTML = 'origin_error_page_html';

type ErrorPageFields = {
  enabled: boolean;
  statusCodes: string[];
  html: string;
};

const defaultFields: ErrorPageFields = {
  enabled: true,
  statusCodes: [...DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS],
  html: '',
};

function optionsToMap(options: Array<{ key: string; value: string }>) {
  return options.reduce<Record<string, string>>((acc, option) => {
    acc[option.key] = option.value;
    return acc;
  }, {});
}

function mapOptionsToFields(
  optionMap: Record<string, string>,
): ErrorPageFields {
  const enabledRaw = optionMap[KEY_ENABLED];
  return {
    enabled: enabledRaw === undefined ? true : enabledRaw === 'true',
    statusCodes: parseStatusCodeTagsJSON(optionMap[KEY_STATUS_CODES]),
    html: optionMap[KEY_HTML] ?? '',
  };
}

function validateFields(fields: ErrorPageFields) {
  validateStatusCodeTags(fields.statusCodes);

  const htmlBytes = new TextEncoder().encode(fields.html).length;
  if (htmlBytes > ORIGIN_ERROR_PAGE_HTML_MAX_BYTES) {
    throw new Error(
      `HTML 超过最大长度限制（${ORIGIN_ERROR_PAGE_HTML_MAX_BYTES} 字节）`,
    );
  }
}

export default function ErrorPagesPage() {
  const { user, loading: authLoading } = useAuth();
  const queryClient = useQueryClient();
  const [fields, setFields] = useState<ErrorPageFields>(defaultFields);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [tagError, setTagError] = useState<string | null>(null);

  const optionsQuery = useQuery({
    queryKey: optionsQueryKey,
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

  const saveMutation = useMutation({
    mutationFn: async () => {
      validateFields(fields);
      await OptionService.updateBatch([
        { key: KEY_ENABLED, value: String(fields.enabled) },
        {
          key: KEY_STATUS_CODES,
          value: JSON.stringify(fields.statusCodes),
        },
        { key: KEY_HTML, value: fields.html },
      ]);
    },
    onSuccess: async () => {
      toast.success('源站错误页已保存，请前往版本发布使配置生效');
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: optionsQueryKey }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'config-preview'],
        }),
        queryClient.invalidateQueries({
          queryKey: ['openflare', 'config-versions'],
        }),
      ]);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  const handleStatusCodesChange = (statusCodes: string[]) => {
    setFields((prev) => ({ ...prev, statusCodes }));
    setTagError(null);
  };

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

  const loadDefaultTemplate = () => {
    setFields((prev) => ({
      ...prev,
      html: DEFAULT_ORIGIN_ERROR_PAGE_HTML,
    }));
    toast.success('已加载默认 HTML 模板到编辑器');
  };

  const restoreDefault = () => {
    setFields((prev) => ({
      ...prev,
      html: '',
      statusCodes: [...DEFAULT_ORIGIN_ERROR_PAGE_STATUS_TAGS],
    }));
    setRestoreOpen(false);
    setTagError(null);
    toast.success('已恢复默认：状态码 500-599，HTML 使用服务端内置模板');
  };

  if (authLoading) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder
          icon={FileWarning}
          description='加载权限信息...'
        />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='py-6 px-1'>
        <EmptyStateWithBorder
          icon={FileWarning}
          title='权限不足'
          description='只有管理员可以访问源站错误页设置。'
        />
      </div>
    );
  }

  if (optionsQuery.isLoading) {
    return (
      <div className='py-6 px-1'>
        <LoadingStateWithBorder
          icon={FileWarning}
          description='加载错误页配置...'
        />
      </div>
    );
  }

  if (optionsQuery.isError) {
    return (
      <div className='py-6 px-1'>
        <ErrorInline
          message={
            optionsQuery.error instanceof Error
              ? optionsQuery.error.message
              : '加载失败'
          }
          onRetry={() => void optionsQuery.refetch()}
        />
      </div>
    );
  }

  return (
    <div className='flex flex-col gap-6 py-6 px-1'>
      <div className='flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between'>
        <div className='flex items-center gap-2'>
          <FileWarning className='size-5 text-primary' />
          <div>
            <h1 className='text-2xl font-semibold tracking-tight'>错误页</h1>
            <p className='text-sm text-muted-foreground'>
              配置源站/网关错误响应时的统一 HTML
              页面。保存后需发布配置版本后生效。
            </p>
          </div>
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button variant='outline' size='sm' asChild>
            <Link href='/config-versions'>
              <ExternalLink className='size-3.5' />
              查看配置预览
            </Link>
          </Button>
          <Button
            size='sm'
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
        </div>
      </div>

      <Card className='border-dashed shadow-none'>
        <CardHeader className='flex flex-row items-center justify-between gap-4'>
          <div>
            <CardTitle className='text-base'>启用源站错误页</CardTitle>
            <CardDescription>
              关闭后透传源站或 Nginx 默认错误响应，不注入 error_page 指令。
            </CardDescription>
          </div>
          <Switch
            checked={fields.enabled}
            onCheckedChange={(enabled) =>
              setFields((prev) => ({ ...prev, enabled }))
            }
            aria-label='启用源站错误页'
          />
        </CardHeader>
      </Card>

      <Card className='border-dashed shadow-none'>
        <CardHeader>
          <CardTitle className='text-base'>触发状态码</CardTitle>
          <CardDescription>
            支持单码（如 502）或闭区间（如 500-599），范围 400–599。默认
            500-599。
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-2'>
          <Label htmlFor='origin-error-status-codes' className='sr-only'>
            状态码标签
          </Label>
          <TagsInput
            id='origin-error-status-codes'
            value={fields.statusCodes}
            onChange={handleStatusCodesChange}
            validateTag={handleValidateTag}
            placeholder='例如 502 或 500-599，回车添加'
            aria-invalid={!!tagError}
          />
          {tagError ? (
            <p className='text-xs text-destructive'>{tagError}</p>
          ) : (
            <p className='text-xs text-muted-foreground'>
              输入后按 Enter 或逗号添加；Backspace 可删除最后一个标签。
            </p>
          )}
        </CardContent>
      </Card>

      <div className='grid gap-6 xl:grid-cols-2'>
        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
            <div>
              <CardTitle className='text-base'>HTML 模板</CardTitle>
              <CardDescription>
                支持占位符 {'{{status}}'} 与 {'{{host}}'}
                。留空则使用服务端内置默认页。
              </CardDescription>
            </div>
            <div className='flex flex-wrap gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={loadDefaultTemplate}
              >
                <Sparkles className='size-3.5' />
                加载默认模板
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => setRestoreOpen(true)}
              >
                <RotateCcw className='size-3.5' />
                恢复默认
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <Textarea
              id='origin-error-html'
              value={fields.html}
              onChange={(event) =>
                setFields((prev) => ({ ...prev, html: event.target.value }))
              }
              placeholder='留空使用内置默认模板…'
              className='min-h-[28rem] font-mono text-xs leading-relaxed'
              spellCheck={false}
            />
            <p className='mt-2 text-xs text-muted-foreground'>
              当前 {new TextEncoder().encode(fields.html).length} /{' '}
              {ORIGIN_ERROR_PAGE_HTML_MAX_BYTES} 字节
              {fields.html.trim() === '' ? '（使用内置默认）' : ''}
            </p>
          </CardContent>
        </Card>

        <Card className='border-dashed shadow-none'>
          <CardHeader>
            <CardTitle className='text-base'>预览</CardTitle>
            <CardDescription>
              客户端预览：{'{{status}}'}→502，{'{{host}}'}→example.com
            </CardDescription>
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

      <AlertDialog open={restoreOpen} onOpenChange={setRestoreOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>恢复默认配置？</AlertDialogTitle>
            <AlertDialogDescription>
              将状态码重置为 500-599，并将 HTML
              清空为「使用服务端内置默认模板」。此操作不会自动保存，仍需点击保存并发布配置版本。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={restoreDefault}>
              确认恢复
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
