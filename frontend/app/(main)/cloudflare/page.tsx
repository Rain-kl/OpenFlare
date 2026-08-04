'use client';

import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, Cloud, FolderCog, Settings } from 'lucide-react';
import Link from 'next/link';

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { ErrorInline } from '@/components/layout/error';
import { LoadingStateWithBorder } from '@/components/layout/loading';
import {
  CloudflareService,
  cloudflareQueryKey,
} from '@/lib/services/openflare';
import { getErrorMessage } from '../websites/components/website-utils';

export default function CloudflarePage() {
  const overviewQuery = useQuery({
    queryKey: [...cloudflareQueryKey, 'overview'],
    queryFn: () => CloudflareService.getOverview(),
  });

  const overview = overviewQuery.data;

  return (
    <div className='flex w-full flex-col gap-6 py-6 px-1'>
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2'>
          <Cloud className='size-5 text-primary' />
          <h1 className='text-2xl font-semibold tracking-tight'>Cloudflare</h1>
        </div>
        <div className='flex items-center gap-2'>
          <Button asChild variant='outline' size='sm'>
            <Link href='/cloudflare/settings'>
              <Settings data-icon='inline-start' />
              连接设置
            </Link>
          </Button>
          <Button asChild size='sm'>
            <Link href='/cloudflare/groups'>
              <FolderCog data-icon='inline-start' />
              指向分组
            </Link>
          </Button>
        </div>
      </div>

      {overviewQuery.isLoading ? (
        <LoadingStateWithBorder
          icon={Cloud}
          description='加载 Cloudflare 总览中...'
        />
      ) : overviewQuery.isError ? (
        <Card>
          <CardContent className='pt-6'>
            <ErrorInline
              message={getErrorMessage(overviewQuery.error)}
              onRetry={() => void overviewQuery.refetch()}
            />
          </CardContent>
        </Card>
      ) : !overview?.connection.ready ? (
        <Alert>
          <AlertTriangle />
          <AlertTitle>Cloudflare 连接尚未就绪</AlertTitle>
          <AlertDescription className='flex flex-col items-start gap-3'>
            <span>
              请先导入现有 Cloudflare DNS 账号，或录入独立 API Token
              并完成连接测试。
            </span>
            <Button asChild size='sm'>
              <Link href='/cloudflare/settings'>配置连接</Link>
            </Button>
          </AlertDescription>
        </Alert>
      ) : (
        <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-5'>
          {[
            ['指向分组', overview.group_count],
            ['域名成员', overview.member_count],
            ['同步正常', overview.ok_count],
            ['等待同步', overview.pending_count],
            ['同步错误', overview.error_count],
          ].map(([label, value]) => (
            <Card key={label}>
              <CardHeader className='pb-2'>
                <CardDescription>{label}</CardDescription>
                <CardTitle className='text-2xl'>{value}</CardTitle>
              </CardHeader>
            </Card>
          ))}
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>同步边界</CardTitle>
          <CardDescription>
            OpenFlare 数据库是本模块的期望状态来源。
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-3 text-sm text-muted-foreground'>
          <p>
            同步会覆盖本模块接管的同名 A 记录；如果 Cloudflare 中存在多条同名
            A，需先手动清理。
          </p>
          <p>成员移出或分组删除时，默认同时删除本模块管理的远端 A 记录。</p>
          <div>
            <Badge variant='secondary'>一期限制</Badge>
            <span className='ml-2'>
              一期不提供自动故障切换，备用节点仅保存配置。
            </span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
