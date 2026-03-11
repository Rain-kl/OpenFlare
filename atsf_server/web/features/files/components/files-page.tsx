'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { EmptyState } from '@/components/feedback/empty-state';
import { ErrorState } from '@/components/feedback/error-state';
import { InlineMessage } from '@/components/feedback/inline-message';
import { LoadingState } from '@/components/feedback/loading-state';
import { useAuth } from '@/components/providers/auth-provider';
import { PageHeader } from '@/components/layout/page-header';
import { AppCard } from '@/components/ui/app-card';
import {
  buildFileAbsoluteUrl,
  buildFileDownloadUrl,
  deleteFile,
  getFiles,
  searchFiles,
  uploadFiles,
} from '@/features/files/api/files';
import type { FileItem } from '@/features/files/types';
import {
  DangerButton,
  PrimaryButton,
  ResourceField,
  ResourceInput,
  SecondaryButton,
} from '@/features/shared/components/resource-primitives';
import { formatDateTime, formatRelativeTime } from '@/lib/utils/date';

const ITEMS_PER_PAGE = 10;
const filesQueryKey = ['files', 'list'] as const;

type FeedbackState = {
  tone: 'info' | 'success' | 'danger';
  message: string;
};

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : '请求失败，请稍后重试。';
}

async function copyToClipboard(value: string) {
  await navigator.clipboard.writeText(value);
}

