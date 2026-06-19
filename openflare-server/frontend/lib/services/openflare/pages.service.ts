import type {AxiosProgressEvent, InternalAxiosRequestConfig} from 'axios';

import apiClient from '@/lib/services/core/api-client';
import type {ApiResponse} from '@/lib/services/core';

import {OpenFlareBaseService} from './base.service';
import type {
  PagesDeployment,
  PagesDeploymentFile,
  PagesDeploymentUploadPayload,
  PagesProject,
  PagesProjectPayload,
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

  static listDeployments(projectId: number): Promise<PagesDeployment[]> {
    return this.get<PagesDeployment[]>(`/${projectId}/deployments`);
  }

  static listDeploymentFiles(
    projectId: number,
    deploymentId: number,
  ): Promise<PagesDeploymentFile[]> {
    return this.get<PagesDeploymentFile[]>(
      `/${projectId}/deployments/${deploymentId}/files`,
    );
  }

  static uploadDeployment(
    projectId: number,
    payload: PagesDeploymentUploadPayload,
  ): Promise<PagesDeployment> {
    const formData = new FormData();
    formData.append('package', payload.file);
    formData.append('root_dir', payload.rootDir ?? '');
    formData.append('entry_file', payload.entryFile ?? 'index.html');

    return this.postFormData<PagesDeployment>(
      `/${projectId}/deployments/upload`,
      formData,
      payload.onProgress,
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
    return this.post<void>(
      `/${projectId}/deployments/${deploymentId}/delete`,
    );
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