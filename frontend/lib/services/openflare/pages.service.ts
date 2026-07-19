import type { AxiosProgressEvent, InternalAxiosRequestConfig } from 'axios';

import apiClient from '@/lib/services/core/api-client';
import { apiConfig } from '@/lib/services/core/config';
import type { ApiResponse } from '@/lib/services/core';

import { OpenFlareBaseService } from './base.service';
import type {
  PagesDeployment,
  PagesDeploymentFile,
  PagesDeploymentUploadFromURLPayload,
  PagesDeploymentUploadPayload,
  PagesProject,
  PagesProjectPayload,
  PagesRemoteSourceUpdatePayload,
  PagesSource,
  PagesSourceActionPayload,
  PagesSourceActionReceipt,
  PagesSourceUpdateResult,
} from './types';

export class PagesService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/pages';

  static listProjects(): Promise<PagesProject[]> {
    return this.get<PagesProject[]>('/');
  }

  static getProject(id: number): Promise<PagesProject> {
    return this.get<PagesProject>(`/${id}`);
  }

  static createProject(payload: PagesProjectPayload): Promise<PagesProject> {
    return this.post<PagesProject>('/', payload);
  }

  static updateProject(
    id: number,
    payload: PagesProjectPayload,
  ): Promise<PagesProject> {
    return this.post<PagesProject>(`/${id}/update`, payload);
  }

  static deleteProject(id: number): Promise<void> {
    return this.post<void>(`/${id}/delete`);
  }

  static getSource(projectId: number): Promise<PagesSource> {
    return this.get<PagesSource>(`/${projectId}/source`);
  }

  static updateSource(
    projectId: number,
    payload: PagesRemoteSourceUpdatePayload,
  ): Promise<PagesSourceUpdateResult> {
    return this.post<PagesSourceUpdateResult>(
      `/${projectId}/source/update`,
      payload,
    );
  }

  static deleteSource(projectId: number): Promise<PagesSource> {
    return this.post<PagesSource>(`/${projectId}/source/delete`);
  }

  static checkSource(projectId: number): Promise<PagesSourceActionReceipt> {
    return this.post<PagesSourceActionReceipt>(
      `/${projectId}/source/check`,
      {},
    );
  }

  static syncSource(
    projectId: number,
    payload: PagesSourceActionPayload = {},
  ): Promise<PagesSourceActionReceipt> {
    return this.post<PagesSourceActionReceipt>(
      `/${projectId}/source/sync`,
      payload,
    );
  }

  static listDeployments(projectId: number): Promise<PagesDeployment[]> {
    return this.get<PagesDeployment[]>(`/${projectId}/deployments`);
  }

  static listDeploymentFiles(
    deploymentId: number,
  ): Promise<PagesDeploymentFile[]> {
    return this.get<PagesDeploymentFile[]>(
      `/deployments/${deploymentId}/files`,
    );
  }

  static uploadDeployment(
    projectId: number,
    payload: PagesDeploymentUploadPayload,
  ): Promise<PagesDeployment> {
    const formData = new FormData();
    formData.append('package', payload.file);

    return this.postFormData<PagesDeployment>(
      `/${projectId}/deployments/upload`,
      formData,
      payload.onProgress,
    );
  }

  static uploadDeploymentFromURL(
    projectId: number,
    payload: PagesDeploymentUploadFromURLPayload,
  ): Promise<PagesDeployment> {
    return this.post<PagesDeployment>(
      `/${projectId}/deployments/upload-from-url`,
      payload,
      { timeout: apiConfig.uploadTimeout } as InternalAxiosRequestConfig,
    );
  }

  static activateDeployment(
    projectId: number,
    deploymentId: number,
  ): Promise<PagesProject> {
    return this.post<PagesProject>(
      `/${projectId}/deployments/${deploymentId}/activate`,
    );
  }

  static deleteDeployment(
    projectId: number,
    deploymentId: number,
  ): Promise<void> {
    return this.post<void>(`/${projectId}/deployments/${deploymentId}/delete`);
  }

  private static async postFormData<T>(
    path: string,
    formData: FormData,
    onProgress?: (percent: number) => void,
  ): Promise<T> {
    const response = await apiClient.post<ApiResponse<T>>(
      this.getFullPath(path),
      formData,
      {
        timeout: apiConfig.uploadTimeout,
        headers: { 'Content-Type': 'multipart/form-data' },
        onUploadProgress: (event: AxiosProgressEvent) => {
          if (!onProgress || !event.total) return;
          const percent = Math.round((event.loaded / event.total) * 100);
          onProgress(percent);
        },
      } as InternalAxiosRequestConfig,
    );

    return response.data.data;
  }
}
