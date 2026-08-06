'use client';

import type { ReactNode } from 'react';
import { FileWarning } from 'lucide-react';
import type { UseQueryResult } from '@tanstack/react-query';

import { useAuth } from '@/components/providers/auth-provider';
import { EmptyStateWithBorder } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';

type ErrorPageGateProps = {
  optionsQuery: UseQueryResult<Array<{ key: string; value: string }>, Error>;
  children: ReactNode;
};

export function ErrorPageGate({ optionsQuery, children }: ErrorPageGateProps) {
  const { user, loading: authLoading } = useAuth();

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

  return <>{children}</>;
}
