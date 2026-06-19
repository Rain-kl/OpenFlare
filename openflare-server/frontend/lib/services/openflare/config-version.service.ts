import type {InternalAxiosRequestConfig} from 'axios';

import {OpenFlareBaseService} from './base.service';
import type {
  ConfigDiffResult,
  ConfigPreviewResult,
  ConfigVersionCleanupPayload,
  ConfigVersionCleanupResult,
  ConfigVersionDetail,
  ConfigVersionSummary,
} from './types';

export class ConfigVersionService extends OpenFlareBaseService {
  protected static override readonly basePath: string = '/api/v1/d/config-versions';

  static list(): Promise<ConfigVersionSummary[]> {
    return this.get<ConfigVersionSummary[]>('/');
  }

  static getActive(): Promise<ConfigVersionDetail> {
    return this.get<ConfigVersionDetail>('/active');
  }

  static preview(): Promise<ConfigPreviewResult> {
    return this.get<ConfigPreviewResult>('/preview');
  }

  static diff(): Promise<ConfigDiffResult> {
    return this.get<ConfigDiffResult>('/diff');
  }

  static getById(id: number): Promise<ConfigVersionDetail> {
    return this.get<ConfigVersionDetail>(`/${id}`);
  }

  static publish(force?: boolean): Promise<ConfigVersionDetail> {
    return this.post<ConfigVersionDetail>(
      '/publish',
      undefined,
      force
        ? ({ params: { force: true } } as InternalAxiosRequestConfig)
        : undefined,
    );
  }

  static activate(id: number): Promise<ConfigVersionDetail> {
    return this.post<ConfigVersionDetail>(`/${id}/activate`);
  }

  static cleanup(
    payload: ConfigVersionCleanupPayload,
  ): Promise<ConfigVersionCleanupResult> {
    return this.post<ConfigVersionCleanupResult>('/cleanup', payload);
  }
}