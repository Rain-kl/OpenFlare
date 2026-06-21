import type {InternalAxiosRequestConfig} from 'axios';

import {apiConfig} from '../core/config';
import {BaseService} from '../core/base.service';
import type {ListUploadsResponse, Upload, UploadImageResponse} from './types';

export class UploadService extends BaseService {
  protected static readonly basePath = '/api/v1/upload';

  static async uploadFile(
    file: File,
    type: string = 'generic',
    metadata?: Record<string, unknown>,
    accessMode?: number,
  ): Promise<Upload> {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('type', type);
    if (metadata) {
      formData.append('metadata', JSON.stringify(metadata));
    }
    if (accessMode !== undefined) {
      formData.append('access_mode', String(accessMode));
    }

    return this.post<Upload>('', formData, {
      timeout: apiConfig.uploadTimeout,
      headers: { 'Content-Type': 'multipart/form-data' },
    } as InternalAxiosRequestConfig);
  }

  static async uploadBase64Image(
    base64: string,
    type: string = 'generic',
    filename: string = 'image.png',
    accessMode?: number,
  ): Promise<UploadImageResponse> {
    const response = await fetch(base64);
    const blob = await response.blob();
    const mimeType = base64.match(/data:([^;]+);/)?.[1] || 'image/png';
    const file = new File([blob], filename, { type: mimeType });
    const result = await this.uploadFile(file, type, undefined, accessMode);
    return { id: result.id };
  }

  static async listMyUploads(
    page = 1,
    pageSize = 20,
    keyword?: string,
    type?: string,
    extension?: string,
  ): Promise<ListUploadsResponse> {
    const params: Record<string, string | number> = { page, page_size: pageSize };
    if (keyword) params.keyword = keyword;
    if (type) params.type = type;
    if (extension) params.extension = extension;
    return this.get<ListUploadsResponse>('/my', params);
  }

  static async deleteMyFile(id: string): Promise<void> {
    return this.delete<void>(`/${id}`);
  }

  static async updateMyFile(id: string, fileName: string, accessMode?: number): Promise<Upload> {
    return this.put<Upload>(`/${id}`, {
      file_name: fileName,
      access_mode: accessMode,
    });
  }

  static async batchDownloadMyFiles(ids: string[]): Promise<Blob> {
    return this.post<Blob>('/download/batch', { ids }, {
      responseType: 'blob',
    } as InternalAxiosRequestConfig);
  }

  static getDownloadUrl(id: string): string {
    return `${this.basePath}/download/${id}`;
  }
}