'use client';

import { useQuery } from '@tanstack/react-query';

import { EmptyInline } from '@/components/layout/empty';
import { ErrorInline } from '@/components/layout/error';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { PagesService } from '@/lib/services/openflare';

import {
  deploymentFilesQueryKey,
  formatBytes,
} from '../../components/pages-utils';

interface DeploymentFilesPanelProps {
  projectId: number;
  deploymentId: number;
}

export function DeploymentFilesPanel({
  projectId,
  deploymentId,
}: DeploymentFilesPanelProps) {
  const filesQuery = useQuery({
    queryKey: deploymentFilesQueryKey(projectId, deploymentId),
    queryFn: () => PagesService.listDeploymentFiles(deploymentId),
  });

  if (filesQuery.isLoading) {
    return (
      <div className='flex flex-col gap-2 p-4'>
        <Skeleton className='h-8 w-full' />
        <Skeleton className='h-8 w-full' />
      </div>
    );
  }

  if (filesQuery.isError) {
    return (
      <div className='p-4'>
        <ErrorInline
          message={
            filesQuery.error instanceof Error
              ? filesQuery.error.message
              : '文件清单加载失败'
          }
          onRetry={() => void filesQuery.refetch()}
        />
      </div>
    );
  }

  const files = filesQuery.data ?? [];
  if (files.length === 0) {
    return <EmptyInline message='暂无文件记录' />;
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>路径</TableHead>
          <TableHead className='text-right'>大小</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {files.map((file) => (
          <TableRow key={file.id}>
            <TableCell className='font-mono text-xs'>{file.path}</TableCell>
            <TableCell className='text-right text-xs text-muted-foreground'>
              {formatBytes(file.size)}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
