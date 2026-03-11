import { apiRequest, getApiUrl } from '@/lib/api/client';

import type { FileItem } from '@/features/files/types';

export function getFiles(page: number) {
  return apiRequest<FileItem[]>(`/file/?p=${page}`);
}

export function searchFiles(keyword: string) {
  return apiRequest<FileItem[]>(`/file/search?keyword=${encodeURIComponent(keyword)}`);
}

export function deleteFile(id: number) {
  return apiRequest<void>(`/file/${id}`, {
    method: 'DELETE',
  });
}

export function buildFileDownloadUrl(link: string) {
  return `/upload/${link}`;
}

export function buildFileAbsoluteUrl(link: string) {
  if (typeof window === 'undefined') {
    return buildFileDownloadUrl(link);
  }

  return new URL(buildFileDownloadUrl(link), window.location.origin).toString();
}

export async function uploadFiles(files: File[], description?: string) {
  const formData = new FormData();

  files.forEach((file) => {
    formData.append('file', file);
  });

  if (description) {
    formData.append('description', description);
  }

  const response = await fetch(getApiUrl('/file/'), {
    method: 'POST',
    credentials: 'include',
    body: formData,
  });

  const payload = (await response.json()) as {
    success: boolean;
    message: string;
  };

  if (!response.ok || !payload.success) {
    throw new Error(payload.message || `上传失败（${response.status}）`);
  }
}
