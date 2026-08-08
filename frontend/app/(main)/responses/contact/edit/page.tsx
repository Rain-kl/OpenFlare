'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, FileCode2, Loader2, RotateCcw, Save } from 'lucide-react';
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
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { HtmlEditorWorkspace } from '@/components/common/html-editor-workspace';
import { OptionService } from '@/lib/services/openflare';

import {
  invalidateResponseQueries,
  KEY_SW_HTML,
  mapOptionsToContactFields,
  OPTIONS_QUERY_KEY,
  optionsToMap,
} from '../../components/shared';

const CONTACT_PAGE_HTML_MAX_BYTES = 256 * 1024;

export default function ContactPageEditPage() {
  const { user, loading: authLoading } = useAuth();
  const queryClient = useQueryClient();
  const [html, setHtml] = useState('');
  const [restoreOpen, setRestoreOpen] = useState(false);

  const optionsQuery = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  useEffect(() => {
    if (!optionsQuery.data) return;
    setHtml(mapOptionsToContactFields(optionsToMap(optionsQuery.data)).html);
  }, [optionsQuery.data]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      const bytes = new TextEncoder().encode(html).length;
      if (bytes > CONTACT_PAGE_HTML_MAX_BYTES) {
        throw new Error('HTML 大小超出限制（最大 256KB）');
      }
      await OptionService.updateBatch([{ key: KEY_SW_HTML, value: html }]);
    },
    onSuccess: async () => {
      toast.success('离线联系页 HTML 已保存，请前往版本发布使配置生效');
      await invalidateResponseQueries(queryClient);
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : '保存失败');
    },
  });

  const restoreDefault = () => {
    setHtml('');
    setRestoreOpen(false);
    toast.success('已清空为使用内置默认模板（需保存后生效）');
  };

  if (authLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder
          icon={FileCode2}
          description='加载权限信息...'
        />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='w-full py-6 px-1'>
        <EmptyStateWithBorder
          icon={FileCode2}
          title='权限不足'
          description='只有管理员可以编辑联系页 HTML。'
        />
      </div>
    );
  }

  if (optionsQuery.isLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder
          icon={FileCode2}
          description='加载联系页配置...'
        />
      </div>
    );
  }

  if (optionsQuery.isError) {
    return (
      <div className='w-full py-6 px-1'>
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
    <div className='flex flex-col gap-4 py-6 px-1'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex items-center gap-3'>
          <Button variant='outline' size='icon' className='h-8 w-8' asChild>
            <Link href='/responses?tab=contact' aria-label='返回联系页'>
              <ArrowLeft className='size-4' />
            </Link>
          </Button>
          <div className='flex items-center gap-2'>
            <FileCode2 className='size-5 text-primary' />
            <div>
              <h1 className='text-2xl font-semibold tracking-tight'>
                编辑离线联系页 HTML
              </h1>
            </div>
          </div>
        </div>
      </div>

      <HtmlEditorWorkspace
        value={html}
        onChange={setHtml}
        preview={(h) => h}
        footerHint={null}
        showPreviewLink={false}
        previewTitle='离线联系页实时预览'
        toolbarRight={
          <>
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

      <AlertDialog open={restoreOpen} onOpenChange={setRestoreOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>恢复默认 HTML？</AlertDialogTitle>
            <AlertDialogDescription>
              将清空编辑器内容，保存后使用内置默认模板。此操作不会自动保存。
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
