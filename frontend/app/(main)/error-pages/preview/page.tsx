'use client';

import Link from 'next/link';
import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Pencil, X } from 'lucide-react';

import { useAuth } from '@/components/providers/auth-provider';
import { Button } from '@/components/ui/button';
import { previewOriginErrorPageHTML } from '@/lib/openflare/default-origin-error-page-html';
import { OptionService } from '@/lib/services/openflare';

import { ErrorPageGate } from '../components/error-page-gate';
import {
  mapOptionsToFields,
  OPTIONS_QUERY_KEY,
  optionsToMap,
} from '../components/shared';

/**
 * 全屏「真实预览」：覆盖主布局，按访客视角展示替换后的 HTML。
 */
export default function ErrorPagePreviewPage() {
  const { user } = useAuth();

  const optionsQuery = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  const html = useMemo(() => {
    if (!optionsQuery.data) return '';
    return mapOptionsToFields(optionsToMap(optionsQuery.data)).html;
  }, [optionsQuery.data]);

  const previewSrcDoc = useMemo(() => previewOriginErrorPageHTML(html), [html]);

  return (
    <ErrorPageGate optionsQuery={optionsQuery}>
      <div className='fixed inset-0 z-50 flex flex-col bg-background'>
        <div className='flex items-center justify-between gap-3 border-b bg-background/95 px-4 py-2 backdrop-blur shrink-0'>
          <div className='flex items-center gap-2 min-w-0'>
            <Button variant='outline' size='icon' className='h-8 w-8' asChild>
              <Link href='/error-pages' aria-label='返回错误页'>
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
              <Link href='/error-pages/edit'>
                <Pencil className='size-3.5' />
                编辑
              </Link>
            </Button>
            <Button variant='ghost' size='icon' className='h-8 w-8' asChild>
              <Link href='/error-pages' aria-label='关闭预览'>
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
    </ErrorPageGate>
  );
}
