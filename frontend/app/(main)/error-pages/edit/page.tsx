'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  FileCode2,
  Loader2,
  RotateCcw,
  Save,
  Sparkles,
} from 'lucide-react';
import { toast } from 'sonner';

import { useAuth } from '@/components/providers/auth-provider';
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  DEFAULT_ORIGIN_ERROR_PAGE_TEMPLATE_ID,
  getOriginErrorPageTemplate,
  ORIGIN_ERROR_PAGE_TEMPLATES,
} from '@/lib/openflare/origin-error-page-templates';
import { OptionService } from '@/lib/services/openflare';

import { HtmlEditorWorkspace } from '@/components/common/html-editor-workspace';
import { ErrorPageGate } from '../components/error-page-gate';
import {
  invalidateErrorPageQueries,
  KEY_HTML,
  mapOptionsToFields,
  OPTIONS_QUERY_KEY,
  optionsToMap,
  validateErrorPageHTML,
} from '../components/shared';

export default function ErrorPageEditPage() {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [html, setHtml] = useState('');
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [templateId, setTemplateId] = useState(
    DEFAULT_ORIGIN_ERROR_PAGE_TEMPLATE_ID,
  );

  const optionsQuery = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  useEffect(() => {
    if (!optionsQuery.data) return;
    setHtml(mapOptionsToFields(optionsToMap(optionsQuery.data)).html);
  }, [optionsQuery.data]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      validateErrorPageHTML(html);
      await OptionService.updateBatch([{ key: KEY_HTML, value: html }]);
    },
    onSuccess: async () => {
      toast.success('HTML 模板已保存，请前往版本发布使配置生效');
      await invalidateErrorPageQueries(queryClient);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  const loadSelectedTemplate = () => {
    const tmpl = getOriginErrorPageTemplate(templateId);
    if (!tmpl) {
      toast.error('未找到所选模板');
      return;
    }
    setHtml(tmpl.html);
    toast.success(`已加载模板「${tmpl.name}」到编辑器`);
  };

  const restoreDefault = () => {
    setHtml('');
    setRestoreOpen(false);
    toast.success('已清空为使用服务端内置默认模板（需保存后生效）');
  };

  return (
    <ErrorPageGate optionsQuery={optionsQuery}>
      <div className='flex flex-col gap-4 py-6 px-1'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='flex items-center gap-3'>
            <Button variant='outline' size='icon' className='h-8 w-8' asChild>
              <Link href='/error-pages' aria-label='返回错误页'>
                <ArrowLeft className='size-4' />
              </Link>
            </Button>
            <div className='flex items-center gap-2'>
              <FileCode2 className='size-5 text-primary' />
              <div>
                <h1 className='text-2xl font-semibold tracking-tight'>编辑</h1>
              </div>
            </div>
          </div>
        </div>

        <HtmlEditorWorkspace
          value={html}
          onChange={setHtml}
          toolbarRight={
            <>
              <div className='flex items-center gap-1.5'>
                <span className='text-[11px] text-muted-foreground hidden sm:inline'>
                  内置模板
                </span>
                <Select value={templateId} onValueChange={setTemplateId}>
                  <SelectTrigger className='h-7 w-[160px] text-[11px] bg-background'>
                    <SelectValue placeholder='选择模板' />
                  </SelectTrigger>
                  <SelectContent>
                    {ORIGIN_ERROR_PAGE_TEMPLATES.map((tmpl) => (
                      <SelectItem
                        key={tmpl.id}
                        value={tmpl.id}
                        className='text-xs'
                      >
                        {tmpl.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className='h-7 px-2 text-[11px]'
                  onClick={loadSelectedTemplate}
                >
                  <Sparkles className='size-3.5' />
                  加载模板
                </Button>
              </div>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                className='h-7 px-2 text-[11px]'
                onClick={() => setRestoreOpen(true)}
              >
                <RotateCcw className='size-3.5' />
                恢复默认
              </Button>
              <Button
                size='sm'
                className='h-7 px-3 text-[11px]'
                disabled={saveMutation.isPending}
                onClick={() => saveMutation.mutate()}
              >
                {saveMutation.isPending ? (
                  <Loader2 className='size-3 animate-spin' />
                ) : (
                  <Save className='size-3' />
                )}
                保存
              </Button>
            </>
          }
        />
      </div>

      <AlertDialog open={restoreOpen} onOpenChange={setRestoreOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>恢复默认 HTML？</AlertDialogTitle>
            <AlertDialogDescription>
              将清空编辑器内容，保存后使用服务端内置默认模板。此操作不会自动保存。
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
    </ErrorPageGate>
  );
}
