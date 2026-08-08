'use client';

import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { MessageSquareText } from 'lucide-react';

import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import { useAuth } from '@/components/providers/auth-provider';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { OptionService } from '@/lib/services/openflare';

import { ContactPageTab } from './components/contact-page-tab';
import { ErrorPageTab } from './components/error-page-tab';
import { OPTIONS_QUERY_KEY, optionsToMap } from './components/shared';

export default function ResponsesPage() {
  const { user, loading: authLoading } = useAuth();
  const [activeTab, setActiveTab] = useState('error');

  const optionsQuery = useQuery({
    queryKey: OPTIONS_QUERY_KEY,
    queryFn: () => OptionService.list(),
    enabled: !!user?.is_admin,
  });

  const optionMap = useMemo(
    () => optionsToMap(optionsQuery.data ?? []),
    [optionsQuery.data],
  );

  if (authLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder
          icon={MessageSquareText}
          description='加载权限信息...'
        />
      </div>
    );
  }

  if (!user?.is_admin) {
    return (
      <div className='w-full py-6 px-1'>
        <EmptyStateWithBorder
          icon={MessageSquareText}
          title='权限不足'
          description='只有管理员可以访问响应页面设置。'
        />
      </div>
    );
  }

  if (optionsQuery.isLoading) {
    return (
      <div className='w-full py-6 px-1'>
        <LoadingStateWithBorder
          icon={MessageSquareText}
          description='加载响应页面配置...'
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

  if (!optionsQuery.data) return null;

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex items-center gap-2'>
        <MessageSquareText className='size-5 text-primary' />
        <div>
          <h1 className='text-2xl font-semibold tracking-tight'>响应页面</h1>
          <p className='text-sm text-muted-foreground'>
            配置源站错误页与离线兜底联系页。保存后需发布配置版本后生效。
          </p>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className='w-full'>
        <TabsList variant='line' className='mb-6 inline-flex w-fit gap-8'>
          <TabsTrigger
            value='error'
            className='px-0 pb-2 text-xs font-semibold'
          >
            错误页
          </TabsTrigger>
          <TabsTrigger
            value='contact'
            className='px-0 pb-2 text-xs font-semibold'
          >
            联系页
          </TabsTrigger>
        </TabsList>

        <TabsContent value='error' className='focus-visible:outline-none'>
          <ErrorPageTab optionMap={optionMap} />
        </TabsContent>

        <TabsContent value='contact' className='focus-visible:outline-none'>
          <ContactPageTab optionMap={optionMap} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
