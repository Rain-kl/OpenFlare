'use client';

import { useQuery } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';

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
  const t = useTranslations('pages.files');
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
              : t('loadFailed')
          }
          onRetry={() => void filesQuery.refetch()}
        />
      </div>
    );
  }

  const files = filesQuery.data ?? [];
  if (files.length === 0) {
    return <EmptyInline message={t('empty')} />;
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('path')}</TableHead>
          <TableHead className='text-right'>{t('size')}</TableHead>
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