export function FilesPage() {
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const [page, setPage] = useState(0);
  const [searchInput, setSearchInput] = useState('');
  const [searchKeyword, setSearchKeyword] = useState('');
  const [description, setDescription] = useState('');
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);

  const isAdmin = (user?.role ?? 0) >= 10;

  const filesPageQuery = useQuery({
    queryKey: [...filesQueryKey, page],
    queryFn: () => getFiles(page),
    enabled: isAdmin && searchKeyword.length === 0,
  });

  const filesSearchQuery = useQuery({
    queryKey: [...filesQueryKey, 'search', searchKeyword],
    queryFn: () => searchFiles(searchKeyword),
    enabled: isAdmin && searchKeyword.length > 0,
  });

  const uploadMutation = useMutation({
    mutationFn: async () => uploadFiles(selectedFiles, description.trim()),
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: `已上传 ${selectedFiles.length} 个文件。` });
      setSelectedFiles([]);
      setDescription('');
      await queryClient.invalidateQueries({ queryKey: filesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteFile,
    onSuccess: async () => {
      setFeedback({ tone: 'success', message: '文件已删除。' });
      await queryClient.invalidateQueries({ queryKey: filesQueryKey });
    },
    onError: (error) => {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    },
  });

  const activeQuery = searchKeyword.length > 0 ? filesSearchQuery : filesPageQuery;
  const files = activeQuery.data ?? [];

  const summary = useMemo(() => {
    return [
      { label: searchKeyword ? '搜索结果' : '当前页文件', value: files.length },
      { label: '当前上传批次', value: selectedFiles.length },
      {
        label: '下载总次数',
        value: files.reduce((total, item) => total + (item.download_counter || 0), 0),
      },
      {
        label: '上传者数',
        value: new Set(files.map((item) => item.uploader_id)).size,
      },
    ];
  }, [files, searchKeyword, selectedFiles.length]);

  const handleUpload = () => {
    if (selectedFiles.length === 0) {
      setFeedback({ tone: 'info', message: '请先选择要上传的文件。' });
      return;
    }

    setFeedback(null);
    uploadMutation.mutate();
  };

  const handleDelete = (file: FileItem) => {
    if (!window.confirm(`确认删除文件“${file.filename}”吗？`)) {
      return;
    }

    setFeedback(null);
    deleteMutation.mutate(file.id);
  };

  const handleCopy = async (link: string) => {
    try {
      await copyToClipboard(buildFileAbsoluteUrl(link));
      setFeedback({ tone: 'success', message: '文件链接已复制到剪贴板。' });
    } catch (error) {
      setFeedback({ tone: 'danger', message: getErrorMessage(error) });
    }
  };

  const handleSearchSubmit = () => {
    setFeedback(null);
    setPage(0);
    setSearchKeyword(searchInput.trim());
  };

  const handleResetSearch = () => {
    setSearchInput('');
    setSearchKeyword('');
    setPage(0);
    setFeedback(null);
  };

  if (!isAdmin) {
    return (
      <div className='space-y-6'>
        <PageHeader
          title='文件管理'
          description='阶段 4 已迁移文件管理页面，但当前账户没有管理员权限。'
        />
        <EmptyState
          title='权限不足'
          description='文件上传、删除与下载链接管理仅对管理员开放，请使用管理员账号登录后继续。'
        />
      </div>
    );
  }

  return (
    <div className='space-y-6'>
      <PageHeader
        title='文件管理'
        description='支持上传、搜索、分页、下载、复制外链和删除文件，覆盖常见日常文件分发场景。'
      />

      {feedback ? <InlineMessage tone={feedback.tone} message={feedback.message} /> : null}

      <div className='grid gap-6 xl:grid-cols-[1.1fr_0.9fr]'>
        <AppCard title='文件概览' description='统计信息会随当前视图和待上传队列同步变化。'>
          <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
            {summary.map((item) => (
              <div
                key={item.label}
                className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4'
              >
                <p className='text-xs uppercase tracking-[0.2em] text-[var(--foreground-muted)]'>
                  {item.label}
                </p>
                <p className='mt-2 text-lg font-semibold text-[var(--foreground-primary)]'>{item.value}</p>
              </div>
            ))}
          </div>
        </AppCard>

        <AppCard
          title='上传文件'
          description='支持一次选择多个文件并附加统一描述信息。文件上传后将立即可下载。'
          action={
            <PrimaryButton type='button' onClick={handleUpload} disabled={uploadMutation.isPending}>
              {uploadMutation.isPending ? '上传中...' : '开始上传'}
            </PrimaryButton>
          }
        >
          <div className='space-y-5'>
            <ResourceField label='上传描述' hint='可选，适用于同批次文件的统一说明。'>
              <ResourceInput
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder='例如：节点安装包 / 测试证书 / 宣传素材'
              />
            </ResourceField>

            <ResourceField label='文件选择' hint='按住 Ctrl 或 Shift 可批量选择多个文件。'>
              <input
                type='file'
                multiple
                onChange={(event) => setSelectedFiles(Array.from(event.target.files ?? []))}
                className='block w-full rounded-2xl border border-dashed border-[var(--border-default)] bg-[var(--surface-muted)] px-4 py-6 text-sm text-[var(--foreground-secondary)] file:mr-4 file:rounded-xl file:border-0 file:bg-[var(--brand-primary)] file:px-4 file:py-2 file:text-sm file:font-medium file:text-[var(--foreground-inverse)]'
              />
            </ResourceField>

            {selectedFiles.length > 0 ? (
              <div className='rounded-2xl border border-[var(--border-default)] bg-[var(--surface-elevated)] px-4 py-4'>
                <p className='text-sm font-medium text-[var(--foreground-primary)]'>待上传文件</p>
                <ul className='mt-3 space-y-2 text-sm text-[var(--foreground-secondary)]'>
                  {selectedFiles.map((file) => (
                    <li key={`${file.name}-${file.size}`} className='break-all'>
                      {file.name} · {(file.size / 1024).toFixed(1)} KB
                    </li>
                  ))}
                </ul>
              </div>
            ) : (
              <EmptyState title='尚未选择文件' description='选择文件后即可直接上传，无需离开当前页面。' />
            )}
          </div>
        </AppCard>
      </div>

      <AppCard
        title='文件列表'
        description='分页模式下按服务器每页 10 条加载；搜索模式会返回全部匹配结果。'
        action={
          <SecondaryButton
            type='button'
            onClick={() => void queryClient.invalidateQueries({ queryKey: filesQueryKey })}
          >
            刷新
          </SecondaryButton>
        }
      >
        <div className='space-y-5'>
          <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
            <div className='flex w-full flex-col gap-3 md:flex-row'>
              <ResourceInput
                value={searchInput}
                onChange={(event) => setSearchInput(event.target.value)}
                placeholder='搜索文件名、上传者或上传者 ID'
                className='md:max-w-xl'
              />
              <div className='flex flex-wrap gap-2'>
                <PrimaryButton type='button' onClick={handleSearchSubmit}>
                  搜索
                </PrimaryButton>
                <SecondaryButton type='button' onClick={handleResetSearch}>
                  清空
                </SecondaryButton>
              </div>
            </div>

            {searchKeyword.length === 0 ? (
              <div className='flex flex-wrap gap-2'>
                <SecondaryButton type='button' onClick={() => setPage((value) => Math.max(value - 1, 0))} disabled={page === 0 || activeQuery.isLoading}>
                  上一页
                </SecondaryButton>
                <SecondaryButton
                  type='button'
                  onClick={() => setPage((value) => value + 1)}
                  disabled={activeQuery.isLoading || files.length < ITEMS_PER_PAGE}
                >
                  下一页
                </SecondaryButton>
              </div>
            ) : null}
          </div>

          {activeQuery.isLoading ? (
            <LoadingState />
          ) : activeQuery.isError ? (
            <ErrorState title='文件列表加载失败' description={getErrorMessage(activeQuery.error)} />
          ) : files.length === 0 ? (
            <EmptyState title='暂无文件' description='当前条件下没有可展示的文件记录。' />
          ) : (
            <div className='overflow-x-auto'>
              <table className='min-w-full divide-y divide-[var(--border-default)] text-left text-sm'>
                <thead>
                  <tr className='text-[var(--foreground-secondary)]'>
                    <th className='px-3 py-3 font-medium'>文件</th>
                    <th className='px-3 py-3 font-medium'>上传者</th>
                    <th className='px-3 py-3 font-medium'>上传时间</th>
                    <th className='px-3 py-3 font-medium'>下载量</th>
                    <th className='px-3 py-3 font-medium'>操作</th>
                  </tr>
                </thead>
                <tbody className='divide-y divide-[var(--border-default)]'>
                  {files.map((file) => (
                    <tr key={file.id} className='align-top'>
                      <td className='px-3 py-4'>
                        <div className='space-y-1'>
                          <a
                            href={buildFileDownloadUrl(file.link)}
                            target='_blank'
                            rel='noreferrer'
                            className='font-medium text-[var(--brand-primary)] transition hover:opacity-80'
                          >
                            {file.filename}
                          </a>
                          <p className='max-w-80 text-xs leading-5 text-[var(--foreground-secondary)]'>
                            {file.description || '无描述信息'}
                          </p>
                          <p className='text-xs text-[var(--foreground-secondary)]'>
                            存储链接：{file.link}
                          </p>
                        </div>
                      </td>
                      <td className='px-3 py-4 text-[var(--foreground-secondary)]'>
                        <div className='space-y-1'>
                          <p>{file.uploader}</p>
                          <p className='text-xs'>上传者 ID：{file.uploader_id}</p>
                        </div>
                      </td>
                      <td className='px-3 py-4 text-[var(--foreground-secondary)]'>
                        {formatRelativeTime(file.upload_time)} · {formatDateTime(file.upload_time)}
                      </td>
                      <td className='px-3 py-4 text-[var(--foreground-secondary)]'>
                        {file.download_counter || 0}
                      </td>
                      <td className='px-3 py-4'>
                        <div className='flex flex-wrap gap-2'>
                          <PrimaryButton
                            type='button'
                            onClick={() => window.open(buildFileDownloadUrl(file.link), '_blank', 'noopener,noreferrer')}
                            className='px-3 py-2 text-xs'
                          >
                            下载
                          </PrimaryButton>
                          <SecondaryButton
                            type='button'
                            onClick={() => void handleCopy(file.link)}
                            className='px-3 py-2 text-xs'
                          >
                            复制链接
                          </SecondaryButton>
                          <DangerButton
                            type='button'
                            onClick={() => handleDelete(file)}
                            disabled={deleteMutation.isPending}
                            className='px-3 py-2 text-xs'
                          >
                            删除
                          </DangerButton>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </AppCard>
    </div>
  );
}
