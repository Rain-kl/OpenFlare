'use client';

import Link from 'next/link';
import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Pencil, X } from 'lucide-react';

import { useAuth } from '@/components/providers/auth-provider';
import { Button } from '@/components/ui/button';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { previewOriginErrorPageHTML } from '@/lib/openflare/default-origin-error-page-html';
import { OptionService } from '@/lib/services/openflare';

import {
  mapOptionsToErrorFields,
  OPTIONS_QUERY_KEY,
  optionsToMap,
} from '../../components/shared';

export default function ErrorPagePreviewPage() {
  const { user, loading: authLoading } = useAuth();

  const optionsQuery = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  const html = useMemo(() => {
    if (!optionsQuery.data) return '';
    return mapOptionsToErrorFields(optionsToMap(optionsQuery.data)).html;
  }, [optionsQuery.data]);

  const previewSrcDoc = useMemo(() => previewOriginErrorPageHTML(html), [html]);

  if (authLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder description='加载权限信息...' />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='w-full py-6 px-1'>
        <EmptyStateWithBorder
          title='权限不足'
          description='只有管理员可以预览错误页。'
        />
      </div>
    );
  }

  if (optionsQuery.isLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder description='加载错误页配置...' />
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
    <div className='fixed inset-0 z-50 flex flex-col bg-background'>
      <div className='flex items-center justify-between gap-3 border-b bg-background/95 px-4 py-2 backdrop-blur shrink-0'>
        <div className='flex items-center gap-2 min-w-0'>
          <Button variant='outline' size='icon' className='h-8 w-8' asChild>
            <Link href='/responses?tab=error' aria-label='返回错误页'>
              <ArrowLeft className='size-4' />
            </Link>
          </Button>
          <div className='min-w-0'>
            <p className='text-sm font-semibold truncate'>错误页真实预览</p>
            <p className='text-[11px] text-muted-foreground font-mono truncate'>
              {'{{status}}'}→502 · {'{{host}}'}→example.com · 全屏展示
            </p>
          </div>
        </div>
        <div className='flex items-center gap-2 shrink-0'>
          <Button variant='outline' size='sm' asChild>
            <Link href='/responses/error-page/edit'>
              <Pencil className='size-3.5' />
              编辑
            </Link>
          </Button>
          <Button variant='ghost' size='icon' className='h-8 w-8' asChild>
            <Link href='/responses?tab=error' aria-label='关闭预览'>
              <X className='size-4' />
            </Link>
          </Button>
        </div>
      </div>
      <iframe
        title='源站错误页真实预览'
        sandbox=''
        srcDoc={previewSrcDoc}
        className='flex-1 w-full border-0 bg-background min-h-0'
      />
    </div>
  );
}
