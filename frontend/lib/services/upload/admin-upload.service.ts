import type {InternalAxiosRequestConfig} from 'axios';

import {BaseService} from '../core/base.service';
import type {FileStatsResponse, ListUploadsResponse} from './types';

export class AdminUploadService extends BaseService {
  protected static readonly basePath = '/api/v1/admin/uploads';

  static async listUploads(
    page = 1,
    pageSize = 20,
    keyword?: string,
    type?: string,
    extension?: string,
  ): Promise<ListUploadsResponse> {
    const params: Record<string, string | number> = {
      page,
      page_size: pageSize,
    };
    if (keyword) params.keyword = keyword;
    if (type) params.type = type;
    if (extension) params.extension = extension;
    return this.get<ListUploadsResponse>('', params);
  }

  static async getFileStats(): Promise<FileStatsResponse> {
    return this.get<FileStatsResponse>('/stats');
  }

  static async deleteFile(id: string): Promise<void> {
    return this.delete<void>(`/${id}`);
  }

  static getDownloadUrl(id: string): string {
    return `${this.basePath}/download/${id}`;
  }

  static async batchDownload(ids: string[]): Promise<Blob> {
    return this.post<Blob>('/download/batch', { ids }, {
      responseType: 'blob',
    } as InternalAxiosRequestConfig);
  }
}
