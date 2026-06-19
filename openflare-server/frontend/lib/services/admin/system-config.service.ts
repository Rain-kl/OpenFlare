import {BaseService} from '@/lib/services/core';
import type {CreateSystemConfigRequest, SystemConfig, UpdateSystemConfigRequest,} from './types';

export class AdminSystemConfigService extends BaseService {
  protected static readonly basePath = '/api/v1/admin';

  static async createSystemConfig(request: CreateSystemConfigRequest): Promise<void> {
    return this.post<void>('/system-configs', request);
  }

  static async listSystemConfigs(type?: 'system' | 'business'): Promise<SystemConfig[]> {
    const query = type ? `?type=${type}` : '';
    return this.get<SystemConfig[]>(`/system-configs${query}`);
  }

  static async getSystemConfig(key: string): Promise<SystemConfig> {
    return this.get<SystemConfig>(`/system-configs/${key}`);
  }

  static async updateSystemConfig(key: string, request: UpdateSystemConfigRequest): Promise<void> {
    return this.put<void>(`/system-configs/${key}`, request);
  }

  static async testSMTP(request: {
    smtp_host: string;
    smtp_port: number;
    smtp_username: string;
    smtp_password: string;
    to: string;
  }): Promise<{ success: boolean; log: string; error: string }> {
    return this.post<{ success: boolean; log: string; error: string }>(
      '/system-configs/smtp/test',
      request,
    );
  }

  static async listUploadTypes(): Promise<string[]> {
    return this.get<string[]>('/uploads/types');
  }
}
